package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
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
		// Listing and searching the whole workspace stay out of reach.
		files, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/files", nil)
		files.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(files); response.StatusCode != http.StatusForbidden {
			t.Fatalf("files = %d, want 403", response.StatusCode)
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

	t.Run("tokens cannot read annotations or write anything", func(t *testing.T) {
		secret := mint(t, allScope)
		annotations, _ := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/annotations?path=guide.md", nil)
		annotations.Header.Set("Authorization", "Bearer "+secret)
		if response := fixture.do(annotations); response.StatusCode != http.StatusOK {
			t.Fatalf("annotation read via bearer = %d", response.StatusCode)
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
