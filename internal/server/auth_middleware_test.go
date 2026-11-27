package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

// authedTestServer assembles an authenticated document handler backed by a
// real SQLite store and serves it through httptest.
type authedTestServer struct {
	server  *httptest.Server
	stack   *authStack
	docsDir string
	dataDir string
}

func newAuthedTestServer(t *testing.T, config *auth.Config) *authedTestServer {
	t.Helper()
	base := t.TempDir()
	return newAuthedTestServerAt(t, config, filepath.Join(base, "docs"), filepath.Join(base, "m2h-data"))
}

// newAuthedTestServerAt builds the fixture on explicit directories so a test
// can re-create the server against the same documents and the same
// persisted sessions and tokens — the restart a configuration edit models.
func newAuthedTestServerAt(t *testing.T, config *auth.Config, docsDir, dataDir string) *authedTestServer {
	t.Helper()
	writeTestFile(t, filepath.Join(docsDir, "guide.md"), "# Guide\n\nBody text.")
	writeTestFile(t, filepath.Join(docsDir, "asset.txt"), "attachment")
	resolved := make([]files.Input, 0)
	for _, raw := range []string{docsDir} {
		input, err := files.Resolve(raw)
		if err != nil {
			t.Fatalf("resolve %q: %v", raw, err)
		}
		resolved = append(resolved, input)
	}
	workspace, err := newWorkspace(resolved, files.DiscoverOptions{}, false)
	if err != nil {
		t.Fatalf("newWorkspace failed: %v", err)
	}
	dataStore, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := workspace.assignIdentities(context.Background(), dataStore.EnsureDocumentRoot); err != nil {
		t.Fatalf("assignIdentities failed: %v", err)
	}
	stack, err := buildAuthStack(config, dataStore, "")
	if err != nil {
		t.Fatalf("buildAuthStack failed: %v", err)
	}
	ui := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="root"></div>`)}}
	handler := newDocumentHandlerWithAuth(context.Background(), workspace, nil, ui, "test", false, "", stack)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &authedTestServer{server: server, stack: stack, docsDir: docsDir, dataDir: dataDir}
}

func loginJSON(username, password string) io.Reader {
	body, _ := json.Marshal(loginRequest{Username: username, Password: password})
	return bytes.NewReader(body)
}

// loginAs performs a same-origin login and returns the session cookie.
func (fixture *authedTestServer) loginAs(t *testing.T, username, password string) *http.Cookie {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login", loginJSON(username, password))
	if err != nil {
		t.Fatalf("build login request: %v", err)
	}
	request.Header.Set("Origin", fixture.server.URL)
	response, err := fixture.server.Client().Do(request)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login %s = %d %s", username, response.StatusCode, body)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == auth.SessionCookieName {
			return cookie
		}
	}
	t.Fatalf("login response carried no session cookie")
	return nil
}

func (fixture *authedTestServer) do(request *http.Request) *http.Response {
	response, err := fixture.server.Client().Do(request)
	if err != nil {
		panic(err)
	}
	return response
}

// TestRevokedLocalUserCredentialsDieOnRebuild pins the P1-1 acceptance
// flow: mint a session and a token, remove the account from the
// configuration, restart against the same database — both credentials must
// die, while the surviving account keeps working.
func TestRevokedLocalUserCredentialsDieOnRebuild(t *testing.T) {
	base := t.TempDir()
	docsDir := filepath.Join(base, "docs")
	dataDir := filepath.Join(base, "m2h-data")
	fixture := &tokenFixture{authedTestServer: newAuthedTestServerAt(t, localAuthConfig(), docsDir, dataDir)}
	session := fixture.loginAs(t, "alice", "password-1")
	status, created := fixture.createTokenViaSession(t, session, map[string]any{
		"kind": "persistent",
		"name": "agent",
		"scope": map[string]any{
			"permissions": []string{"document:read", "search:read"},
			"resource":    map[string]any{"type": "all_published"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("mint = %d", status)
	}

	// Restart with alice removed from M2H_LOCAL_USERS_JSON.
	reduced := &auth.Config{
		Mode:       auth.ModeLocal,
		LocalUsers: []auth.User{{Username: "bob", Password: "password-2"}},
	}
	reborn := newAuthedTestServerAt(t, reduced, docsDir, dataDir)

	stale, _ := http.NewRequest(http.MethodGet, reborn.server.URL+"/api/files", nil)
	stale.AddCookie(session)
	response := reborn.do(stale)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked user's session = %d, want 401", response.StatusCode)
	}

	bearer, _ := http.NewRequest(http.MethodGet, reborn.server.URL+"/api/files", nil)
	bearer.Header.Set("Authorization", "Bearer "+created.Secret)
	bearerResponse := reborn.do(bearer)
	bearerResponse.Body.Close()
	if bearerResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked user's token = %d, want 401", bearerResponse.StatusCode)
	}

	// The surviving account is unaffected.
	if bob := reborn.loginAs(t, "bob", "password-2"); bob == nil {
		t.Fatalf("bob must still log in after the rebuild")
	}
}

func TestAuthenticatedRouteProtectionMatrix(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())
	session := fixture.loginAs(t, "alice", "password-1")

	protected := []string{
		"/api/files",
		"/api/document?path=guide.md",
		"/api/file-metadata?path=guide.md",
		"/api/search?q=body",
		"/raw/guide.md",
		"/assets/asset.txt",
		"/api/auth/me",
	}
	public := []string{
		"/",
		"/doc/guide.md",
		"/doc/secret.md",
		"/healthz",
		"/api/config",
		"/ui/index.html",
	}

	for _, path := range protected {
		t.Run("anonymous "+path, func(t *testing.T) {
			response, err := http.Get(fixture.server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("anonymous %s = %d, want 401", path, response.StatusCode)
			}
		})
		t.Run("authenticated "+path, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, fixture.server.URL+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.AddCookie(session)
			response := fixture.do(request)
			defer response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				t.Fatalf("authenticated %s = 401", path)
			}
		})
	}
	for _, path := range public {
		t.Run("public "+path, func(t *testing.T) {
			response, err := http.Get(fixture.server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("public %s = %d, want 200", path, response.StatusCode)
			}
		})
	}
}

func TestAuthenticatedHeadMatchesGetAuthorization(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	anonymous, err := http.Head(fixture.server.URL + "/raw/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	anonymous.Body.Close()
	if anonymous.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous HEAD /raw = %d, want 401", anonymous.StatusCode)
	}

	session := fixture.loginAs(t, "alice", "password-1")
	request, err := http.NewRequest(http.MethodHead, fixture.server.URL+"/raw/guide.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(session)
	authenticated := fixture.do(request)
	authenticated.Body.Close()
	if authenticated.StatusCode != http.StatusOK {
		t.Fatalf("authenticated HEAD /raw = %d, want 200", authenticated.StatusCode)
	}
}

func TestAnonymousDocDeepLinkDoesNotLeakExistence(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	for _, path := range []string{"/doc/guide.md", "/doc/no-such.md"} {
		response, err := http.Get(fixture.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("root")) {
			t.Fatalf("anonymous %s = %d %q, want the shell", path, response.StatusCode, body)
		}
	}

	// Authenticated visitors do see misses.
	session := fixture.loginAs(t, "alice", "password-1")
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/doc/no-such.md", nil)
	request.AddCookie(session)
	missing := fixture.do(request)
	missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("authenticated missing doc = %d, want 404", missing.StatusCode)
	}
}

func TestLoginLogoutMeLifecycle(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	// Wrong credentials are uniformly rejected without revealing which
	// part failed.
	for name, credentials := range map[string]loginRequest{
		"wrong password": {Username: "alice", Password: "nope"},
		"unknown user":   {Username: "mallory", Password: "password-1"},
		"empty password": {Username: "alice", Password: ""},
	} {
		body, _ := json.Marshal(credentials)
		request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login", bytes.NewReader(body))
		request.Header.Set("Origin", fixture.server.URL)
		response := fixture.do(request)
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("login %s = %d, want 401", name, response.StatusCode)
		}
	}

	session := fixture.loginAs(t, "alice", "password-1")
	if session.Value == "" {
		t.Fatalf("session token not issued")
	}
	if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie attributes missing hardening: %+v", session)
	}

	// me answers the identity.
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/auth/me", nil)
	request.AddCookie(session)
	me := fixture.do(request)
	defer me.Body.Close()
	var identity principalResponse
	if err := json.NewDecoder(me.Body).Decode(&identity); err != nil || identity.Username != "alice" {
		t.Fatalf("me = %v (err %v)", identity, err)
	}

	// Logout invalidates the session immediately.
	logout, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/logout", nil)
	logout.AddCookie(session)
	logout.Header.Set("Origin", fixture.server.URL)
	response := fixture.do(logout)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("logout = %d", response.StatusCode)
	}
	after, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/auth/me", nil)
	after.AddCookie(session)
	expired := fixture.do(after)
	expired.Body.Close()
	if expired.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me after logout = %d, want 401", expired.StatusCode)
	}
}

func TestSessionLifetimeSlidesOnActivity(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())
	for _, tc := range []struct {
		name       string
		method     string
		path       string
		fresh      bool
		crossSite  bool
		expired    bool
		wantStatus int
		renews     bool
	}{
		{name: "identity API", method: http.MethodGet, path: "/api/auth/me", wantStatus: http.StatusOK, renews: true},
		{name: "recently renewed session", method: http.MethodGet, path: "/api/auth/me", fresh: true, wantStatus: http.StatusOK},
		{name: "document shell", method: http.MethodGet, path: "/doc/guide.md", wantStatus: http.StatusOK, renews: true},
		{name: "health probe", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK},
		{name: "cross-origin mutation", method: http.MethodPost, path: "/api/annotations", crossSite: true, wantStatus: http.StatusForbidden},
		{name: "expired session", method: http.MethodGet, path: "/api/auth/me", expired: true, wantStatus: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := fixture.loginAs(t, "alice", "password-1")
			sevenDays := 7 * 24 * time.Hour
			if remaining := time.Until(session.Expires); remaining < sevenDays-time.Minute || remaining > sevenDays+time.Second {
				t.Fatalf("initial cookie lifetime = %s, want 7 days", remaining)
			}

			// Simulate an existing session close to its persisted expiry.
			oldExpiry := time.Now().Add(time.Hour)
			if tc.fresh {
				oldExpiry = time.Now().Add(sevenDays - time.Minute)
			}
			if tc.expired {
				oldExpiry = time.Now().Add(-time.Second)
			}
			sessionStore := fixture.stack.sessionStore
			data, found, err := sessionStore.Find(session.Value)
			if err != nil || !found {
				t.Fatalf("find session = %v, %v", found, err)
			}
			if err := sessionStore.Commit(session.Value, data, oldExpiry); err != nil {
				t.Fatal(err)
			}

			request, err := http.NewRequest(tc.method, fixture.server.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.AddCookie(session)
			if tc.crossSite {
				request.Header.Set("Origin", "https://other.example")
			}
			response := fixture.do(request)
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.wantStatus)
			}
			cookies := response.Cookies()
			if tc.renews {
				if len(cookies) != 1 || cookies[0].Name != session.Name || cookies[0].Value != session.Value {
					t.Fatal("response did not refresh the existing session cookie")
				}
				if remaining := time.Until(cookies[0].Expires); remaining < sevenDays-time.Minute || remaining > sevenDays+time.Second {
					t.Fatalf("refreshed cookie lifetime = %s, want 7 days", remaining)
				}
			} else if len(cookies) != 0 {
				t.Fatal("response unexpectedly set a cookie")
			}

			var storedExpiry int64
			err = fixture.stack.dataStore.DB().QueryRow(
				"SELECT expires_at FROM sessions ORDER BY rowid DESC LIMIT 1",
			).Scan(&storedExpiry)
			if err != nil {
				t.Fatal(err)
			}
			if tc.renews {
				if expiry := time.UnixMilli(storedExpiry); cookies[0].Expires.Sub(expiry) > time.Second || cookies[0].Expires.Before(expiry) {
					t.Fatalf("stored expiry %v does not match cookie expiry %v", expiry, cookies[0].Expires)
				}
			} else if storedExpiry != oldExpiry.UnixMilli() {
				t.Fatal("stored expiry changed without activity")
			}
		})
	}
}

func TestSessionCookieCarriesFreshTokenAfterLogin(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	// Establish an anonymous session token first (the browser would have
	// one from visiting the shell), then log in: the token must rotate so
	// a value fixed before login cannot survive as the authenticated
	// session.
	shell, err := http.Get(fixture.server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	var anonymous *http.Cookie
	for _, cookie := range shell.Cookies() {
		if cookie.Name == auth.SessionCookieName {
			anonymous = cookie
		}
	}
	shell.Body.Close()

	request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login",
		loginJSON("alice", "password-1"))
	if anonymous != nil {
		request.AddCookie(anonymous)
	}
	request.Header.Set("Origin", fixture.server.URL)
	response := fixture.do(request)
	response.Body.Close()
	var renewed *http.Cookie
	for _, cookie := range response.Cookies() {
		if cookie.Name == auth.SessionCookieName {
			renewed = cookie
		}
	}
	if renewed == nil || renewed.Value == "" {
		t.Fatalf("login did not set a session cookie")
	}
	if anonymous != nil && renewed.Value == anonymous.Value {
		t.Fatalf("session token survived login unchanged (fixation)")
	}
}

func TestLoginRequiresSameOrigin(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	tests := []struct {
		name   string
		origin string
		refer  string
		want   int
	}{
		{name: "same origin", origin: fixture.server.URL, want: http.StatusOK},
		{name: "cross origin", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "no headers", want: http.StatusForbidden},
		{name: "referer fallback", refer: fixture.server.URL + "/doc/guide.md", want: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login",
				loginJSON("alice", "password-1"))
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.refer != "" {
				request.Header.Set("Referer", test.refer)
			}
			response := fixture.do(request)
			response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("login with %s = %d, want %d", test.name, response.StatusCode, test.want)
			}
		})
	}
}

func TestStateChangesRequireSameOriginWithSession(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())
	session := fixture.loginAs(t, "alice", "password-1")

	logout, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/logout", nil)
	logout.AddCookie(session)
	response := fixture.do(logout)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("logout without Origin = %d, want 403", response.StatusCode)
	}
}

func TestLoginRateLimit(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	attempt := func() int {
		request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login",
			loginJSON("alice", "wrong"))
		request.Header.Set("Origin", fixture.server.URL)
		response := fixture.do(request)
		response.Body.Close()
		return response.StatusCode
	}
	for attemptIndex := range loginFailureMax {
		if got := attempt(); got != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", attemptIndex+1, got)
		}
	}

	if got := attempt(); got != http.StatusTooManyRequests {
		t.Fatalf("attempt after limit = %d, want 429", got)
	}

	// A different username from the same address is not collateral damage.
	request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login",
		loginJSON("bob", "wrong-password"))
	request.Header.Set("Origin", fixture.server.URL)
	response := fixture.do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unrelated username = %d, want 401", response.StatusCode)
	}
}

func TestSessionProviderMustMatchMode(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	// Forge a session payload whose provider names a different mode; the
	// middleware must destroy it rather than trust it.
	sessionCtx, err := fixture.stack.sessions.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("open session context: %v", err)
	}
	fixture.stack.sessions.Put(sessionCtx, sessionPrincipalID, "ldap:someone")
	fixture.stack.sessions.Put(sessionCtx, sessionUsername, "someone")
	fixture.stack.sessions.Put(sessionCtx, sessionProvider, "ldap")
	token, _, err := fixture.stack.sessions.Commit(sessionCtx)
	if err != nil {
		t.Fatalf("commit forged session: %v", err)
	}
	if token == "" {
		t.Fatal("no session token to forge with")
	}

	probe, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/auth/me", nil)
	probe.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	response := fixture.do(probe)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cross-provider session accepted: %d", response.StatusCode)
	}
}

func TestBearerCredentialsRejectedBeforeTokenStage(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())
	session := fixture.loginAs(t, "alice", "password-1")

	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
	request.Header.Set("Authorization", "Bearer m2h_not_a_real_token")
	bearerOnly := fixture.do(request)
	bearerOnly.Body.Close()
	if bearerOnly.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bearer-only request = %d, want 401", bearerOnly.StatusCode)
	}

	// Even alongside a valid cookie, a presented bearer is the authority.
	requestWithBoth, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
	requestWithBoth.Header.Set("Authorization", "Bearer m2h_not_a_real_token")
	requestWithBoth.AddCookie(session)
	both := fixture.do(requestWithBoth)
	both.Body.Close()
	if both.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bearer + cookie = %d, want 401 (no downgrade)", both.StatusCode)
	}
}

func TestPublicRequestsDoNotSetSessionCookies(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	for _, path := range []string{"/healthz", "/api/config"} {
		response, err := http.Get(fixture.server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		for _, cookie := range response.Cookies() {
			if cookie.Name == auth.SessionCookieName {
				t.Fatalf("%s set a session cookie on an anonymous request", path)
			}
		}
	}
}

func TestDefaultModeRegistersNoAuthEndpoints(t *testing.T) {
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "guide.md"), "# Guide")
	resolved, err := files.Resolve(docs)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	workspace, err := newWorkspace([]files.Input{resolved}, files.DiscoverOptions{}, false)
	if err != nil {
		t.Fatalf("newWorkspace failed: %v", err)
	}
	handler := newDocumentHandler(workspace, nil, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="root"></div>`)},
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	for _, path := range []string{"/api/auth/login", "/api/auth/logout", "/api/auth/me"} {
		response, err := http.Post(server.URL+path, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("default mode %s = %d, want 404", path, response.StatusCode)
		}
	}
	response, err := http.Get(server.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var config configResponse
	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		t.Fatal(err)
	}
	if config.AuthMode != "none" {
		t.Fatalf("default config authMode = %q, want none", config.AuthMode)
	}
	if config.Features.Annotations || config.Features.Tokens || config.Features.Shares {
		t.Fatalf("default config must not advertise features: %+v", config.Features)
	}
}

func TestAuthWorksBehindBasePath(t *testing.T) {
	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	writeTestFile(t, filepath.Join(docs, "guide.md"), "# Guide")
	resolved, _ := files.Resolve(docs)
	workspace, _ := newWorkspace([]files.Input{resolved}, files.DiscoverOptions{}, false)
	dataStore, err := store.Open(filepath.Join(base, "m2h-data"))
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer dataStore.Close()
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	stack, err := buildAuthStack(localAuthConfig(), dataStore, "/apps/docs")
	if err != nil {
		t.Fatalf("buildAuthStack failed: %v", err)
	}
	handler := newDocumentHandlerWithAuth(context.Background(), workspace, nil,
		fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="root"></div>`)}},
		"test", false, "/apps/docs", stack)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Anonymous request through the sub-path is protected.
	response, err := http.Get(server.URL + "/apps/docs/api/files")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous behind base path = %d, want 401", response.StatusCode)
	}

	login, _ := http.NewRequest(http.MethodPost, server.URL+"/apps/docs/api/auth/login",
		loginJSON("alice", "password-1"))
	login.Header.Set("Origin", server.URL)
	loginResponse, err := server.Client().Do(login)
	if err != nil {
		t.Fatal(err)
	}
	loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("login behind base path = %d", loginResponse.StatusCode)
	}
	var session *http.Cookie
	for _, cookie := range loginResponse.Cookies() {
		if cookie.Name == auth.SessionCookieName {
			session = cookie
		}
	}
	if session == nil {
		t.Fatal("no session cookie behind base path")
	}
	if session.Path != "/apps/docs" {
		t.Fatalf("cookie path = %q, want /apps/docs", session.Path)
	}
}
