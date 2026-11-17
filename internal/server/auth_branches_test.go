package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/store"
)

// requestWithSession builds a same-origin request carrying the session.
func (fixture *authedTestServer) requestWithSession(method, path string, session *http.Cookie, body []byte) *http.Request {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request, _ := http.NewRequest(method, fixture.server.URL+path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Origin", fixture.server.URL)
	if session != nil {
		request.AddCookie(session)
	}
	return request
}

func TestAuthEndpointMethodGuards(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())
	session := fixture.loginAs(t, "alice", "password-1")

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/login"},
		{http.MethodGet, "/api/auth/logout"},
		{http.MethodPost, "/api/auth/me"},
		{http.MethodPut, "/api/annotations"},
		{http.MethodPut, "/api/annotations/xyz"},
		{http.MethodPut, "/api/annotations/xyz/anchor"},
		{http.MethodPost, "/api/config"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request, _ := http.NewRequest(test.method, fixture.server.URL+test.path, strings.NewReader("{}"))
			request.Header.Set("Origin", fixture.server.URL)
			request.AddCookie(session)
			response := fixture.do(request)
			response.Body.Close()
			if response.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s = %d, want 405", test.method, test.path, response.StatusCode)
			}
		})
	}
}

func TestAnnotationUnauthenticatedAndInvalidPaths(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	// Unauthenticated writes never reach the handlers.
	for _, probe := range []struct{ method, path string }{
		{http.MethodDelete, "/api/annotations/whatever"},
		{http.MethodPatch, "/api/annotations/whatever/anchor"},
	} {
		request, _ := http.NewRequest(probe.method, fixture.server.URL+probe.path, strings.NewReader("{}"))
		request.Header.Set("Origin", fixture.server.URL)
		response := fixture.do(request)
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s anonymous = %d, want 401", probe.method, probe.path, response.StatusCode)
		}
	}

	alice := fixture.loginAs(t, "alice", "password-1")

	// Malformed ids are refused before any lookup.
	response := fixture.do(fixture.requestWithSession(http.MethodDelete, "/api/annotations/", alice, nil))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty id delete = %d, want 400", response.StatusCode)
	}

	// Unknown annotation ids miss plainly.
	response = fixture.do(fixture.requestWithSession(http.MethodDelete, "/api/annotations/no-such-id", alice, nil))
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown id delete = %d, want 404", response.StatusCode)
	}

	// Malformed bodies fail as bad requests.
	response = fixture.do(fixture.requestWithSession(http.MethodPost, "/api/annotations", alice, []byte("not json")))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed create body = %d, want 400", response.StatusCode)
	}
	response = fixture.do(fixture.requestWithSession(http.MethodPatch, "/api/annotations/some-id/anchor", alice, []byte("not json")))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed re-anchor body = %d, want 400", response.StatusCode)
	}

	// Listing demands exactly one path parameter.
	response = fixture.do(fixture.requestWithSession(http.MethodGet, "/api/annotations", alice, nil))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("list without path = %d, want 400", response.StatusCode)
	}
	response = fixture.do(fixture.requestWithSession(http.MethodGet, "/api/annotations?path=..%2F..%2Fetc", alice, nil))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("list with traversal path = %d, want 400", response.StatusCode)
	}
}

func TestServeConfigHeadAllowed(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())
	response, err := http.Head(fixture.server.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HEAD config = %d", response.StatusCode)
	}
}

func TestPublicOriginOf(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "https://docs.example.com/docs", want: "https://docs.example.com"},
		{value: "http://host:8080", want: "http://host:8080"},
		{value: "https://plain", want: "https://plain"},
	}
	for _, test := range tests {
		if got := publicOriginOf(test.value); got != test.want {
			t.Fatalf("publicOriginOf(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestBuildAuthStackLDAPBranch(t *testing.T) {
	base := t.TempDir()
	dataStore, err := store.Open(base)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer dataStore.Close()
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	config := &auth.Config{
		Mode: auth.ModeLDAP,
		LDAP: auth.LDAPConfig{
			URL:          "ldaps://ldap.example.internal:636",
			BindDN:       "uid=service,ou=people,dc=example,dc=internal",
			BindPassword: "secret",
			UserBaseDN:   "ou=people,dc=example,dc=internal",
			UserFilter:   "(uid=%s)",
			AllowedUsers: []string{"alice"},
		},
		PublicURL: "https://docs.example.com/docs",
	}
	stack, err := buildAuthStack(config, dataStore, "/")
	if err != nil {
		t.Fatalf("buildAuthStack(ldap) failed: %v", err)
	}
	if stack.mode != auth.ModeLDAP {
		t.Fatalf("mode = %q", stack.mode)
	}
	if !stack.sessions.Cookie.Secure {
		t.Fatalf("https public URL must set the Secure cookie flag")
	}
	if stack.publicOrigin != "https://docs.example.com" {
		t.Fatalf("publicOrigin = %q", stack.publicOrigin)
	}

	// A none-mode config has no authenticator and is refused.
	if _, err := buildAuthStack(&auth.Config{Mode: auth.ModeNone}, dataStore, "/"); err == nil {
		t.Fatalf("none mode accepted by buildAuthStack")
	}
}

func TestSameOriginEdgeCases(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "host only origin", origin: "http://" + strings.TrimPrefix(fixture.server.URL, "http://"), want: true},
		{name: "empty scheme", origin: "://nope", want: false},
		{name: "malformed referer", origin: "", want: false},
	}
	for _, test := range tests {
		request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/logout", nil)
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		} else {
			request.Header.Set("Referer", "not a url")
		}
		allowed := (&documentHandler{auth: fixture.stack}).sameOrigin(request)
		if allowed != test.want {
			t.Fatalf("%s: sameOrigin = %v, want %v", test.name, allowed, test.want)
		}
	}
}

func TestLoginRejectsMalformedBodies(t *testing.T) {
	fixture := newAuthedTestServer(t, localAuthConfig())

	request, _ := http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login", strings.NewReader("{"))
	request.Header.Set("Origin", fixture.server.URL)
	response := fixture.do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed login = %d, want 400", response.StatusCode)
	}

	// A login with a JSON array body fails the same way.
	request, _ = http.NewRequest(http.MethodPost, fixture.server.URL+"/api/auth/login",
		strings.NewReader(`["alice"]`))
	request.Header.Set("Origin", fixture.server.URL)
	response = fixture.do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("array login = %d, want 400", response.StatusCode)
	}
}

func TestAnnotationListDecodesAuthorFields(t *testing.T) {
	fixture := newAnnotationFixture(t)
	created := fixture.createAnnotation(t, fixture.alice, "身份字段")
	var entry annotationResponse
	_ = json.NewDecoder(created.Body).Decode(&entry)
	created.Body.Close()

	list := fixture.do(fixture.requestWithSession(http.MethodGet, "/api/annotations?path=guide.md", fixture.alice, nil))
	entries := decodeAnnotations(t, list)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].AuthorID != "local:alice" || entries[0].Selector.Version != 1 {
		t.Fatalf("entry = %+v", entries[0])
	}
}
