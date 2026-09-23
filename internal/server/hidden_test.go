package server

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

// hiddenWorkspace builds one directory root containing plain, hidden,
// protected and aliasing fixtures, then serves it with the SkipHidden value
// derived from the hidden flag — exactly the shape the CLI passes for
// default runs (--hidden off) and --hidden runs. The workspace decides the
// sidebar, document routes, assets and search, so one builder covers the
// whole publishing surface.
func hiddenWorkspace(t *testing.T, hidden bool) (workspace, string) {
	t.Helper()
	root := t.TempDir()
	for name, contents := range map[string]string{
		"README.md":        "# Root\n\nvisible text\n",
		".draft.md":        "# Draft\n\ndraft text\n",
		".notes/guide.md":  "# Notes guide\n\nnotes guide text\n",
		".notes/image.png": "PNG",
		".git/config":      "[core]",
		".env.production":  "TOKEN=1",
		".env/hosts.md":    "# Env doc\n",
		".ssh/id_ed25519":  "key",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), contents)
	}
	// A visibly named alias to a hidden document, and one to a protected
	// file: the publishing policy must hold for canonical targets too.
	if err := os.Symlink(filepath.Join(root, ".notes", "guide.md"), filepath.Join(root, "alias.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".env.production"), filepath.Join(root, "config.txt")); err != nil {
		t.Fatal(err)
	}

	input, err := files.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	built, err := newWorkspace([]files.Input{input}, files.DiscoverOptions{Depth: 4, SkipHidden: !hidden}, false)
	if err != nil {
		t.Fatal(err)
	}
	return built, root
}

func hiddenHandler(built workspace) http.Handler {
	return newDocumentHandler(built, nil, directoryTestUI())
}

func hiddenFileList(t *testing.T, handler http.Handler) []string {
	t.Helper()
	response := performRequest(handler, http.MethodGet, "/api/files")
	if response.Code != http.StatusOK {
		t.Fatalf("/api/files status = %d, want 200", response.Code)
	}
	var payload struct {
		Roots []struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		} `json:"roots"`
	}
	decodeJSON(t, response, &payload)
	paths := make([]string, 0, len(payload.Roots[0].Files))
	for _, file := range payload.Roots[0].Files {
		paths = append(paths, file.Path)
	}
	return paths
}

func hiddenSearchPaths(t *testing.T, handler http.Handler, query string) []string {
	t.Helper()
	response := performRequest(handler, http.MethodGet, "/api/search?q="+query)
	if response.Code != http.StatusOK {
		t.Fatalf("/api/search status = %d, want 200", response.Code)
	}
	var payload struct {
		Results []struct {
			Path string `json:"path"`
		} `json:"results"`
	}
	decodeJSON(t, response, &payload)
	paths := make([]string, 0, len(payload.Results))
	for _, result := range payload.Results {
		paths = append(paths, result.Path)
	}
	return paths
}

// TestHiddenPublishingMatrix walks the release matrix over every publishing
// entrance: the default run hides dot-prefixed paths everywhere, --hidden
// admits the ordinary ones in one synchronized motion, and the protected
// paths stay refused whatever the flag says.
func TestHiddenPublishingMatrix(t *testing.T) {
	t.Run("default hides dot-prefixed paths everywhere", func(t *testing.T) {
		built, _ := hiddenWorkspace(t, false)
		handler := hiddenHandler(built)

		paths := hiddenFileList(t, handler)
		for _, unwanted := range []string{".draft.md", ".notes/guide.md", "alias.md"} {
			if slices.Contains(paths, unwanted) {
				t.Errorf("default listing contains hidden path %q", unwanted)
			}
		}
		if len(paths) == 0 || paths[0] != "README.md" {
			t.Errorf("default listing = %v, want README.md only", paths)
		}

		for _, target := range []string{
			"/api/document?path=.draft.md",
			"/api/document?path=.notes/guide.md",
			"/api/document?path=alias.md",
			"/raw/.draft.md",
			"/assets/.notes/image.png",
			"/assets/.git/config",
			"/assets/.env.production",
			"/assets/config.txt",
		} {
			if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusNotFound {
				t.Errorf("GET %s status = %d, want 404 by default", target, response.Code)
			}
		}
		if paths := hiddenSearchPaths(t, handler, "draft+text"); len(paths) != 0 {
			t.Errorf("default search surfaced %v, want none", paths)
		}
	})

	t.Run("hidden admits ordinary hidden paths and keeps protected ones refused", func(t *testing.T) {
		built, _ := hiddenWorkspace(t, true)
		handler := hiddenHandler(built)

		paths := hiddenFileList(t, handler)
		for _, wanted := range []string{"README.md", ".draft.md", ".notes/guide.md", "alias.md"} {
			if !slices.Contains(paths, wanted) {
				t.Errorf("--hidden listing lost %q: %v", wanted, paths)
			}
		}

		for _, target := range []string{
			"/api/document?path=.draft.md",
			"/api/document?path=.notes/guide.md",
			"/api/document?path=alias.md",
			"/raw/.draft.md",
			"/assets/.notes/image.png",
		} {
			if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusOK {
				t.Errorf("GET %s status = %d, want 200 with --hidden", target, response.Code)
			}
		}
		for _, target := range []string{
			"/assets/.git/config",
			"/assets/.env.production",
			"/assets/config.txt",
			"/api/document?path=.env/hosts.md",
			"/api/document?path=.ssh/id_ed25519",
		} {
			if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusNotFound {
				t.Errorf("GET %s status = %d, want 404 (protected paths stay refused with --hidden)", target, response.Code)
			}
		}
		if paths := hiddenSearchPaths(t, handler, "draft+text"); len(paths) != 1 || paths[0] != ".draft.md" {
			t.Errorf("--hidden search = %v, want [.draft.md]", paths)
		}
		// The alias shares its target's body, so both spellings surface —
		// with --hidden the alias satisfies every rule and stays reachable.
		if paths := hiddenSearchPaths(t, handler, "notes+guide+text"); !slices.Equal(paths, []string{".notes/guide.md", "alias.md"}) {
			t.Errorf("--hidden search = %v, want [.notes/guide.md alias.md]", paths)
		}
	})
}

// TestHiddenSingleFileInputStaysExplicit pins the explicit-input boundary:
// naming a hidden Markdown file on the command line serves and lists it
// whatever the hidden option, while the workspace stays single-document.
func TestHiddenSingleFileInputStaysExplicit(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, ".draft.md"), "# Draft\n\ndraft text\n")
		input, err := files.Resolve(filepath.Join(root, ".draft.md"))
		if err != nil {
			t.Fatal(err)
		}
		built, err := newWorkspace([]files.Input{input}, files.DiscoverOptions{Depth: 4, SkipHidden: !hidden}, false)
		if err != nil {
			t.Fatal(err)
		}
		handler := hiddenHandler(built)

		if response := performRequest(handler, http.MethodGet, "/api/document?path=.draft.md"); response.Code != http.StatusOK {
			t.Errorf("hidden=%v: explicit hidden input status = %d, want 200", hidden, response.Code)
		}
		paths := hiddenFileList(t, handler)
		if len(paths) != 1 || paths[0] != ".draft.md" {
			t.Errorf("hidden=%v: single-file listing = %v, want [.draft.md]", hidden, paths)
		}
	}
}

// TestHiddenDocumentBodyRenders pins that an admitted hidden document serves
// real rendered content, not just a 200 shell.
func TestHiddenDocumentBodyRenders(t *testing.T) {
	built, _ := hiddenWorkspace(t, true)
	handler := hiddenHandler(built)

	response := performRequest(handler, http.MethodGet, "/api/document?path=.notes/guide.md")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var payload struct {
		Path  string `json:"path"`
		Title string `json:"title"`
	}
	decodeJSON(t, response, &payload)
	if payload.Path != ".notes/guide.md" {
		t.Errorf("path = %q, want .notes/guide.md", payload.Path)
	}
	if !strings.Contains(payload.Title, "Notes guide") {
		t.Errorf("title = %q, want the hidden document's own title", payload.Title)
	}
}
