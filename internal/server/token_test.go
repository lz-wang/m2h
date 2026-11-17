package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

// tokenFixtureServer extends the authenticated fixture with token helpers.
type tokenFixture struct {
	*authedTestServer
}

func newTokenFixture(t *testing.T) *tokenFixture {
	t.Helper()
	return &tokenFixture{authedTestServer: newAuthedTestServer(t, localAuthConfig())}
}

// createTokenViaSession mints a token through the management API.
func (fixture *tokenFixture) createTokenViaSession(
	t *testing.T,
	session *http.Cookie,
	body map[string]any,
) (int, tokenResponse) {
	t.Helper()
	encoded, _ := json.Marshal(body)
	request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/tokens", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", fixture.server.URL)
	request.AddCookie(session)
	response := fixture.do(request)
	defer response.Body.Close()
	var created tokenResponse
	_ = json.NewDecoder(response.Body).Decode(&created)
	return response.StatusCode, created
}

func TestTokenCreateListRevoke(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")

	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "llm-agent",
		"scope": map[string]any{
			"permissions": []string{"document:read", "search:read"},
			"resource":    map[string]any{"type": "all_published"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}
	if created.Secret == "" || !auth.LooksLikeTokenSecret(created.Secret) {
		t.Fatalf("creation must return the secret once: %+v", created)
	}

	// Listing answers metadata only — never the secret again.
	listRequest, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/tokens", nil)
	listRequest.AddCookie(alice)
	listResponse := fixture.do(listRequest)
	var payload struct {
		Tokens []tokenResponse `json:"tokens"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	listResponse.Body.Close()
	if len(payload.Tokens) != 1 || payload.Tokens[0].Secret != "" {
		t.Fatalf("listing = %+v", payload.Tokens)
	}

	// Another user's tokens are invisible and untouchable.
	bob := fixture.loginAs(t, "bob", "password-2")
	bobList, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/tokens", nil)
	bobList.AddCookie(bob)
	bobResponse := fixture.do(bobList)
	var bobPayload struct {
		Tokens []tokenResponse `json:"tokens"`
	}
	_ = json.NewDecoder(bobResponse.Body).Decode(&bobPayload)
	bobResponse.Body.Close()
	if len(bobPayload.Tokens) != 0 {
		t.Fatalf("bob sees alice's tokens")
	}
	bobRevoke, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/tokens/"+created.ID, nil)
	bobRevoke.AddCookie(bob)
	bobRevoke.Header.Set("Origin", fixture.server.URL)
	if bobResponse := fixture.do(bobRevoke); bobResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign revoke = %d", bobResponse.StatusCode)
	}

	// Revoked tokens stop authorizing immediately.
	revoked, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/tokens/"+created.ID, nil)
	revoked.AddCookie(alice)
	revoked.Header.Set("Origin", fixture.server.URL)
	if response := fixture.do(revoked); response.StatusCode != http.StatusOK {
		t.Fatalf("revoke = %d", response.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	if response := fixture.do(request); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked bearer = %d", response.StatusCode)
	}
}

func TestTokenCreateValidation(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")

	tests := []struct {
		name string
		body map[string]any
	}{
		{name: "unknown kind", body: map[string]any{"kind": "eternal", "name": "x", "scope": map[string]any{"permissions": []string{"document:read"}, "resource": map[string]any{"type": "all_published"}}}},
		{name: "empty name", body: map[string]any{"kind": "persistent", "name": " ", "scope": map[string]any{"permissions": []string{"document:read"}, "resource": map[string]any{"type": "all_published"}}}},
		{name: "out-of-range expiry", body: map[string]any{
			"kind": "temporary", "name": "x", "expiresInDays": 30,
			"scope": map[string]any{"permissions": []string{"document:read"}, "resource": map[string]any{"type": "all_published"}},
		}},
		{name: "bad scope", body: map[string]any{"kind": "persistent", "name": "x", "scope": map[string]any{"permissions": []string{"document:write"}, "resource": map[string]any{"type": "all_published"}}}},
		{name: "unknown document scope", body: map[string]any{"kind": "persistent", "name": "x", "scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "document", "rootId": "nope", "path": "a.md"},
		}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if status, _ := fixture.createTokenViaSession(t, alice, test.body); status == http.StatusCreated {
				t.Fatalf("invalid token accepted")
			}
		})
	}
}

func TestBearerAuthorization(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")

	mint := func(t *testing.T, scope map[string]any, extra ...map[string]any) string {
		t.Helper()
		body := map[string]any{
			"kind":  "persistent",
			"name":  "agent",
			"scope": scope,
		}
		for _, addition := range extra {
			for key, value := range addition {
				body[key] = value
			}
		}
		status, created := fixture.createTokenViaSession(t, alice, body)
		if status != http.StatusCreated {
			t.Fatalf("mint = %d", status)
		}
		return created.Secret
	}
	allScope := map[string]any{
		"permissions": []string{"document:read", "search:read"},
		"resource":    map[string]any{"type": "all_published"},
	}
	documentScope := map[string]any{
		"permissions": []string{"document:read"},
		"resource": map[string]any{
			"type":   "document",
			"rootId": fixtureDocumentRef(t, fixture).RootID,
			"path":   "guide.md",
		},
	}

	t.Run("global token reads documents and searches", func(t *testing.T) {
		secret := mint(t, allScope)
		document, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/document?path=guide.md", nil)
		document.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(document); response.StatusCode != http.StatusOK {
			t.Fatalf("document = %d", response.StatusCode)
		}
		search, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/search?q=body", nil)
		search.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(search); response.StatusCode != http.StatusOK {
			t.Fatalf("search = %d", response.StatusCode)
		}
	})

	t.Run("document token reads only its document", func(t *testing.T) {
		secret := mint(t, documentScope)
		own, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/raw/guide.md", nil)
		own.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(own); response.StatusCode != http.StatusOK {
			t.Fatalf("own raw read = %d", response.StatusCode)
		}
		writeTestFile(t, fixture.docsDir+"/other.md", "# Other")
		other, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/raw/other.md", nil)
		other.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(other); response.StatusCode != http.StatusForbidden {
			t.Fatalf("foreign raw read = %d, want 403", response.StatusCode)
		}
		// Listing answers only the in-scope document.
		files, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
		files.Header.Set("Authorization", "Bearer "+secret)
		response := fixture.do(files)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("files = %d, want 200", response.StatusCode)
		}
		var payload struct {
			Roots []struct {
				Files []struct {
					Path string `json:"path"`
				} `json:"files"`
			} `json:"roots"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		for _, root := range payload.Roots {
			for _, file := range root.Files {
				if file.Path != "guide.md" {
					t.Fatalf("document token listed out-of-scope file %q", file.Path)
				}
			}
		}
	})

	t.Run("content-bound token 409s after edit", func(t *testing.T) {
		hash := fixtureDocumentHash(t, fixture)
		secret := mint(t, documentScope, map[string]any{"documentHash": hash})
		read, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/raw/guide.md", nil)
		read.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(read); response.StatusCode != http.StatusOK {
			t.Fatalf("bound read = %d", response.StatusCode)
		}
		writeTestFile(t, fixture.docsDir+"/guide.md", "# Edited\n\nNew body.")
		stale, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/raw/guide.md", nil)
		stale.Header.Set("Authorization", "Bearer "+secret)
		response := fixture.do(stale)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("stale bound read = %d, want 409", response.StatusCode)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if !bytes.Contains(body, []byte("document_changed")) {
			t.Fatalf("conflict body = %s", body)
		}
	})

	t.Run("tokens never manage tokens or login", func(t *testing.T) {
		secret := mint(t, allScope)
		management, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/tokens", nil)
		management.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(management); response.StatusCode != http.StatusForbidden {
			t.Fatalf("token management via bearer = %d", response.StatusCode)
		}
		login, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login",
			bytes.NewReader([]byte(`{"username":"alice","password":"password-1"}`)))
		login.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(login); response.StatusCode != http.StatusForbidden {
			t.Fatalf("login via bearer = %d", response.StatusCode)
		}
	})

	t.Run("tokens never touch annotations, read or write", func(t *testing.T) {
		secret := mint(t, allScope)
		annotations, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/annotations?path=guide.md", nil)
		annotations.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(annotations); response.StatusCode != http.StatusForbidden {
			t.Fatalf("annotation read via bearer = %d, want 403 — shared review comments are session-only", response.StatusCode)
		}
		create, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/annotations",
			bytes.NewReader([]byte("{}")))
		create.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(create); response.StatusCode != http.StatusForbidden {
			t.Fatalf("annotation write via bearer = %d, want 403", response.StatusCode)
		}
	})

	t.Run("tokens cannot read attachments", func(t *testing.T) {
		secret := mint(t, allScope)
		asset, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/assets/asset.txt", nil)
		asset.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(asset); response.StatusCode != http.StatusForbidden {
			t.Fatalf("asset read via bearer = %d", response.StatusCode)
		}
	})

	t.Run("garbage and absent bearers are 401", func(t *testing.T) {
		for _, credential := range []string{"Bearer junk", "Bearer m2h_", "Basic dXNlcjpwYXNz", "Bearer"} {
			request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
			request.Header.Set("Authorization", credential)
			if response := fixture.do(request); response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("credential %q = %d, want 401", credential, response.StatusCode)
			}
		}
	})

	t.Run("unlisted routes deny tokens by default", func(t *testing.T) {
		secret := mint(t, allScope)
		// Routes the scope matrix never names must fail closed: a future
		// API cannot leak open to bearers just by being added.
		for _, path := range []string{
			"/api/annotations/some-id",
			"/api/annotations/some-id/anchor",
			"/api/some-future-endpoint",
			"/",
		} {
			request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+path, nil)
			request.Header.Set("Authorization", "Bearer "+secret)
			if response := fixture.do(request); response.StatusCode != http.StatusForbidden {
				t.Fatalf("unlisted %s = %d, want 403", path, response.StatusCode)
			}
		}
		// The public capability surfaces stay reachable for monitoring and
		// agent bootstrapping, exactly as for anonymous callers.
		for _, path := range []string{"/healthz", "/api/config"} {
			request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+path, nil)
			request.Header.Set("Authorization", "Bearer "+secret)
			if response := fixture.do(request); response.StatusCode != http.StatusOK {
				t.Fatalf("public %s = %d, want 200", path, response.StatusCode)
			}
		}
	})

	t.Run("last_used_at writes are throttled", func(t *testing.T) {
		secret := mint(t, allScope)
		for range 3 {
			request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
			request.Header.Set("Authorization", "Bearer "+secret)
			fixture.do(request).Body.Close()
		}
		// The throttle allows one stamp per minute; three rapid requests
		// must not produce three database writes. Observable here only as
		// "no errors, requests still served" — the write count itself is
		// covered by the store's TouchToken test.
		request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(request); response.StatusCode != http.StatusOK {
			t.Fatalf("requests after throttle = %d", response.StatusCode)
		}
	})
}

// fixtureDocumentRef returns the authenticated document identity.
func fixtureDocumentRef(t *testing.T, fixture *tokenFixture) documentRefResponse {
	t.Helper()
	return fixtureDocument(t, fixture).Ref
}

func fixtureDocumentHash(t *testing.T, fixture *tokenFixture) string {
	t.Helper()
	return fixtureDocument(t, fixture).Hash
}

func fixtureDocument(t *testing.T, fixture *tokenFixture) struct {
	Ref  documentRefResponse `json:"documentRef"`
	Hash string              `json:"documentHash"`
} {
	t.Helper()
	session := fixture.loginAs(t, "alice", "password-1")
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/document?path=guide.md", nil)
	request.AddCookie(session)
	response := fixture.do(request)
	defer response.Body.Close()
	var document struct {
		Ref  documentRefResponse `json:"documentRef"`
		Hash string              `json:"documentHash"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestTokenExpiryEnforcedOnWire(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")

	// The shortest temporary expiry the API allows in days is too long for
	// a test, so the record is expired directly in the database.
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "temporary",
		"name": "share",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "all_published"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}
	_, err := fixture.stack.dataStore.DB().Exec(
		`UPDATE api_tokens SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Minute).UnixMilli(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	if response := fixture.do(request); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired bearer = %d", response.StatusCode)
	}
}

func TestScopedSearchFiltersInsideTheScan(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")

	ref := fixtureDocumentRef(t, fixture)
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "doc-token",
		"scope": map[string]any{
			"permissions": []string{"document:read", "search:read"},
			"resource": map[string]any{
				"type": "document", "rootId": ref.RootID, "path": "guide.md",
			},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}

	writeTestFile(t, fixture.docsDir+"/other.md", "# Guide\n\nBody text.")
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/search?q=body", nil)
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	response := fixture.do(request)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("scoped search = %d", response.StatusCode)
	}
	var results struct {
		Results []struct {
			Path string `json:"path"`
		} `json:"results"`
	}
	if err := json.NewDecoder(response.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results.Results) == 0 {
		t.Fatalf("scoped search found nothing")
	}
	for _, result := range results.Results {
		if result.Path != "guide.md" {
			t.Fatalf("document token searched out of scope: %q", result.Path)
		}
	}
}

func TestSessionCookiePathCoversRootDeployment(t *testing.T) {
	base := t.TempDir()
	dataStore, err := store.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A root deployment normalizes to "" internally; the cookie must say
	// "/" or browsers would default it to the login endpoint's directory.
	stack, err := buildAuthStack(localAuthConfig(), dataStore, "")
	if err != nil {
		t.Fatal(err)
	}
	if stack.sessions.Cookie.Path != "/" {
		t.Fatalf("cookie path = %q, want /", stack.sessions.Cookie.Path)
	}
}

func TestTokenScopeFilterBranches(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")
	handler := newTokenHandlerForTest(t, fixture)

	// Session callers see everything.
	sessionRequest, _ := http.NewRequest(http.MethodGet, "/api/files", nil)
	sessionRequest = requestWithPrincipal(sessionRequest, &principal{
		ID: "local:alice", Username: "alice", Provider: "local", Via: principalViaSession,
	})
	if !handler.tokenScopeFilter(sessionRequest)("r0/anything.md") {
		t.Fatalf("session caller filtered")
	}

	// Anonymous requests see everything (the middleware already gated).
	anonymous, _ := http.NewRequest(http.MethodGet, "/api/files", nil)
	if !handler.tokenScopeFilter(anonymous)("r0/anything.md") {
		t.Fatalf("anonymous request filtered")
	}

	// A token caller with a document scope sees exactly its document.
	ref := fixtureDocumentRef(t, fixture)
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "scoped",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "document", "rootId": ref.RootID, "path": "guide.md"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}
	record, err := fixture.stack.dataStore.FindTokenByHash(
		context.Background(), auth.HashTokenSecret(created.Secret))
	if err != nil {
		t.Fatal(err)
	}
	scopedRequest, _ := http.NewRequest(http.MethodGet, "/api/files", nil)
	scopedRequest = scopedRequest.WithContext(
		context.WithValue(scopedRequest.Context(), tokenRecordContextKey{}, record))
	scopedRequest = requestWithPrincipal(scopedRequest, &principal{
		ID: "local:alice", Via: principalViaToken, Provider: "local",
	})
	filter := handler.tokenScopeFilter(scopedRequest)
	if !filter("guide.md") {
		t.Fatalf("document scope rejected its own document")
	}
	if filter("other.md") {
		t.Fatalf("document scope accepted a foreign document")
	}

	// A malformed record on the context fails closed.
	brokenRequest, _ := http.NewRequest(http.MethodGet, "/api/files", nil)
	brokenRequest = brokenRequest.WithContext(context.WithValue(
		brokenRequest.Context(), tokenRecordContextKey{}, store.TokenRecord{ScopeJSON: "{"}))
	brokenRequest = requestWithPrincipal(brokenRequest, &principal{
		ID: "local:alice", Via: principalViaToken, Provider: "local",
	})
	if handler.tokenScopeFilter(brokenRequest)("guide.md") {
		t.Fatalf("malformed scope allowed everything")
	}
}

// newTokenHandlerForTest digs the documentHandler out of the fixture's
// serving stack for direct unit access.
func newTokenHandlerForTest(t *testing.T, fixture *tokenFixture) *documentHandler {
	t.Helper()
	// Serve the fixture's own docs directory so root identities match
	// the tokens the fixture minted.
	docs := fixture.docsDir
	resolved, err := files.Resolve(docs)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := newWorkspace([]files.Input{resolved}, files.DiscoverOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.assignIdentities(context.Background(), fixture.stack.dataStore.EnsureDocumentRoot); err != nil {
		t.Fatal(err)
	}
	handler := &documentHandler{
		workspace: workspace,
		auth:      fixture.stack,
	}
	return handler
}

func TestTokenRouteEdges(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")
	ref := fixtureDocumentRef(t, fixture)
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "edges",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "document", "rootId": ref.RootID, "path": "guide.md"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}
	bearer := func(method, path string) *http.Request {
		request, _ := http.NewRequest(method, fixture.server.URL+path, nil)
		request.Header.Set("Authorization", "Bearer "+created.Secret)
		return request
	}

	// File metadata follows the same scope rule as the document itself.
	if response := fixture.do(bearer(http.MethodGet, "/api/file-metadata?path=guide.md")); response.StatusCode != http.StatusOK {
		t.Fatalf("in-scope metadata = %d", response.StatusCode)
	}
	writeTestFile(t, fixture.docsDir+"/foreign.md", "# Foreign")
	if response := fixture.do(bearer(http.MethodGet, "/api/file-metadata?path=foreign.md")); response.StatusCode != http.StatusForbidden {
		t.Fatalf("out-of-scope metadata = %d, want 403", response.StatusCode)
	}

	// Annotations are session-only for tokens, whatever the scope: the
	// listing is a human review surface, not part of document reading.
	if response := fixture.do(bearer(http.MethodGet, "/api/annotations?path=guide.md")); response.StatusCode != http.StatusForbidden {
		t.Fatalf("in-scope annotations = %d, want 403", response.StatusCode)
	}
	if response := fixture.do(bearer(http.MethodGet, "/api/annotations?path=foreign.md")); response.StatusCode != http.StatusForbidden {
		t.Fatalf("out-of-scope annotations = %d, want 403", response.StatusCode)
	}

	// Token management route guards: a bearer never reaches the handler
	// (403 first), a session sees the method guard.
	if response := fixture.do(bearer(http.MethodPut, "/api/tokens")); response.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT /api/tokens via bearer = %d", response.StatusCode)
	}
	sessionPut, _ := http.NewRequest(http.MethodPut, fixture.server.URL+"/api/tokens", nil)
	sessionPut.AddCookie(alice)
	sessionPut.Header.Set("Origin", fixture.server.URL)
	if response := fixture.do(sessionPut); response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("PUT /api/tokens via session = %d", response.StatusCode)
	}
	invalidRevoke, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/tokens/", nil)
	invalidRevoke.AddCookie(alice)
	invalidRevoke.Header.Set("Origin", fixture.server.URL)
	if response := fixture.do(invalidRevoke); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid revoke id = %d", response.StatusCode)
	}
	unknownRevoke, _ := http.NewRequest(http.MethodDelete, fixture.server.URL+"/api/tokens/no-such", nil)
	unknownRevoke.AddCookie(alice)
	unknownRevoke.Header.Set("Origin", fixture.server.URL)
	if response := fixture.do(unknownRevoke); response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown revoke = %d", response.StatusCode)
	}

	// Raw paths with traversal are refused before scope checks.
	if response := fixture.do(bearer(http.MethodGet, "/raw/../etc/passwd")); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("traversal raw = %d, want 400", response.StatusCode)
	}
	if response := fixture.do(bearer(http.MethodGet, "/api/document")); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("document without path = %d, want 400", response.StatusCode)
	}
}

// TestFilesExposeTokenScopeRootId pins the settings UI's real request
// sequence: the listing must expose each root's persistent identity, and
// that identity is what /api/tokens accepts for restricted scopes. The
// positional root id ("r0") is a URL artifact the scope validation
// rejects — a UI reading the wrong one fails on every creation.
func TestFilesExposeTokenScopeRootId(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")

	list, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
	list.AddCookie(alice)
	response := fixture.do(list)
	var payload struct {
		Roots []struct {
			ID     string `json:"id"`
			RootID string `json:"rootId"`
			Files  []struct {
				Path string `json:"path"`
			} `json:"files"`
		} `json:"roots"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(payload.Roots) != 1 {
		t.Fatalf("roots = %d", len(payload.Roots))
	}
	root := payload.Roots[0]
	if root.RootID == "" || root.RootID == root.ID {
		t.Fatalf("listing must expose the persistent identity, got id=%q rootId=%q", root.ID, root.RootID)
	}

	// Both restricted scopes mint from the listing's identity.
	for name, scope := range map[string]map[string]any{
		"root": {
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "root", "rootId": root.RootID},
		},
		"document": {
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "document", "rootId": root.RootID, "path": "guide.md"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			status, _ := fixture.createTokenViaSession(t, alice, map[string]any{
				"kind": "persistent", "name": "ui-sequence", "scope": scope,
			})
			if status != http.StatusCreated {
				t.Fatalf("restricted scope from listing identity = %d, want 201", status)
			}
		})
	}
}

func TestCreateRootScopedToken(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")
	ref := fixtureDocumentRef(t, fixture)

	// Root scopes carry no path — creation must accept them and the
	// resulting token must read every document under the root.
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "root-token",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "root", "rootId": ref.RootID},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("root-scoped create = %d, want 201", status)
	}
	writeTestFile(t, fixture.docsDir+"/another.md", "# Another")
	read, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/raw/another.md", nil)
	read.Header.Set("Authorization", "Bearer "+created.Secret)
	if response := fixture.do(read); response.StatusCode != http.StatusOK {
		t.Fatalf("root token reading sibling = %d", response.StatusCode)
	}

	// Document scopes normalize the stored path: a client-side
	// non-canonical spelling must still mint a usable token.
	status, normalized := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "doc-token",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource": map[string]any{
				"type": "document", "rootId": ref.RootID, "path": "./guide.md",
			},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("non-canonical document create = %d, want 201", status)
	}
	if normalized.Scope.Resource.Path != "guide.md" {
		t.Fatalf("stored path = %q, want normalized guide.md", normalized.Scope.Resource.Path)
	}
	read, _ = http.NewRequest(http.MethodGet, fixture.server.URL+"/raw/guide.md", nil)
	read.Header.Set("Authorization", "Bearer "+normalized.Secret)
	if response := fixture.do(read); response.StatusCode != http.StatusOK {
		t.Fatalf("normalized document token = %d", response.StatusCode)
	}
}

func TestBearerCannotProbeDeepLinkExistence(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")
	ref := fixtureDocumentRef(t, fixture)
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "persistent",
		"name": "doc-only",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "document", "rootId": ref.RootID, "path": "guide.md"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}
	// A token has no use for the HTML shell; it must not learn whether
	// out-of-scope documents exist through /doc/ status codes either.
	writeTestFile(t, fixture.docsDir+"/secret.md", "# Secret")
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/doc/secret.md", nil)
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	if response := fixture.do(request); response.StatusCode != http.StatusForbidden {
		t.Fatalf("bearer deep link = %d, want 403", response.StatusCode)
	}
}

func TestContentBoundTokenCoversMetadata(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")
	hash := fixtureDocumentHash(t, fixture)
	ref := fixtureDocumentRef(t, fixture)
	status, created := fixture.createTokenViaSession(t, alice, map[string]any{
		"kind": "temporary",
		"name": "share",
		"scope": map[string]any{
			"permissions": []string{"document:read"},
			"resource":    map[string]any{"type": "document", "rootId": ref.RootID, "path": "guide.md"},
		},
		"documentHash": hash,
	})
	if status != http.StatusCreated {
		t.Fatalf("create = %d", status)
	}
	writeTestFile(t, fixture.docsDir+"/guide.md", "# Edited\n\nNew body.")
	request, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/file-metadata?path=guide.md", nil)
	request.Header.Set("Authorization", "Bearer "+created.Secret)
	if response := fixture.do(request); response.StatusCode != http.StatusConflict {
		t.Fatalf("stale metadata read = %d, want 409", response.StatusCode)
	}
}
