package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

// ignoreWorkspace builds one directory root with Markdown, an asset, a
// nested rule file and a symlink alias into the ignored tree, served with
// gitignore enabled — the shape every route below is exercised against.
func ignoreWorkspace(t *testing.T) (workspace, string) {
	t.Helper()
	root := t.TempDir()
	for name, contents := range map[string]string{
		"README.md":          "# Root\n\nvisible text\n",
		"guide.md":           "# Guide\n\nfindable guide text\n",
		"draft/note.md":      "# Draft note\n\nhidden draft text\n",
		"notes/public.md":    "# Public notes\n\npublic notes text\n",
		"notes/private.md":   "# Private notes\n\nprivate notes text\n",
		"images/logo.png":    "PNG",
		"images/scratch.png": "PNG",
		"secret/hidden.md":   "# Hidden\n",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), contents)
	}
	rules := "draft/\nimages/scratch.png\nsecret/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", ".gitignore"), []byte("private.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret", "hidden.md"), filepath.Join(root, "alias.md")); err != nil {
		t.Fatal(err)
	}

	input, err := files.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	built, err := newWorkspace([]files.Input{input}, files.DiscoverOptions{Depth: 4, SkipHidden: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	return built, root
}

func ignoreHandler(workspace workspace) http.Handler {
	return newDocumentHandler(workspace, nil, directoryTestUI())
}

func TestIgnoreRoutesRefuseIgnoredPaths(t *testing.T) {
	built, _ := ignoreWorkspace(t)
	handler := ignoreHandler(built)

	t.Run("api/files omits ignored documents", func(t *testing.T) {
		response := performRequest(handler, http.MethodGet, "/api/files")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}
		body := response.Body.String()
		// /api/files lists documents only; assets are asserted through
		// the /assets route below.
		for _, unwanted := range []string{"draft/note.md", "notes/private.md", "alias.md"} {
			if strings.Contains(body, unwanted) {
				t.Errorf("file listing contains ignored path %q", unwanted)
			}
		}
		for _, wanted := range []string{"README.md", "guide.md", "notes/public.md"} {
			if !strings.Contains(body, wanted) {
				t.Errorf("file listing lost visible path %q", wanted)
			}
		}
	})

	t.Run("api/document answers 404 for ignored documents", func(t *testing.T) {
		for _, path := range []string{"/api/document?path=draft%2Fnote.md", "/api/document?path=notes%2Fprivate.md", "/api/document?path=alias.md"} {
			assertJSONError(t, performRequest(handler, http.MethodGet, path), http.StatusNotFound)
		}
		if response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md"); response.Code != http.StatusOK {
			t.Errorf("visible document status = %d, want 200", response.Code)
		}
	})

	t.Run("raw answers 404 for ignored documents", func(t *testing.T) {
		if response := performRequest(handler, http.MethodGet, "/raw/draft/note.md"); response.Code != http.StatusNotFound {
			t.Errorf("raw draft status = %d, want 404", response.Code)
		}
		if response := performRequest(handler, http.MethodGet, "/raw/notes/private.md"); response.Code != http.StatusNotFound {
			t.Errorf("raw private status = %d, want 404", response.Code)
		}
		if response := performRequest(handler, http.MethodGet, "/raw/guide.md"); response.Code != http.StatusOK {
			t.Errorf("raw guide status = %d, want 200", response.Code)
		}
	})

	t.Run("assets answer 404 for ignored files", func(t *testing.T) {
		if response := performRequest(handler, http.MethodGet, "/assets/images/scratch.png"); response.Code != http.StatusNotFound {
			t.Errorf("ignored asset status = %d, want 404", response.Code)
		}
		if response := performRequest(handler, http.MethodGet, "/assets/images/logo.png"); response.Code != http.StatusOK {
			t.Errorf("visible asset status = %d, want 200", response.Code)
		}
	})

	t.Run("api/search skips ignored documents", func(t *testing.T) {
		response := performRequest(handler, http.MethodGet, "/api/search?q=hidden+draft+text")
		if response.Code != http.StatusOK {
			t.Fatalf("search status = %d, want 200", response.Code)
		}
		if strings.Contains(response.Body.String(), "draft/note.md") {
			t.Error("search returned the ignored draft/note.md")
		}
		if response := performRequest(handler, http.MethodGet, "/api/search?q=findable+guide"); !strings.Contains(response.Body.String(), "guide.md") {
			t.Error("search lost the visible guide.md")
		}
	})
}

// TestIgnoreRulesFollowRuleEdits proves the per-request snapshot contract:
// rewriting a rule file changes the very next answer, with no restart and
// no stale cache.
func TestIgnoreRulesFollowRuleEdits(t *testing.T) {
	built, root := ignoreWorkspace(t)
	handler := ignoreHandler(built)

	if response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md"); response.Code != http.StatusOK {
		t.Fatalf("pre-edit status = %d, want 200", response.Code)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("guide.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md"); response.Code != http.StatusNotFound {
		t.Fatalf("post-edit status = %d, want 404", response.Code)
	}
	// Removing the rule restores the document on the next request as well.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if response := performRequest(handler, http.MethodGet, "/api/document?path=guide.md"); response.Code != http.StatusOK {
		t.Fatalf("restored status = %d, want 200", response.Code)
	}
}

// TestIgnoreDisabledKeepsEverythingServed pins the --no-gitignore path: the
// same tree with ignore off serves every file the structural rules admit.
func TestIgnoreDisabledKeepsEverythingServed(t *testing.T) {
	_, root := ignoreWorkspace(t)
	input, err := files.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	built, err := newWorkspace([]files.Input{input}, files.DiscoverOptions{Depth: 4, SkipHidden: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	handler := ignoreHandler(built)

	if response := performRequest(handler, http.MethodGet, "/api/document?path=draft%2Fnote.md"); response.Code != http.StatusOK {
		t.Errorf("ignore-off draft status = %d, want 200", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/api/document?path=notes%2Fprivate.md"); response.Code != http.StatusOK {
		t.Errorf("ignore-off private status = %d, want 200", response.Code)
	}
	if response := performRequest(handler, http.MethodGet, "/assets/images/scratch.png"); response.Code != http.StatusOK {
		t.Errorf("ignore-off asset status = %d, want 200", response.Code)
	}
}

// TestSingleFileScopeIgnoresNothing pins the explicit-publishing boundary:
// a single Markdown input is served whatever its own rules say.
func TestSingleFileScopeIgnoresNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("chosen.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "chosen.md"), "# Chosen\n")
	writeTestFile(t, filepath.Join(root, "other.md"), "# Other\n")
	writeTestFile(t, filepath.Join(root, "picture.png"), "PNG")

	// The file input is named explicitly, so it is served; the rest of the
	// parent directory stays ordinary assets with no ignore rules applied.
	input, err := files.Resolve(filepath.Join(root, "chosen.md"))
	if err != nil {
		t.Fatal(err)
	}
	built, err := newWorkspace([]files.Input{input}, files.DiscoverOptions{Depth: 4, SkipHidden: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	handler := ignoreHandler(built)

	if response := performRequest(handler, http.MethodGet, "/api/document?path=chosen.md"); response.Code != http.StatusOK {
		t.Errorf("explicit single-file input status = %d, want 200", response.Code)
	}
}

// TestIgnoreRulesApplyPerRoot proves multi-root isolation: two directory
// roots each answer from their own rule files.
func TestIgnoreRulesApplyPerRoot(t *testing.T) {
	alpha, beta := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(alpha, "doc.md"), "# Alpha\n")
	writeTestFile(t, filepath.Join(beta, "doc.md"), "# Beta\n")
	if err := os.WriteFile(filepath.Join(alpha, ".gitignore"), []byte("doc.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := files.Resolve(alpha)
	if err != nil {
		t.Fatal(err)
	}
	second, err := files.Resolve(beta)
	if err != nil {
		t.Fatal(err)
	}
	built, err := newWorkspace([]files.Input{first, second}, files.DiscoverOptions{Depth: 4, SkipHidden: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	handler := ignoreHandler(built)

	assertJSONError(t, performRequest(handler, http.MethodGet, "/api/document?path=r0%2Fdoc.md"), http.StatusNotFound)
	if response := performRequest(handler, http.MethodGet, "/api/document?path=r1%2Fdoc.md"); response.Code != http.StatusOK {
		t.Errorf("beta root document status = %d, want 200", response.Code)
	}
}
