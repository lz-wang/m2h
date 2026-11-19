package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/lz-wang/m2h/internal/auth"
	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/store"
)

func recentFrom(t *testing.T, handler http.Handler, target string) recentResponse {
	t.Helper()
	response := performRequest(handler, http.MethodGet, target)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", target, response.Code, response.Body.String())
	}
	var result recentResponse
	decodeJSON(t, response, &result)
	if result.Created == nil || result.Updated == nil {
		t.Fatalf("expected JSON arrays, got %s", response.Body.String())
	}
	return result
}

func writeRecentFile(t *testing.T, root, name, source string, day int) {
	t.Helper()
	filename := filepath.Join(root, name)
	writeTestFile(t, filename, source)
	mtime := time.Date(2025, time.January, day, 12, 0, 0, 0, time.Local)
	if err := os.Chtimes(filename, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func recentTestHandler(t *testing.T, root string) (*documentHandler, http.Handler) {
	t.Helper()
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{
			root:      canonicalDirectory(t, root),
			discovery: files.DiscoverOptions{Depth: 4, SkipHidden: true},
		}),
		ui: directoryTestUI(),
	}
	return handler, handler.routes(nil)
}

func TestRecentOrdersLogicalDatesWithStableTieBreakers(t *testing.T) {
	root := t.TempDir()
	writeRecentFile(t, root, "a.md", "---\ntitle: Explicit title\ncreate_date: 2026-11-05\ncreate_at: 2099-11-05\nupdate_time: 2026-11-10T10:00:00+08:00\n---\n# Ignored heading", 2)
	writeRecentFile(t, root, "b.md", "---\ncreate_date: 2026-11-05\nupdate_at: 2026-11-09\n---\n# B heading", 2)
	writeRecentFile(t, root, "c.md", "---\ncreate_date: 2026-02-30\ncreate_at: 2026-11-05\nupdate_date: 2026-11-11\n---\n# C heading", 3)
	writeRecentFile(t, root, "d.md", "---\ncreate_date: invalid\nupdate_date: invalid\ndate: 2099-12-31\n---\n# D heading", 4)
	writeRecentFile(t, root, "e.md", "---\ntitle: [broken\n---\n# E heading", 1)
	writeRecentFile(t, root, "filename.md", "## No H1", 1)

	handler, routed := recentTestHandler(t, root)
	got := recentFrom(t, routed, "/api/recent")
	wantCreated := []recentDocument{
		{"c.md", "C heading", "2026-11-05", "frontmatter"},
		{"a.md", "Explicit title", "2026-11-05", "frontmatter"},
		{"b.md", "B heading", "2026-11-05", "frontmatter"},
		{"d.md", "D heading", "2025-01-04", "mtime"},
		{"e.md", "E heading", "2025-01-01", "mtime"},
		{"filename.md", "filename.md", "2025-01-01", "mtime"},
	}
	wantUpdated := []recentDocument{
		{"c.md", "C heading", "2026-11-11", "frontmatter"},
		{"a.md", "Explicit title", "2026-11-10", "frontmatter"},
		{"b.md", "B heading", "2026-11-09", "frontmatter"},
		wantCreated[3], wantCreated[4], wantCreated[5],
	}
	if !reflect.DeepEqual(got.Created, wantCreated) || !reflect.DeepEqual(got.Updated, wantUpdated) {
		t.Fatalf("unexpected result: %+v", got)
	}

	// A checkout can change a file's mtime. It affects tie-breaking only
	// when the entry has a valid logical date for the list.
	checkout := time.Date(2027, time.January, 1, 12, 0, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(root, "a.md"), checkout, checkout); err != nil {
		t.Fatal(err)
	}
	handler.recent.expiresAt = time.Time{}
	got = recentFrom(t, routed, "/api/recent")
	wantCreated[0], wantCreated[1] = wantCreated[1], wantCreated[0]
	if !reflect.DeepEqual(got.Created, wantCreated) || !reflect.DeepEqual(got.Updated, wantUpdated) {
		t.Fatalf("checkout changed the logical date: %+v", got)
	}
}

func TestRecentCacheRefreshesAndDropsUnavailableEntries(t *testing.T) {
	root := t.TempDir()
	for index := range 12 {
		writeRecentFile(t, root, fmt.Sprintf("%02d.md", index), fmt.Sprintf("# Note %d", index), index+1)
	}
	handler, routed := recentTestHandler(t, root)
	first := recentFrom(t, routed, "/api/recent")
	if len(first.Created) != 10 || len(first.Updated) != 10 || first.Created[0].Path != "11.md" {
		t.Fatalf("expected ten newest files, got %+v", first)
	}
	untouched := handler.recent.files["00.md"].info

	if err := os.Remove(filepath.Join(root, "11.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "10.md"), filepath.Join(root, "renamed.md")); err != nil {
		t.Fatal(err)
	}
	warm := recentFrom(t, routed, "/api/recent")
	if len(warm.Created) != 10 || warm.Created[0].Path != "09.md" || warm.Created[9].Path != "00.md" {
		t.Fatalf("warm lists must omit old paths and backfill the page: %+v", warm)
	}
	if !reflect.DeepEqual(warm.Created, warm.Updated) {
		t.Fatalf("warm updated list disagrees with created: %+v", warm)
	}

	writeRecentFile(t, root, "09.md", "# Edited and refreshed", 13)
	handler.recent.expiresAt = time.Time{}
	refreshed := recentFrom(t, routed, "/api/recent")
	if len(refreshed.Created) != 10 || refreshed.Created[0].Title != "Edited and refreshed" || refreshed.Created[1].Path != "renamed.md" {
		t.Fatalf("refresh did not update edited and renamed documents: %+v", refreshed)
	}
	if handler.recent.files["00.md"].info != untouched {
		t.Fatal("unchanged file did not reuse cached metadata")
	}
}

func TestRecentConcurrentRequestsShareDiscovery(t *testing.T) {
	root := t.TempDir()
	writeRecentFile(t, root, "a.md", "# A", 1)
	handler, routed := recentTestHandler(t, root)
	var discoveries atomic.Int32
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		discoveries.Add(1)
		return scope.discover(ctx)
	}
	start := make(chan struct{})
	var requests sync.WaitGroup
	for range 8 {
		requests.Go(func() {
			<-start
			response := performRequest(routed, http.MethodGet, "/api/recent")
			var result recentResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil ||
				response.Code != http.StatusOK || len(result.Created) != 1 || len(result.Updated) != 1 {
				t.Errorf("unexpected concurrent response: %d %s", response.Code, response.Body.String())
			}
		})
	}
	close(start)
	requests.Wait()
	if discoveries.Load() != 1 {
		t.Fatalf("discovery ran %d times, want one", discoveries.Load())
	}
}

func TestRecentSharesPublishingRules(t *testing.T) {
	workspace, root := ignoreWorkspace(t)
	writeTestFile(t, filepath.Join(root, ".hidden.md"), "# Hidden")
	routed := newDocumentHandler(workspace, nil, directoryTestUI())
	want := map[string]bool{"README.md": true, "guide.md": true, "notes/public.md": true}
	assertVisible := func() {
		t.Helper()
		result := recentFrom(t, routed, "/api/recent")
		for _, list := range [][]recentDocument{result.Created, result.Updated} {
			if len(list) != len(want) {
				t.Fatalf("unexpected published files: %+v", list)
			}
			for _, entry := range list {
				if !want[entry.Path] {
					t.Fatalf("unpublished path %q in %+v", entry.Path, list)
				}
			}
		}
	}
	assertVisible()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "guide.md\ndraft/\nsecret/\n")
	delete(want, "guide.md")
	assertVisible()

	// An unusable ignore file must fail closed even with warm metadata.
	ignorePath := filepath.Join(root, ".gitignore")
	if err := os.Remove(ignorePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(ignorePath, 0o755); err != nil {
		t.Fatal(err)
	}
	assertJSONError(t, performRequest(routed, http.MethodGet, "/api/recent"), http.StatusInternalServerError)
}

func TestRecentEndpointContract(t *testing.T) {
	handler, routed := recentTestHandler(t, t.TempDir())
	empty := recentFrom(t, routed, "/api/recent")
	if len(empty.Created) != 0 || len(empty.Updated) != 0 {
		t.Fatalf("empty workspace returned %+v", empty)
	}
	assertJSONError(t, performRequest(routed, http.MethodGet, "/api/recent?limit=1"), http.StatusBadRequest)
	assertJSONError(t, performRequest(routed, http.MethodPost, "/api/recent"), http.StatusMethodNotAllowed)

	handler.recent.expiresAt = time.Time{}
	handler.discover = func(context.Context, rootScope) (files.Discovery, error) {
		return files.Discovery{}, fs.ErrNotExist
	}
	recentFrom(t, routed, "/api/recent")
	handler.recent.expiresAt = time.Time{}
	handler.discover = func(context.Context, rootScope) (files.Discovery, error) {
		return files.Discovery{}, errors.New("fixture discovery failed")
	}
	assertJSONError(t, performRequest(routed, http.MethodGet, "/api/recent"), http.StatusInternalServerError)
}

func TestRecentCancelsBeforePublishingPartialSnapshot(t *testing.T) {
	root := t.TempDir()
	writeRecentFile(t, root, "a.md", "# A", 1)
	handler, routed := recentTestHandler(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	discover := handler.discover
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		discovered, err := discover(ctx, scope)
		cancel()
		return discovered, err
	}
	response := httptest.NewRecorder()
	routed.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/recent", nil).WithContext(ctx))
	if response.Body.Len() != 0 || handler.recent.files != nil {
		t.Fatal("canceled request returned or stored a partial snapshot")
	}
	handler.discover = discover
	if got := recentFrom(t, routed, "/api/recent"); len(got.Created) != 1 {
		t.Fatalf("retry failed after cancellation: %+v", got)
	}
}

func TestRecentEndpointUsesVirtualPathsUnderBasePath(t *testing.T) {
	workspace := multiRootFixture(t)
	routed := newDocumentHandlerWithConfig(workspace, nil, directoryTestUI(), "test", false, "/docs")
	got := recentFrom(t, routed, "/docs/api/recent")
	paths := map[string]bool{}
	for _, entry := range got.Created {
		paths[entry.Path] = true
	}
	if !paths["r0/README.md"] || !paths["r1/README.md"] {
		t.Fatalf("expected both virtual README paths: %+v", got)
	}
}

func TestRecentPrefersReadingOnlyTheHeader(t *testing.T) {
	large := strings.Repeat("Body text\n", recentHeaderSize/4)
	header := strings.NewReader("---\ntitle: Metadata title\n---\n" + large)
	body, meta, err := readRecentFrontMatter(io.MultiReader(header, iotest.ErrReader(errors.New("read beyond header"))))
	if err != nil || len(body) != 0 || meta == nil || meta.Title != "Metadata title" {
		t.Fatalf("header-only read failed: body length %d, metadata %+v, error %v", len(body), meta, err)
	}

	root := t.TempDir()
	writeRecentFile(t, root, "late.md", large+"\n# Late heading\n", 1)
	_, routed := recentTestHandler(t, root)
	result := recentFrom(t, routed, "/api/recent")
	if len(result.Created) != 1 || result.Created[0].Title != "Late heading" {
		t.Fatalf("late heading fallback failed: %+v", result)
	}
}

func TestRecentDocumentTokenSkipsWorkspaceDiscovery(t *testing.T) {
	workspace := multiRootFixture(t)
	workspace.roots[0].identity = "first"
	workspace.roots[1].identity = "second"
	handler := &documentHandler{
		workspace: workspace,
		discover: func(context.Context, rootScope) (files.Discovery, error) {
			t.Error("document-scoped recent request must not discover the workspace")
			return files.Discovery{}, errors.New("unexpected workspace discovery")
		},
	}

	scope := auth.TokenScope{Permissions: []string{auth.PermissionDocumentRead}}
	scope.Resource.Type = auth.ResourceDocument
	scope.Resource.RootID = "second"
	scope.Resource.Path = "README.md"
	encoded, err := auth.MarshalScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/recent", nil)
	record := store.TokenRecord{ScopeJSON: encoded}
	request = request.WithContext(context.WithValue(request.Context(), tokenRecordContextKey{}, record))
	request = requestWithPrincipal(request, &principal{Via: principalViaToken})
	response := httptest.NewRecorder()
	handler.serveRecent(response, request)

	var got recentResponse
	decodeJSON(t, response, &got)
	for _, list := range [][]recentDocument{got.Created, got.Updated} {
		if response.Code != http.StatusOK || len(list) != 1 || list[0].Path != "r1/README.md" {
			t.Fatalf("expected only the scoped virtual document: %d %+v", response.Code, got)
		}
	}
	if handler.recent.files != nil {
		t.Fatal("document-scoped request populated the shared cache")
	}
}

func TestRecentTokenPermissionsAndVersionBinding(t *testing.T) {
	fixture := newTokenFixture(t)
	alice := fixture.loginAs(t, "alice", "password-1")
	for index := range 12 {
		writeTestFile(t, filepath.Join(fixture.docsDir, fmt.Sprintf("future-%02d.md", index)),
			"---\ncreate_date: 9999-01-01\nupdate_date: 9999-01-01\n---\n# Another note")
	}
	ref := fixtureDocumentRef(t, fixture)
	mint := func(kind string, permissions []string, resource map[string]any, hash string) string {
		t.Helper()
		status, token := fixture.createTokenViaSession(t, alice, map[string]any{
			"kind":         kind,
			"name":         "recent test",
			"scope":        map[string]any{"permissions": permissions, "resource": resource},
			"documentHash": hash,
		})
		if status != http.StatusCreated {
			t.Fatalf("token creation: %d", status)
		}
		return token.Secret
	}
	get := func(secret string, cookie *http.Cookie, wantStatus int) recentResponse {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, fixture.server.URL+"/api/recent", nil)
		if err != nil {
			t.Fatal(err)
		}
		if secret != "" {
			request.Header.Set("Authorization", "Bearer "+secret)
		}
		if cookie != nil {
			request.AddCookie(cookie)
		}
		response := fixture.do(request)
		defer response.Body.Close()
		if response.StatusCode != wantStatus {
			t.Fatalf("GET recent status = %d, want %d", response.StatusCode, wantStatus)
		}
		var result recentResponse
		if wantStatus == http.StatusOK {
			if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}

	get("", nil, http.StatusUnauthorized)
	warm := get("", alice, http.StatusOK)
	if len(warm.Created) != 10 || warm.Created[9].Date != "9999-01-01" {
		t.Fatalf("Guide must rank outside the unfiltered results: %+v", warm)
	}
	read := []string{"document:read"}
	guide := map[string]any{"type": "document", "rootId": ref.RootID, "path": "guide.md"}
	scoped := mint("persistent", read, guide, "")
	assertGuide := func(got recentResponse) {
		t.Helper()
		for _, list := range [][]recentDocument{got.Created, got.Updated} {
			if len(list) != 1 || list[0].Path != "guide.md" {
				t.Fatalf("expected one authorized document: %+v", got)
			}
		}
	}
	assertGuide(get(scoped, nil, http.StatusOK))

	searchOnly := mint("persistent", []string{"search:read"}, map[string]any{"type": "all_published"}, "")
	get(searchOnly, nil, http.StatusForbidden)

	bound := mint("temporary", read, guide, fixtureDocumentHash(t, fixture))
	assertGuide(get(bound, nil, http.StatusOK))
	path := filepath.Join(fixture.docsDir, "guide.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original[len(original)-1] ^= 1
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := get(bound, nil, http.StatusOK); len(got.Created) != 0 || len(got.Updated) != 0 {
		t.Fatalf("bound token saw a different version: %+v", got)
	}
}
