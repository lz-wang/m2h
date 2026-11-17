package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

// Session payload keys. The principal travels inside the signed session
// blob; the middleware re-reads and re-validates it on every request.
const (
	sessionPrincipalID = "principalID"
	sessionUsername    = "username"
	sessionDisplayName = "displayName"
	sessionProvider    = "provider"
)

// Login limiter bounds: five failures per username+address inside fifteen
// minutes lock that pair out.
const (
	loginFailureMax    = 5
	loginFailureWindow = 15 * time.Minute
)

// sessionCleanupInterval paces expired-session removal in authenticated
// runs. The default mode never starts this loop.
const sessionCleanupInterval = time.Hour

// authStack bundles everything an authenticated run adds to the server:
// the authenticator behind the mode, the scs session manager backed by
// SQLite, the login limiter, and the trusted public origin.
type authStack struct {
	mode          auth.Mode
	authenticator auth.Authenticator
	sessions      *scs.SessionManager
	sessionStore  *auth.SQLiteSessionStore
	limiter       *auth.LoginLimiter
	// dataStore is the run's SQLite store: annotations and tokens
	// persist through it.
	dataStore *store.Store
	// publicOrigin is the configured external origin (M2H_PUBLIC_URL
	// without a path), or "" when unconfigured. Same-origin checks accept
	// it in addition to the request's own Host, so deployments behind a
	// proxy with rewritten Host headers keep working.
	publicOrigin string
}

// buildAuthStack assembles the authentication layer from a validated
// config and the already-migrated store.
func buildAuthStack(config *auth.Config, dataStore *store.Store, basePath string) (*authStack, error) {
	var authenticator auth.Authenticator
	switch config.Mode {
	case auth.ModeLocal:
		built, err := auth.NewLocalAuthenticator(config.LocalUsers)
		if err != nil {
			return nil, fmt.Errorf("local authenticator: %w", err)
		}
		authenticator = built
	case auth.ModeLDAP:
		built, err := auth.NewLDAPAuthenticator(config.LDAP)
		if err != nil {
			return nil, fmt.Errorf("ldap authenticator: %w", err)
		}
		authenticator = built
	default:
		return nil, fmt.Errorf("auth mode %q has no authenticator", config.Mode)
	}

	sessionStore := auth.NewSQLiteSessionStore(dataStore.DB())
	sessions := scs.New()
	sessions.Store = sessionStore
	sessions.Lifetime = 7 * 24 * time.Hour
	sessions.IdleTimeout = 12 * time.Hour
	sessions.Cookie.Name = auth.SessionCookieName
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	// The cookie must carry an explicit path: an unset Path makes browsers
	// fall back to the request-directory default ("/api/auth"), which
	// would confine the session cookie to the login endpoint. The base
	// path arrives normalized ("/" collapses to "").
	cookiePath := basePath
	if cookiePath == "" {
		cookiePath = "/"
	}
	sessions.Cookie.Path = cookiePath
	// The Secure flag follows the configured public origin, never a
	// client-supplied forwarded-proto header: m2h itself always speaks
	// plain HTTP, so only the deployment's own declaration is trusted.
	sessions.Cookie.Secure = strings.HasPrefix(config.PublicURL, "https://")

	stack := &authStack{
		mode:          config.Mode,
		authenticator: authenticator,
		sessions:      sessions,
		sessionStore:  sessionStore,
		limiter:       auth.NewLoginLimiter(loginFailureMax, loginFailureWindow),
		dataStore:     dataStore,
	}
	if config.PublicURL != "" {
		stack.publicOrigin = publicOriginOf(config.PublicURL)
	}
	return stack, nil
}

// publicOriginOf reduces a configured public URL to its origin
// (scheme://host[:port]).
func publicOriginOf(publicURL string) string {
	if index := strings.Index(publicURL, "://"); index >= 0 {
		if slash := strings.IndexByte(publicURL[index+3:], '/'); slash >= 0 {
			return publicURL[:index+3+slash]
		}
	}
	return publicURL
}

// startSessionCleanup periodically drops expired sessions until the
// context ends; the returned channel closes after the final pass.
func (stack *authStack) startSessionCleanup(ctx context.Context) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(sessionCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = stack.sessionStore.CleanupSessions(cleanupContext)
				cancel()
			}
		}
	}()
	return done
}

// principal is the request-scoped identity the middleware installs.
type principal struct {
	ID          string
	Username    string
	DisplayName string
	Provider    string
	// Via records which credential produced the identity — a browser
	// session or (later) an API token. Write-scoped APIs (annotations)
	// accept sessions only.
	Via string
}

// Credential sources for principal.Via.
const (
	principalViaSession = "session"
	principalViaToken   = "token"
)

type principalContextKey struct{}

// principalFromRequest returns the authenticated identity of a request, or
// nil for anonymous requests.
func principalFromRequest(request *http.Request) *principal {
	value, ok := request.Context().Value(principalContextKey{}).(*principal)
	if !ok {
		return nil
	}
	return value
}

// requestWithPrincipal installs the identity on the request context.
func requestWithPrincipal(request *http.Request, identity *principal) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), principalContextKey{}, identity))
}

// loginRequest is the JSON body of POST /api/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// principalResponse is the JSON body of /api/auth/me and login success.
type principalResponse struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Provider    string `json:"provider"`
}

// serveLogin verifies credentials and establishes a session. Failures are
// uniform — 401 for bad credentials, 429 when the limiter is engaged — and
// the response never reveals which part was wrong.
func (handler *documentHandler) serveLogin(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !handler.sameOrigin(request) {
		writeJSONError(response, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 4<<10)
	var credentials loginRequest
	if err := json.NewDecoder(request.Body).Decode(&credentials); err != nil {
		writeJSONError(response, http.StatusBadRequest, "invalid login request")
		return
	}

	limiterKey := clientIP(request.RemoteAddr) + "|" + credentials.Username
	stack := handler.auth
	if !stack.limiter.Allowed(limiterKey) {
		writeJSONError(response, http.StatusTooManyRequests, "too many failed attempts")
		return
	}

	identity, err := stack.authenticator.Authenticate(request.Context(), credentials.Username, credentials.Password)
	if err != nil {
		stack.limiter.RecordFailure(limiterKey)
		writeJSONError(response, http.StatusUnauthorized, "invalid credentials")
		return
	}
	stack.limiter.Reset(limiterKey)

	// Renew before writing the identity: the token issued before login
	// (and anything fixed to it) cannot survive as a valid session.
	if err := stack.sessions.RenewToken(request.Context()); err != nil {
		writeJSONError(response, http.StatusInternalServerError, "establish session")
		return
	}
	stack.sessions.Put(request.Context(), sessionPrincipalID, identity.ID)
	stack.sessions.Put(request.Context(), sessionUsername, identity.Username)
	stack.sessions.Put(request.Context(), sessionDisplayName, identity.DisplayName)
	stack.sessions.Put(request.Context(), sessionProvider, identity.Provider)

	writeJSON(response, http.StatusOK, principalResponse{
		Username:    identity.Username,
		DisplayName: identity.DisplayName,
		Provider:    identity.Provider,
	})
}

// serveLogout destroys the current session.
func (handler *documentHandler) serveLogout(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !handler.sameOrigin(request) {
		writeJSONError(response, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	if err := handler.auth.sessions.Destroy(request.Context()); err != nil {
		writeJSONError(response, http.StatusInternalServerError, "destroy session")
		return
	}
	writeJSON(response, http.StatusOK, struct {
		OK bool `json:"ok"`
	}{OK: true})
}

// serveAuthMe answers the caller's own identity; the middleware guarantees
// a principal is present.
func (handler *documentHandler) serveAuthMe(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity := principalFromRequest(request)
	if identity == nil {
		writeJSONError(response, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(response, http.StatusOK, principalResponse{
		Username:    identity.Username,
		DisplayName: identity.DisplayName,
		Provider:    identity.Provider,
	})
}

// configResponse is the capability description GET /api/config serves to
// both modes. It carries no user or deployment details.
type configResponse struct {
	AuthMode string `json:"authMode"`
	Features struct {
		Annotations bool `json:"annotations"`
		Tokens      bool `json:"tokens"`
	} `json:"features"`
}

// serveConfig describes which capabilities this deployment exposes. Public
// in every mode: the WebUI needs it before it can even show a login page.
func (handler *documentHandler) serveConfig(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		writeJSONError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var config configResponse
	if handler.auth != nil {
		config.AuthMode = string(handler.auth.mode)
		// Annotations and tokens ship with the authenticated reader.
		config.Features.Annotations = true
		config.Features.Tokens = true
	} else {
		config.AuthMode = string(auth.ModeNone)
	}
	writeJSON(response, http.StatusOK, config)
}

// sameOrigin enforces the CSRF baseline for credential-bearing state
// changes: the Origin header (or the Referer's origin when Origin is
// absent, as with some same-origin form posts) must match the request's
// own host or the configured public origin. Browsers always send one of
// the two on cross-site POSTs; a mismatch is a forged request.
func (handler *documentHandler) sameOrigin(request *http.Request) bool {
	if origin := strings.TrimSpace(request.Header.Get("Origin")); origin != "" {
		return handler.originAllowed(request, origin)
	}
	if referer := strings.TrimSpace(request.Header.Get("Referer")); referer != "" {
		if index := strings.Index(referer, "://"); index >= 0 {
			if slash := strings.IndexByte(referer[index+3:], '/'); slash >= 0 {
				return handler.originAllowed(request, referer[:index+3+slash])
			}
			return handler.originAllowed(request, referer)
		}
		return false
	}
	return false
}

func (handler *documentHandler) originAllowed(request *http.Request, origin string) bool {
	if handler.auth != nil && handler.auth.publicOrigin != "" && origin == handler.auth.publicOrigin {
		return true
	}
	_, host, ok := strings.Cut(origin, "://")
	if !ok || host == "" {
		return false
	}
	// Behind a reverse proxy the browser's origin port may differ from the
	// internal one only when the proxy rewrites Host; trust the Host
	// header as received.
	return strings.EqualFold(host, request.Host)
}

// checkDataDirBoundary refuses a data directory that overlaps any served
// root's tree. The SQLite files (including write-ahead logs) must never sit
// inside a published document or attachment scope, nor may a root live
// inside the data directory where session and token rows would surface as
// reachable paths. Both directions are judged on canonical paths so
// symlinked roots cannot smuggle an overlap past the check.
func checkDataDirBoundary(dataDir string, workspace workspace) error {
	canonical, err := files.CanonicalPath(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data directory %q: %w", dataDir, err)
	}
	for _, root := range workspace.roots {
		boundary := root.scope.root
		if files.IsWithin(boundary, canonical) {
			return fmt.Errorf("data directory %q must not live inside served root %q", dataDir, root.input.Path)
		}
		if files.IsWithin(canonical, boundary) {
			return fmt.Errorf("served root %q must not live inside data directory %q", root.input.Path, dataDir)
		}
	}
	return nil
}
