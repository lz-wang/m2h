package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

func TestDirectoryLinksInThreeRootWorkspace(t *testing.T) {
	t.Parallel()
	var inputs []files.Input
	for i := 0; i < 3; i++ {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "docs", "README.md"), "# Index\n\n| Topic |\n|---|\n| [Algorithm](./algorithm/?mode=dark#intro) |\n\n[Nested](nested/) [Empty](empty/) [No slash](algorithm) [Root](/docs/algorithm/) <a href=\"./algorithm/\">Raw</a> ![Not a document](algorithm/) [File](LICENSE)\n")
		writeTestFile(t, filepath.Join(root, "docs", "algorithm", "README.md"), "# Intro\n")
		writeTestFile(t, filepath.Join(root, "docs", "LICENSE"), "license")
		writeTestFile(t, filepath.Join(root, "docs", "nested", "child", "a.md"), "# Child")
		writeTestFile(t, filepath.Join(root, "docs", "empty", "LICENSE"), "license")
		inputs = append(inputs, resolveTestInput(t, root))
	}
	workspace, err := newWorkspace(inputs, files.DiscoverOptions{Depth: 4, SkipHidden: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	handler := newDocumentHandler(workspace, nil, directoryTestUI())
	response := performRequest(handler, http.MethodGet, "/api/document?path=r2/docs/README.md")
	if response.Code != http.StatusOK {
		t.Fatalf("%d %s", response.Code, response.Body.String())
	}
	var payload documentResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="/doc/r2/docs/algorithm/README.md?mode=dark#intro"`, `href="/doc/r2/docs/algorithm/README.md"`, `src="/assets/r2/docs/algorithm"`, `href="/assets/r2/docs/LICENSE"`, `href="/doc/r2/docs/nested/child/a.md"`, `href="/__m2h_invalid_local_reference__?target=empty%2F"`} {
		if !strings.Contains(payload.HTML, want) {
			t.Fatalf("missing %s in %s", want, payload.HTML)
		}
	}
	if strings.Contains(payload.HTML, `href="/assets/r2/docs/algorithm`) {
		t.Fatal(payload.HTML)
	}
	for _, route := range []string{"/doc/r2/docs/algorithm/README.md", "/api/document?path=" + url.QueryEscape("r2/docs/algorithm/README.md")} {
		if r := performRequest(handler, http.MethodGet, route); r.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", route, r.Code, r.Body.String())
		}
	}
}

func TestDirectoryLinkResolverScopeAndEncoding(t *testing.T) {
	t.Parallel()
	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "topic space", "a #.md"), "# A")
	writeTestFile(t, filepath.Join(root, "hidden", "deep", "README.md"), "# Deep")
	writeTestFile(t, filepath.Join(root, ".private", "README.md"), "# Private")
	scope := rootScope{root: root, discovery: files.DiscoverOptions{Depth: 1, SkipHidden: true}}
	handler := &documentHandler{workspace: singleRootWorkspace(scope)}
	resolve := handler.directoryDocumentResolver(t.Context(), handler.workspace.primary())
	if got, _ := resolve("topic%20space"); got != "topic%20space/a%20%23.md" {
		t.Fatal(got)
	}
	for _, p := range []string{"hidden/deep", ".private", "absent", "../escape", "bad%zz"} {
		if got, _ := resolve(p); got != "" {
			t.Fatalf("%q -> %q", p, got)
		}
	}
	single := rootScope{root: root, file: "topic space/a #.md"}
	handler = &documentHandler{workspace: singleRootWorkspace(single)}
	if got, _ := handler.directoryDocumentResolver(t.Context(), handler.workspace.primary())("topic%20space"); got != "" {
		t.Fatal(got)
	}
}

// TestDirectoryEntrySkipsIgnoredDocuments pins the entry-document policy:
// the walk must judge the request's ignore snapshot, so an ignored
// README.md never blocks a publishable index.md and the admission check
// admits the pick. The fixtures sit in a hidden directory, the combination
// --hidden brings into entry resolution.
func TestDirectoryEntrySkipsIgnoredDocuments(t *testing.T) {
	t.Parallel()
	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, ".gitignore"), ".notes/README.md\n")
	writeTestFile(t, filepath.Join(root, ".notes", "README.md"), "# Ignored\n")
	writeTestFile(t, filepath.Join(root, ".notes", "index.md"), "# Notes index\n")
	scope := rootScope{
		root:      root,
		discovery: files.DiscoverOptions{Depth: 4, SkipHidden: false},
		ignore:    true,
	}
	handler := &documentHandler{workspace: singleRootWorkspace(scope)}
	resolve := handler.directoryDocumentResolver(t.Context(), handler.workspace.primary())
	if got, _ := resolve(".notes"); got != ".notes/index.md" {
		t.Fatalf("directory entry = %q, want .notes/index.md", got)
	}

	// The same workspace with ignore disabled publishes the README entry.
	scope.ignore = false
	handler = &documentHandler{workspace: singleRootWorkspace(scope)}
	resolve = handler.directoryDocumentResolver(t.Context(), handler.workspace.primary())
	if got, _ := resolve(".notes"); got != ".notes/README.md" {
		t.Fatalf("entry without ignore rules = %q, want .notes/README.md", got)
	}
}

// TestDirectoryLinkResolverDegradesOnUnreadableRules pins the failure mode
// for rule files that cannot be read: the link stays unresolved instead of
// widening the walk, while the document routes keep answering 500.
func TestDirectoryLinkResolverDegradesOnUnreadableRules(t *testing.T) {
	t.Parallel()
	root := canonicalDirectory(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".gitignore"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "topic", "a.md"), "# A")
	scope := rootScope{
		root:      root,
		discovery: files.DiscoverOptions{Depth: 4, SkipHidden: true},
		ignore:    true,
	}
	handler := &documentHandler{workspace: singleRootWorkspace(scope)}
	resolve := handler.directoryDocumentResolver(t.Context(), handler.workspace.primary())
	if got, _ := resolve("topic"); got != "" {
		t.Fatalf("directory entry = %q, want empty on unreadable rules", got)
	}
}
