package server

import (
	"net/http"
	"strings"

	"github.com/lz-wang/m2h/internal/auth"
)

// publicAuthPath reports whether one internal (base-path-stripped) URL is
// reachable without a session: the WebUI shell and its assets, the health
// probe, the capability description and the login endpoint itself. The
// shell never embeds document content, so publishing it leaks nothing; the
// deep-link document handler takes care not to leak existence either.
func publicAuthPath(path string) bool {
	switch {
	case path == "/", strings.HasPrefix(path, "/doc/"):
		return true
	case path == "/healthz":
		return true
	case path == "/api/config":
		return true
	case path == "/api/auth/login":
		return true
	case path == "/ui/markdown.css" || strings.HasPrefix(path, "/ui/"):
		return true
	case strings.HasPrefix(path, "/runtime/"):
		return true
	default:
		return false
	}
}

// authenticate guards the internal routing layer of authenticated runs.
// It runs after mountBasePath, so authorization decisions see the same
// internal URL shape whatever --base-path the deployment chose.
//
// Credentials come in two shapes, evaluated exclusively:
//
//   - Authorization: Bearer … validates as an API token. A presented
//     bearer never falls back to cookie authority — an invalid token is
//     a 401, not a downgrade. (Token validation itself arrives with the
//     token stage; until then every bearer is rejected.)
//   - the session cookie validates against the session store, and the
//     session's provider must match the mode this server runs.
//
// Session-authenticated state changes additionally require a same-origin
// Origin/Referer (CSRF baseline). HEAD follows the GET rules exactly.
func (handler *documentHandler) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if handler.auth == nil {
			next.ServeHTTP(response, request)
			return
		}
		path := request.URL.Path
		if presented := request.Header.Get("Authorization"); presented != "" {
			// A presented bearer is the only authority for the request,
			// whatever the path: invalid tokens are a 401, valid tokens
			// never borrow cookie reach, and token management stays
			// session-only even for a valid token.
			handler.authorizeBearer(response, request, next)
			return
		}
		public := publicAuthPath(path)
		identity := handler.principalFromSession(request)
		if identity == nil {
			if public {
				next.ServeHTTP(response, request)
			} else {
				writeUnauthorized(response, path)
			}
			return
		}
		if !public && !readOnlyMethod(request.Method) && !handler.sameOrigin(request) {
			writeJSONError(response, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		// Opening the shell counts as activity. Public probes and resources
		// may be requested with no user interaction, and logout deletes the session.
		if (!public && path != "/api/auth/logout") || path == "/" || strings.HasPrefix(path, "/doc/") {
			ctx := request.Context()
			token := handler.auth.sessions.Token(ctx)
			expiry, renewed, err := handler.auth.sessionStore.Refresh(ctx, token, sessionLifetime, sessionRefreshInterval)
			if err != nil {
				handler.auth.sessions.ErrorFunc(response, request, err)
				return
			}
			if expiry.IsZero() {
				if public {
					next.ServeHTTP(response, request)
				} else {
					writeUnauthorized(response, path)
				}
				return
			}
			if renewed {
				handler.auth.sessions.WriteSessionCookie(ctx, response, token, expiry)
			}
		}
		next.ServeHTTP(response, requestWithPrincipal(request, identity))
	})
}

// principalFromSession validates the request's session against the
// current auth mode. A session minted under a different mode — local
// accounts after a switch to LDAP, or back — is destroyed on sight, and so
// is a session whose account no longer exists in the configuration: a
// removed user's cookie dies at its next use, not at its expiry.
func (handler *documentHandler) principalFromSession(request *http.Request) *principal {
	identity := &principal{
		ID:          handler.auth.sessions.GetString(request.Context(), sessionPrincipalID),
		Username:    handler.auth.sessions.GetString(request.Context(), sessionUsername),
		DisplayName: handler.auth.sessions.GetString(request.Context(), sessionDisplayName),
		Provider:    handler.auth.sessions.GetString(request.Context(), sessionProvider),
		Via:         principalViaSession,
	}
	if identity.ID == "" || identity.Username == "" {
		return nil
	}
	if identity.Provider != string(handler.auth.mode) ||
		!handler.auth.authorizer.IsPrincipalAllowed(auth.Principal{
			ID: identity.ID, Username: identity.Username, Provider: identity.Provider,
		}) {
		_ = handler.auth.sessions.Destroy(request.Context())
		return nil
	}
	return identity
}

func readOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// writeUnauthorized answers 401 in the dialect of the target: JSON for
// API paths, plain text everywhere else.
func writeUnauthorized(response http.ResponseWriter, path string) {
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api") {
		writeJSONError(response, http.StatusUnauthorized, "authentication required")
		return
	}
	http.Error(response, "authentication required", http.StatusUnauthorized)
}
