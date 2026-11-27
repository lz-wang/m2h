package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

const shareTestOrigin = "https://docs.example.test"

const shareTestBase = "/docs"

type shareFixture struct {
	docs, other string
	dataStore   *store.Store
	workspace   workspace
	logs        bytes.Buffer
	handler     http.Handler
}

func newShareFixture(t *testing.T, enabled bool) *shareFixture {
	t.Helper()
	fixture := &shareFixture{
		docs: filepath.Join(t.TempDir(), "docs"), other: filepath.Join(t.TempDir(), "other"),
	}
	writeTestFile(t, filepath.Join(fixture.docs, "guide.md"), `---
title: Public guide
private_tag: private-metadata
---
# Guide

![Local](pictures/image%20one.png)

[Download](asset.txt) [Other](private.md) [Section](#guide)

<picture><source srcset="pictures/image%20one.png 1x, pictures/high.png 2x"><img src="pictures/image%20one.png" onerror="bad()"></picture>

<iframe src="frame.txt"></iframe>
<video poster="pictures/high.png" controls><source src="movie.mp4"></video>

<script>bad()</script>
<a href="javascript:bad()">unsafe link</a>

[hidden](.secret.txt) [ignored](ignored.txt) [hidden alias](alias.txt)
`)
	writeTestFile(t, filepath.Join(fixture.docs, "private.md"), "# Private")
	writeTestFile(t, filepath.Join(fixture.docs, "asset.txt"), "asset contents")
	writeTestFile(t, filepath.Join(fixture.docs, "frame.txt"), "frame contents")
	writeTestFile(t, filepath.Join(fixture.docs, "movie.mp4"), "movie contents")
	writeTestFile(t, filepath.Join(fixture.docs, "pictures", "image one.png"), "picture one")
	writeTestFile(t, filepath.Join(fixture.docs, "pictures", "high.png"), "picture high")
	writeTestFile(t, filepath.Join(fixture.docs, ".secret.txt"), "hidden")
	writeTestFile(t, filepath.Join(fixture.docs, ".gitignore"), "ignored.txt\n")
	writeTestFile(t, filepath.Join(fixture.docs, "ignored.txt"), "ignored")
	if err := os.Symlink(".secret.txt", filepath.Join(fixture.docs, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(fixture.other, "guide.md"), "# Other root")
	var inputs []files.Input
	for _, name := range []string{fixture.docs, fixture.other} {
		input, err := files.Resolve(name)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input)
	}
	workspace, err := newWorkspace(inputs, files.DiscoverOptions{Depth: 4}, true)
	if err != nil {
		t.Fatal(err)
	}
	dataStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	ctx := context.Background()
	if err := dataStore.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := workspace.assignIdentities(ctx, dataStore.EnsureDocumentRoot); err != nil {
		t.Fatal(err)
	}
	fixture.workspace = workspace
	fixture.dataStore = dataStore
	fixture.rebuild(t, enabled, "alice", "bob")
	return fixture
}

func (fixture *shareFixture) rebuild(t *testing.T, enabled bool, users ...string) {
	t.Helper()
	dataStore := fixture.dataStore
	config := &auth.Config{Mode: auth.ModeLocal, PublicURL: shareTestOrigin + shareTestBase}
	for _, name := range users {
		config.LocalUsers = append(config.LocalUsers, auth.User{Username: name, Password: "test-password"})
	}
	stack, err := buildAuthStack(config, dataStore, shareTestBase)
	if err != nil {
		t.Fatal(err)
	}
	stack.publicShares = enabled
	if err := stack.pruneShareOwners(context.Background(), fixture.workspace); err != nil {
		t.Fatal(err)
	}
	ui := fstest.MapFS{"index.html": {Data: []byte(`<!doctype html><head></head><div id="root"></div>`)}}
	fixture.handler = newDocumentHandlerWithAuth(context.Background(), fixture.workspace, &fixture.logs, ui, "test", false, shareTestBase, stack)
}

func (fixture *shareFixture) call(method, path string, body string, cookie *http.Cookie, origin, bearer string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, shareTestOrigin+shareTestBase+path, strings.NewReader(body))
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	return response
}

func (fixture *shareFixture) login(t *testing.T, username string) *http.Cookie {
	t.Helper()
	response := fixture.call(http.MethodPost, "/api/auth/login",
		`{"username":"`+username+`","password":"test-password"}`, nil, shareTestOrigin, "")
	expectShareStatus(t, response, http.StatusOK)
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == auth.SessionCookieName {
			return cookie
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func expectShareStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("HTTP %d, want %d: %s", response.Code, want, response.Body)
	}
}

func (fixture *shareFixture) create(t *testing.T, cookie *http.Cookie, path, duration string, want int) shareResponse {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"documentRef": documentRefRequest{RootID: fixture.workspace.primary().identity, Path: path},
		"duration":    duration,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := fixture.call(http.MethodPost, "/api/shares", string(body), cookie, shareTestOrigin, "")
	expectShareStatus(t, response, want)
	var share shareResponse
	if want == http.StatusCreated || want == http.StatusOK {
		decodeJSON(t, response, &share)
	}
	return share
}

func TestPublicShareLifecycleAndIsolation(t *testing.T) {
	fixture := newShareFixture(t, true)
	alice := fixture.login(t, "alice")
	bob := fixture.login(t, "bob")
	config := fixture.call(http.MethodGet, "/api/config", "", nil, "", "")
	if !strings.Contains(config.Body.String(), `"shares":true`) {
		t.Fatalf("missing capability: %s", config.Body)
	}
	expectShareStatus(t, fixture.call(http.MethodPost, "/api/shares", "{}", alice, "https://other.test", ""), http.StatusForbidden)
	expectShareStatus(t, fixture.call(http.MethodGet, "/api/shares", "", nil, "", ""), http.StatusUnauthorized)
	expectShareStatus(t, fixture.call(http.MethodPost, "/api/shares", "{}", alice, shareTestOrigin, "invalid"), http.StatusUnauthorized)
	fixture.create(t, alice, "guide.md", "2 min", http.StatusBadRequest)
	fixture.create(t, alice, ".private.md", "", http.StatusNotFound)
	share := fixture.create(t, alice, "guide.md", "", http.StatusCreated)
	if !validShareID(share.ID) || share.URL != shareTestOrigin+shareTestBase+sharePagePrefix+share.ID ||
		share.Title != "Public guide" || share.ExpiresAt-share.CreatedAt != 30*60_000 {
		t.Fatalf("wrong share: %+v", share)
	}
	if existing := fixture.create(t, alice, "guide.md", "permanent", http.StatusOK); existing != share {
		t.Fatalf("duplicate creation changed share: %+v", existing)
	}
	for _, who := range []struct {
		cookie *http.Cookie
		count  int
	}{{alice, 1}, {bob, 0}} {
		response := fixture.call(http.MethodGet, "/api/shares", "", who.cookie, "", "")
		expectShareStatus(t, response, http.StatusOK)
		var items []shareResponse
		decodeJSON(t, response, &items)
		if len(items) != who.count {
			t.Fatalf("list length = %d, want %d", len(items), who.count)
		}
	}
	page := sharePagePrefix + share.ID
	document := shareDocumentPrefix + share.ID
	for _, path := range []string{page, document, page + "/assets/pictures/image%20one.png"} {
		response := fixture.call(http.MethodGet, path, "", nil, "", "")
		expectShareStatus(t, response, http.StatusOK)
		if response.Header().Get("Cache-Control") != "no-store" ||
			response.Header().Get("Referrer-Policy") != "no-referrer" ||
			response.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("missing public response headers: %v", response.Header())
		}
	}
	response := fixture.call(http.MethodGet, document, "", nil, "", "")
	var public publicDocumentResponse
	decodeJSON(t, response, &public)
	if public.Title != "Public guide" || len(public.TOC) != 1 {
		t.Fatalf("wrong public metadata: %+v", public)
	}
	for _, want := range []string{"asset.txt", "image%20one.png", "high.png", "#guide", "movie.mp4", "Download"} {
		if !strings.Contains(public.HTML, want) {
			t.Errorf("public HTML missing %q", want)
		}
	}
	for _, forbidden := range []string{"private.md", "private-metadata", "frame.txt", "bad()", "onerror", ".secret.txt", "ignored.txt", "alias.txt"} {
		if strings.Contains(public.HTML, forbidden) || strings.Contains(response.Body.String(), forbidden) {
			t.Errorf("public response contains %q", forbidden)
		}
	}
	for _, name := range []string{"private.md", "frame.txt", ".secret.txt", "ignored.txt", "alias.txt", "%2E%2E/asset.txt"} {
		expectShareStatus(t, fixture.call(http.MethodGet, page+"/assets/"+name, "", nil, "", ""), http.StatusNotFound)
	}
	for _, path := range []string{"/api/files", "/api/search?q=guide", "/api/annotations", "/api/document?path=r0/guide.md", "/raw/r0/guide.md", "/assets/r0/asset.txt"} {
		expectShareStatus(t, fixture.call(http.MethodGet, path, "", nil, "", ""), http.StatusUnauthorized)
	}
	expectShareStatus(t, fixture.call(http.MethodHead, page+"/assets/asset.txt", "", nil, "", ""), http.StatusOK)
	expectShareStatus(t, fixture.call(http.MethodPost, page, "", nil, "", ""), http.StatusMethodNotAllowed)
	expectShareStatus(t, fixture.call(http.MethodPost, document, "", nil, "", ""), http.StatusMethodNotAllowed)
	expectShareStatus(t, fixture.call(http.MethodDelete, shareManagementPrefix+share.ID, "", bob, shareTestOrigin, ""), http.StatusNotFound)

	writeTestFile(t, filepath.Join(fixture.docs, "guide.md"), "# Updated")
	response = fixture.call(http.MethodGet, document, "", nil, "", "")
	expectShareStatus(t, response, http.StatusOK)
	if !strings.Contains(response.Body.String(), "Updated") {
		t.Fatal("link did not read updated document")
	}
	expectShareStatus(t, fixture.call(http.MethodGet, page+"/assets/asset.txt", "", nil, "", ""), http.StatusNotFound)

	fixture.workspace.roots[0], fixture.workspace.roots[1] = fixture.workspace.roots[1], fixture.workspace.roots[0]
	fixture.workspace.roots[0].id, fixture.workspace.roots[1].id = "r0", "r1"
	fixture.rebuild(t, true, "alice", "bob")
	response = fixture.call(http.MethodGet, document, "", nil, "", "")
	expectShareStatus(t, response, http.StatusOK)
	if !strings.Contains(response.Body.String(), "Updated") {
		t.Fatal("link moved to a different root after rebuild")
	}
	expectShareStatus(t, fixture.call(http.MethodDelete, shareManagementPrefix+share.ID, "", alice, shareTestOrigin, ""), http.StatusNoContent)
	expectShareStatus(t, fixture.call(http.MethodGet, document, "", nil, "", ""), http.StatusNotFound)
	expectShareStatus(t, fixture.call(http.MethodGet, page, "", nil, "", ""), http.StatusNotFound)
	if strings.Contains(fixture.logs.String(), share.ID) || !strings.Contains(fixture.logs.String(), "[redacted]") {
		t.Fatal("request log exposed the link credential")
	}
}

func TestPublicShareOwnerRemovalIsPermanent(t *testing.T) {
	fixture := newShareFixture(t, true)
	share := fixture.create(t, fixture.login(t, "alice"), "guide.md", "permanent", http.StatusCreated)
	path := shareDocumentPrefix + share.ID
	fixture.rebuild(t, false, "alice", "bob")
	expectShareStatus(t, fixture.call(http.MethodGet, path, "", nil, "", ""), http.StatusUnauthorized)
	fixture.rebuild(t, true, "bob")
	expectShareStatus(t, fixture.call(http.MethodGet, path, "", nil, "", ""), http.StatusNotFound)
	fixture.rebuild(t, true, "alice", "bob")
	expectShareStatus(t, fixture.call(http.MethodGet, path, "", nil, "", ""), http.StatusNotFound)
}
