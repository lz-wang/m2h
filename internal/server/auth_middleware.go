package server

import (
	"net/http"
	"strings"
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
		if publicAuthPath(path) {
			// Public paths stay reachable, but a valid session still
			// identifies the caller — the document shell needs to know
			// whether it may probe deep links without leaking existence
			// to anonymous visitors.
			if identity := handler.principalFromSession(request); identity != nil {
				request = requestWithPrincipal(request, identity)
			}
			next.ServeHTTP(response, request)
			return
		}

		identity := handler.principalFromSession(request)
		if identity == nil {
			writeUnauthorized(response, path)
			return
		}
		if !readOnlyMethod(request.Method) && !handler.sameOrigin(request) {
			writeJSONError(response, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(response, requestWithPrincipal(request, identity))
	})
}

// principalFromSession validates the request's session against the
// current auth mode. A session minted under a different mode — local
// accounts after a switch to LDAP, or back — is destroyed on sight.
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
	if identity.Provider != string(handler.auth.mode) {
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
