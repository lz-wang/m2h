package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/files"
)

// indexedSearchHandler builds a handler whose internal search index is
// reachable for white-box assertions: the tests below watch snapshot
// identity (which entries were reused by pointer, which were re-parsed)
// while going through the real HTTP surface for the queries themselves.
func indexedSearchHandler(t *testing.T, root string, discovery files.DiscoverOptions) (*documentHandler, http.Handler) {
	t.Helper()
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: discovery}),
		ui:        directoryTestUI(),
	}
	return handler, handler.routes(nil)
}

// indexedSearchQuery runs one search request and fails the test unless it
// answers 200.
func indexedSearchQuery(t *testing.T, handler http.Handler, query string) searchResponse {
	t.Helper()
	response := performRequest(handler, http.MethodGet, "/api/search?q="+url.QueryEscape(query))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/search?q=%s status = %d, body = %s", query, response.Code, response.Body.String())
	}
	return decodeSearch(t, response)
}

// touchForRefresh rewrites a file and pushes its mtime far forward, so the
// change is detectable through size or mtime no matter how coarse the
// filesystem's timestamps are.
func touchForRefresh(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(name, future, future); err != nil {
		t.Fatal(err)
	}
}

// Unchanged files keep their cached projection by pointer across refreshes;
// only the edited file is re-read and re-parsed; a deleted file leaves the
// next snapshot. This is the incremental contract every later refresh mode
// (TTL, background) is built on.
func TestSearchIndexReusesUnchangedDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	stable := filepath.Join(root, "stable.md")
	edited := filepath.Join(root, "edited.md")
	writeTestFile(t, stable, "# Stable\n\nstable-token\n")
	writeTestFile(t, edited, "# Edited\n\nold-token\n")
	handler, routed := indexedSearchHandler(t, root, files.DiscoverOptions{Depth: 2})

	if payload := indexedSearchQuery(t, routed, "old-token"); len(payload.Results) != 1 {
		t.Fatalf("initial results = %+v, want edited.md", payload.Results)
	}
	first := handler.search.snapshot.Load()

	// A query with no filesystem change publishes a fresh snapshot whose
	// entries are the very same pointers: nothing was re-parsed.
	if payload := indexedSearchQuery(t, routed, "stable-token"); len(payload.Results) != 1 {
		t.Fatalf("second query results = %+v, want stable.md", payload.Results)
	}
	second := handler.search.snapshot.Load()
	if second == first {
		t.Fatal("refresh did not publish a new snapshot")
	}
	for _, path := range []string{"stable.md", "edited.md"} {
		if first.Files[path] == nil || second.Files[path] == nil {
			t.Fatalf("snapshot lost %s: first=%v second=%v", path, first.Files, second.Files)
		}
		if first.Files[path] != second.Files[path] {
			t.Fatalf("%s was re-parsed although its file did not change", path)
		}
	}

	// Editing one file re-parses exactly that entry.
	touchForRefresh(t, edited, "# Edited\n\nnew-token\n")
	if payload := indexedSearchQuery(t, routed, "new-token"); len(payload.Results) != 1 || payload.Results[0].Path != "edited.md" {
		t.Fatalf("results after edit = %+v, want edited.md", payload.Results)
	}
	if payload := indexedSearchQuery(t, routed, "old-token"); len(payload.Results) != 0 {
		t.Fatalf("stale token survived the edit: %+v", payload.Results)
	}
	third := handler.search.snapshot.Load()
	if third.Files["edited.md"] == first.Files["edited.md"] {
		t.Fatal("edited entry was reused despite the file changing")
	}
	if third.Files["stable.md"] != first.Files["stable.md"] {
		t.Fatal("stable entry was re-parsed despite no change")
	}

	// Deleting a file removes it from the next snapshot.
	if err := os.Remove(edited); err != nil {
		t.Fatal(err)
	}
	if payload := indexedSearchQuery(t, routed, "new-token"); len(payload.Results) != 0 {
		t.Fatalf("deleted document still searchable: %+v", payload.Results)
	}
	fourth := handler.search.snapshot.Load()
	if _, exists := fourth.Files["edited.md"]; exists {
		t.Fatal("deleted document still indexed")
	}
	if fourth.Files["stable.md"] != first.Files["stable.md"] {
		t.Fatal("stable entry was re-parsed despite no change")
	}
}

// Files added after the first build join the index on the next refresh and
// are searchable, including when they appear in a directory no earlier pass
// had seen.
func TestSearchIndexPicksUpNewFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "first.md"), "# First\n\nfirst-token\n")
	_, routed := indexedSearchHandler(t, root, files.DiscoverOptions{Depth: 4})

	if payload := indexedSearchQuery(t, routed, "added-token"); len(payload.Results) != 0 {
		t.Fatalf("results before the file exists = %+v, want none", payload.Results)
	}

	writeTestFile(t, filepath.Join(root, "nested", "added.md"), "# Added\n\nadded-token\n")
	if payload := indexedSearchQuery(t, routed, "added-token"); len(payload.Results) != 1 || payload.Results[0].Path != "nested/added.md" {
		t.Fatalf("results after add = %+v, want nested/added.md", payload.Results)
	}
}

// Each root keeps its own public identity: an edit in one root re-parses
// only that root's entry and never bleeds into the sibling root's results.
func TestSearchIndexMultiRootUpdates(t *testing.T) {
	t.Parallel()

	rootA := t.TempDir()
	rootB := t.TempDir()
	writeTestFile(t, filepath.Join(rootA, "doc.md"), "# A\n\ndual-token alpha\n")
	writeTestFile(t, filepath.Join(rootB, "doc.md"), "# B\n\ndual-token beta\n")
	handler := &documentHandler{
		workspace: workspace{roots: []workspaceRoot{
			{id: "r0", label: "a", input: files.Input{Kind: files.KindDirectory, Path: rootA}, scope: rootScope{root: canonicalDirectory(t, rootA), discovery: files.DiscoverOptions{Depth: 2}}},
			{id: "r1", label: "b", input: files.Input{Kind: files.KindDirectory, Path: rootB}, scope: rootScope{root: canonicalDirectory(t, rootB), discovery: files.DiscoverOptions{Depth: 2}}},
		}},
		ui: directoryTestUI(),
	}
	routed := handler.routes(nil)

	if payload := indexedSearchQuery(t, routed, "dual-token"); len(payload.Results) != 2 {
		t.Fatalf("initial results = %+v, want one per root", payload.Results)
	}
	first := handler.search.snapshot.Load()

	touchForRefresh(t, filepath.Join(rootA, "doc.md"), "# A\n\ndual-token alpha-v2\n")
	if payload := indexedSearchQuery(t, routed, "alpha-v2"); len(payload.Results) != 1 || payload.Results[0].Path != "r0/doc.md" {
		t.Fatalf("results after r0 edit = %+v, want r0/doc.md", payload.Results)
	}
	if payload := indexedSearchQuery(t, routed, "beta"); len(payload.Results) != 1 || payload.Results[0].Path != "r1/doc.md" {
		t.Fatalf("r1 lost its identity after an r0 edit: %+v", payload.Results)
	}
	second := handler.search.snapshot.Load()
	if second.Files["r0/doc.md"] == first.Files["r0/doc.md"] {
		t.Fatal("r0 entry was reused despite the edit")
	}
	if second.Files["r1/doc.md"] != first.Files["r1/doc.md"] {
		t.Fatal("r1 entry was re-parsed although its root did not change")
	}
}
