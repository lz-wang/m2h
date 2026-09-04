package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

func searchHandler(t *testing.T, root string, discovery files.DiscoverOptions) http.Handler {
	t.Helper()
	return newDocumentHandler(
		singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: discovery}),
		nil,
		directoryTestUI(),
	)
}

func decodeSearch(t *testing.T, response *httptest.ResponseRecorder) searchResponse {
	t.Helper()
	var payload searchResponse
	decodeJSON(t, response, &payload)
	return payload
}

func searchResultsFor(t *testing.T, handler http.Handler, query string) searchResponse {
	t.Helper()
	response := performRequest(handler, http.MethodGet, "/api/search?q="+query)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/search?q=%s status = %d, body = %s", query, response.Code, response.Body.String())
	}
	return decodeSearch(t, response)
}

func TestSearchQueryContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n\n正文 token\n")
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2})

	tests := []struct {
		name   string
		target string
	}{
		{name: "missing q", target: "/api/search"},
		{name: "empty q", target: "/api/search?q="},
		{name: "blank q", target: "/api/search?q=%20%20"},
		{name: "duplicate q", target: "/api/search?q=a&q=b"},
		{name: "extra parameter", target: "/api/search?q=a&limit=20"},
		{
			name:   "query over 128 runes",
			target: "/api/search?q=" + strings.Repeat("长", 129),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			response := performRequest(handler, http.MethodGet, test.target)
			assertJSONError(t, response, http.StatusBadRequest)
		})
	}

	t.Run("method not allowed", func(t *testing.T) {
		t.Parallel()
		response := performRequest(handler, http.MethodPost, "/api/search?q=token")
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST status = %d, want 405", response.Code)
		}
	})

	t.Run("single-rune CJK query is legal", func(t *testing.T) {
		t.Parallel()
		writeTestFile(t, filepath.Join(root, "图.md"), "# 图\n\n图库说明")
		payload := searchResultsFor(t, handler, "%E5%9B%BE")
		if len(payload.Results) != 1 || payload.Results[0].Path != "图.md" {
			t.Fatalf("results = %+v, want exactly 图.md", payload.Results)
		}
	})

	t.Run("query of exactly 128 runes is legal", func(t *testing.T) {
		t.Parallel()
		// A long query still matches: the AND token is one long substring.
		writeTestFile(t, filepath.Join(root, "long.md"), "prefix "+strings.Repeat("长", 128)+" suffix")
		payload := searchResultsFor(t, handler, strings.Repeat("%E9%95%BF", 128))
		if len(payload.Results) != 1 || payload.Results[0].Path != "long.md" {
			t.Fatalf("results = %+v, want exactly long.md", payload.Results)
		}
	})
}

func TestSearchBasicResponseShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "guide.md"), "---\ntitle: 指南标题\ndescription: 文档摘要\n---\n\n## 解析器\n\n正文提到 goldmark 引擎。\n")
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2})

	response := performRequest(handler, http.MethodGet, "/api/search?q=goldmark")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	payload := decodeSearch(t, response)
	if payload.Query != "goldmark" {
		t.Errorf("query = %q, want goldmark", payload.Query)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results = %+v, want exactly one", payload.Results)
	}
	result := payload.Results[0]
	if result.Path != "guide.md" {
		t.Errorf("path = %q, want guide.md", result.Path)
	}
	if result.Title != "指南标题" {
		t.Errorf("title = %q, want 指南标题", result.Title)
	}
	if !strings.Contains(result.Snippet, "goldmark") {
		t.Errorf("snippet = %q, want it around the match", result.Snippet)
	}
	if result.Heading == nil || result.Heading.ID != "解析器" || result.Heading.Text != "解析器" {
		t.Errorf("heading = %+v, want 解析器", result.Heading)
	}

	t.Run("metadata-only description match carries the description snippet", func(t *testing.T) {
		t.Parallel()
		payload := searchResultsFor(t, handler, "%E6%91%98%E8%A6%81") // 摘要
		if len(payload.Results) != 1 {
			t.Fatalf("results = %+v", payload.Results)
		}
		if payload.Results[0].Snippet != "文档摘要" {
			t.Errorf("snippet = %q, want 文档摘要", payload.Results[0].Snippet)
		}
		if payload.Results[0].Heading != nil {
			t.Errorf("heading = %+v, want none", payload.Results[0].Heading)
		}
	})
}

// The search walks the disk at request time: edits and deletes made after
// the server started are visible to the very next query.
func TestSearchReflectsFilesystemChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	file := filepath.Join(root, "topic.md")
	writeTestFile(t, file, "# Topic\n\nold-token\n")
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2})

	if results := searchResultsFor(t, handler, "old-token"); len(results.Results) != 1 {
		t.Fatalf("initial results = %+v, want exactly one", results.Results)
	}

	writeTestFile(t, file, "# Topic\n\nnew-token\n")
	if results := searchResultsFor(t, handler, "new-token"); len(results.Results) != 1 {
		t.Fatalf("after edit, results = %+v, want exactly one", results.Results)
	}
	if results := searchResultsFor(t, handler, "old-token"); len(results.Results) != 0 {
		t.Fatalf("stale token results = %+v, want none", results.Results)
	}

	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	response := performRequest(handler, http.MethodGet, "/api/search?q=new-token")
	if response.Code != http.StatusOK {
		t.Fatalf("search after delete status = %d, want 200", response.Code)
	}
	if results := decodeSearch(t, response); len(results.Results) != 0 {
		t.Fatalf("results after delete = %+v, want none", results.Results)
	}
}

// Invalid frontmatter degrades to a whole-source projection: the document
// stays searchable while opening it still answers 422.
func TestSearchKeepsInvalidFrontmatterDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "broken.md"), "---\ntitle: [unclosed\n---\n\n# Broken\n\nunique-broken-token\n")
	writeTestFile(t, filepath.Join(root, "ok.md"), "# OK")
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2})

	payload := searchResultsFor(t, handler, "unique-broken-token")
	if len(payload.Results) != 1 || payload.Results[0].Path != "broken.md" {
		t.Fatalf("results = %+v, want exactly broken.md", payload.Results)
	}

	document := performRequest(handler, http.MethodGet, "/api/document?path=broken.md")
	if document.Code != http.StatusUnprocessableEntity {
		t.Fatalf("document status = %d, want 422", document.Code)
	}
}

func TestSearchHonorsPublishingScope(t *testing.T) {
	t.Parallel()

	t.Run("hidden and excluded files are not searchable", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "public.md"), "# Public\n\nscope-token\n")
		writeTestFile(t, filepath.Join(root, ".hidden.md"), "# Hidden\n\nscope-token\n")
		writeTestFile(t, filepath.Join(root, "skipped", "skipped.md"), "# Skipped\n\nscope-token\n")
		// Only public.md matches the glob; skipped/ is excluded explicitly.
		options := files.DiscoverOptions{
			Depth:    4,
			Pattern:  "public.md",
			Excludes: []string{"skipped/**"},
		}
		handler := searchHandler(t, root, options)

		payload := searchResultsFor(t, handler, "scope-token")
		if len(payload.Results) != 1 || payload.Results[0].Path != "public.md" {
			t.Fatalf("results = %+v, want exactly public.md", payload.Results)
		}
	})

	// The hidden rule must hold without a glob narrowing the scope first:
	// SkipHidden alone keeps dot-prefixed documents out of search.
	t.Run("hidden files are not searchable without a glob", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "public.md"), "# Public\n\nhidden-token\n")
		writeTestFile(t, filepath.Join(root, ".hidden.md"), "# Hidden\n\nhidden-token\n")
		handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2, SkipHidden: true})

		payload := searchResultsFor(t, handler, "hidden-token")
		if len(payload.Results) != 1 || payload.Results[0].Path != "public.md" {
			t.Fatalf("results = %+v, want exactly public.md", payload.Results)
		}
	})

	t.Run("single-file scope searches only its document", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "chosen.md"), "# Chosen\n\nsingle-file-token\n")
		writeTestFile(t, filepath.Join(root, "other.md"), "# Other\n\nsingle-file-token\n")
		input, err := files.Resolve(filepath.Join(root, "chosen.md"))
		if err != nil {
			t.Fatal(err)
		}
		handler := newDocumentHandler(singleRootWorkspace(newRootScope(input, files.DiscoverOptions{Depth: 2})), nil, directoryTestUI())

		payload := searchResultsFor(t, handler, "single-file-token")
		if len(payload.Results) != 1 || payload.Results[0].Path != "chosen.md" {
			t.Fatalf("results = %+v, want exactly chosen.md", payload.Results)
		}
	})

	t.Run("depth limit applies", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "shallow.md"), "# Shallow\n\ndepth-token\n")
		writeTestFile(t, filepath.Join(root, "deep", "nested", "buried.md"), "# Buried\n\ndepth-token\n")
		handler := searchHandler(t, root, files.DiscoverOptions{Depth: 1})

		payload := searchResultsFor(t, handler, "depth-token")
		if len(payload.Results) != 1 || payload.Results[0].Path != "shallow.md" {
			t.Fatalf("results = %+v, want exactly shallow.md", payload.Results)
		}
	})
}

// Search answers with exactly the publishing boundary /api/document and /raw
// enforce: an alias whose canonical target is hidden, and one escaping the
// root, never become a leak channel, while a legal visible alias keeps
// working under its requested identity.
func TestSearchSymlinkAliasesHonorPublishingBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	outside := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, ".secret.md"), "# Secret\n\nhidden-target-token\n")
	writeTestFile(t, filepath.Join(outside, "secret.md"), "# Secret\n\nescape-target-token\n")
	writeTestFile(t, filepath.Join(root, "docs", "real.md"), "# Real\n\nalias-token\n")
	if err := os.Symlink(filepath.Join(root, ".secret.md"), filepath.Join(root, "hidden-alias.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "escape-alias.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "docs", "real.md"), filepath.Join(root, "visible-alias.md")); err != nil {
		t.Fatal(err)
	}

	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 4, SkipHidden: true})

	for _, leak := range []struct{ query, reason string }{
		{"hidden-target-token", "hidden-target alias"},
		{"escape-target-token", "outside-root alias"},
	} {
		if payload := searchResultsFor(t, handler, leak.query); len(payload.Results) != 0 {
			t.Fatalf("%s leaked: %+v", leak.reason, payload.Results)
		}
	}

	// The legal alias stays served, addressed by its requested identity —
	// both the alias and its target are publishable documents in their own
	// right, so each answers on its own path, exactly as /api/document does.
	payload := searchResultsFor(t, handler, "alias-token")
	paths := make([]string, 0, len(payload.Results))
	for _, result := range payload.Results {
		paths = append(paths, result.Path)
	}
	sort.Strings(paths)
	if len(paths) != 2 || paths[0] != "docs/real.md" || paths[1] != "visible-alias.md" {
		t.Fatalf("results = %v, want exactly docs/real.md and visible-alias.md", paths)
	}
}

// A symlink swapped between discovery and the read must not leak its new
// target: search re-enters resolveVisibleDocument before reading, so the
// swapped alias is refused exactly like /api/document refuses it.
func TestSearchRechecksBoundaryAfterDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	outside := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(outside, "secret.md"), "# Secret\n\nswap-token\n")
	writeTestFile(t, filepath.Join(root, "public.md"), "# Public\n\npublic-token\n")
	alias := filepath.Join(root, "alias.md")
	if err := os.Symlink(filepath.Join(root, "public.md"), alias); err != nil {
		t.Fatal(err)
	}

	scope := rootScope{root: root, discovery: files.DiscoverOptions{Depth: 2}}
	handler := &documentHandler{workspace: singleRootWorkspace(scope), ui: directoryTestUI()}
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		discovered, err := scope.discover(ctx)
		// The filesystem mutates the instant discovery returns: the alias
		// now points outside the root, like a racing symlink swap.
		if err == nil {
			if removeErr := os.Remove(alias); removeErr != nil {
				t.Errorf("remove alias: %v", removeErr)
			}
			if linkErr := os.Symlink(filepath.Join(outside, "secret.md"), alias); linkErr != nil {
				t.Errorf("re-link alias: %v", linkErr)
			}
		}
		return discovered, err
	}

	response := performRequest(handler.routes(nil), http.MethodGet, "/api/search?q=swap-token")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if payload := decodeSearch(t, response); len(payload.Results) != 0 {
		t.Fatalf("search leaked the swapped alias target: %+v", payload.Results)
	}
}

func TestSearchMultiRootIdentity(t *testing.T) {
	t.Parallel()

	rootA := t.TempDir()
	rootB := t.TempDir()
	writeTestFile(t, filepath.Join(rootA, "guide.md"), "# Guide A\n\ndual-root-token A\n")
	writeTestFile(t, filepath.Join(rootB, "guide.md"), "# Guide B\n\ndual-root-token B\n")
	handler := newDocumentHandler(
		workspace{roots: []workspaceRoot{
			{id: "r0", label: "a", input: files.Input{Kind: files.KindDirectory, Path: rootA}, scope: rootScope{root: canonicalDirectory(t, rootA), discovery: files.DiscoverOptions{Depth: 2}}},
			{id: "r1", label: "b", input: files.Input{Kind: files.KindDirectory, Path: rootB}, scope: rootScope{root: canonicalDirectory(t, rootB), discovery: files.DiscoverOptions{Depth: 2}}},
		}},
		nil,
		directoryTestUI(),
	)

	payload := searchResultsFor(t, handler, "dual-root-token")
	if len(payload.Results) != 2 {
		t.Fatalf("results = %+v, want one per root", payload.Results)
	}
	gotPaths := []string{payload.Results[0].Path, payload.Results[1].Path}
	if gotPaths[0] != "r0/guide.md" || gotPaths[1] != "r1/guide.md" {
		t.Errorf("paths = %v, want r0/guide.md and r1/guide.md", gotPaths)
	}
	for _, result := range payload.Results {
		if strings.Contains(result.Path, rootA) || strings.Contains(result.Path, rootB) {
			t.Errorf("result path leaks the filesystem root: %q", result.Path)
		}
	}
}

func TestSearchRanksTitleAboveBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "body.md"), "# 无关标题\n\nrank-token\n")
	writeTestFile(t, filepath.Join(root, "titled.md"), "---\ntitle: rank-token\n---\n\n# 无关\n")
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2})

	payload := searchResultsFor(t, handler, "rank-token")
	if len(payload.Results) != 2 {
		t.Fatalf("results = %+v, want two", payload.Results)
	}
	if payload.Results[0].Path != "titled.md" {
		t.Errorf("first result = %q, want titled.md", payload.Results[0].Path)
	}
}

func TestSearchTruncatesToMaxResults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for index := range maxSearchResults + 20 {
		name := filepath.Join(root, fmt.Sprintf("note-%02d.md", index))
		writeTestFile(t, name, fmt.Sprintf("# Note %02d\n\ntruncate-token\n", index))
	}
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 1})

	payload := searchResultsFor(t, handler, "truncate-token")
	if len(payload.Results) != maxSearchResults {
		t.Fatalf("results = %d, want %d", len(payload.Results), maxSearchResults)
	}
}

// An aborted request must not keep scanning the workspace: the handler
// returns without writing anything when the client disappears.
func TestSearchStopsWhenRequestIsCancelled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\ncancel-token")
	handler := searchHandler(t, root, files.DiscoverOptions{Depth: 2})

	request := httptest.NewRequest(http.MethodGet, "/api/search?q=cancel-token", nil)
	requestContext, cancel := context.WithCancel(request.Context())
	request = request.WithContext(requestContext)
	cancel()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("cancelled request wrote %q (status %d), want an empty body", response.Body.String(), response.Code)
	}
}
