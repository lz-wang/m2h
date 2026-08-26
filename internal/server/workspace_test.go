package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

func resolveTestInput(t *testing.T, path string) files.Input {
	t.Helper()
	input, err := files.Resolve(path)
	if err != nil {
		t.Fatalf("resolve %q: %v", path, err)
	}
	return input
}

func TestNewWorkspaceAssignsIDsAndLabelsInInputOrder(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	alpha := filepath.Join(base, "alpha")
	beta := filepath.Join(base, "beta")
	for _, directory := range []string{alpha, beta} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(directory, "README.md"), "# Readme")
	}
	workspace, err := newWorkspace(
		[]files.Input{
			resolveTestInput(t, alpha),
			resolveTestInput(t, beta),
		},
		files.DiscoverOptions{Depth: 4},
	)
	if err != nil {
		t.Fatalf("newWorkspace() error = %v", err)
	}

	if workspace.rootCount() != 2 {
		t.Fatalf("rootCount() = %d, want 2", workspace.rootCount())
	}
	first, second := workspace.roots[0], workspace.roots[1]
	if first.id != "r0" || second.id != "r1" {
		t.Fatalf("root ids = %q, %q; want r0, r1", first.id, second.id)
	}
	if first.label != "alpha" || second.label != "beta" {
		t.Fatalf("root labels = %q, %q; want alpha, beta", first.label, second.label)
	}
	if got, want := workspace.primary().id, "r0"; got != want {
		t.Fatalf("primary().id = %q, want %q", got, want)
	}
	if !workspace.anyDirectory() {
		t.Fatal("anyDirectory() = false, want true with two directory roots")
	}
	if first.scope.kind() != workspaceDirectory || second.scope.kind() != workspaceDirectory {
		t.Fatalf("directory roots reported kinds %q, %q", first.scope.kind(), second.scope.kind())
	}
}

func TestNewWorkspaceKeepsSingleFileAndDirectoryRootsIndependent(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	document := filepath.Join(base, "notes.md")
	writeTestFile(t, document, "# Notes")
	docs := filepath.Join(base, "docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(docs, "README.md"), "# Docs")

	workspace, err := newWorkspace(
		[]files.Input{
			resolveTestInput(t, document),
			resolveTestInput(t, docs),
		},
		files.DiscoverOptions{Depth: 4},
	)
	if err != nil {
		t.Fatalf("newWorkspace() error = %v", err)
	}

	first, second := workspace.roots[0], workspace.roots[1]
	if first.scope.kind() != workspaceSingle {
		t.Fatalf("file root kind = %q, want %q", first.scope.kind(), workspaceSingle)
	}
	if first.label != "notes.md" {
		t.Fatalf("file root label = %q, want notes.md", first.label)
	}
	if !first.scope.allowsDocument("notes.md") || first.scope.allowsDocument("other.md") {
		t.Fatal("single-file root must only admit its own document")
	}
	if second.scope.kind() != workspaceDirectory {
		t.Fatalf("directory root kind = %q, want %q", second.scope.kind(), workspaceDirectory)
	}
	if !second.scope.allowsDocument("README.md") {
		t.Fatal("directory root must admit its Markdown files")
	}
	if !workspace.anyDirectory() {
		t.Fatal("anyDirectory() = false, want true with one directory root")
	}
}

func TestNewWorkspaceLabelsDuplicateBasenamesUniquely(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	one := filepath.Join(base, "one", "docs")
	two := filepath.Join(base, "two", "docs")
	three := filepath.Join(base, "three", "docs")
	for _, directory := range []string{one, two, three} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	workspace, err := newWorkspace(
		[]files.Input{
			resolveTestInput(t, one),
			resolveTestInput(t, two),
			resolveTestInput(t, three),
		},
		files.DiscoverOptions{Depth: 4},
	)
	if err != nil {
		t.Fatalf("newWorkspace() error = %v", err)
	}

	want := []string{"docs", "docs (2)", "docs (3)"}
	for index, label := range want {
		if got := workspace.roots[index].label; got != label {
			t.Fatalf("root %d label = %q, want %q", index, got, label)
		}
	}
	// Internal ids stay positional regardless of display-label collisions.
	for index, id := range []string{"r0", "r1", "r2"} {
		if got := workspace.roots[index].id; got != id {
			t.Fatalf("root %d id = %q, want %q", index, got, id)
		}
	}
}

func TestNewWorkspaceRejectsDuplicateCanonicalRoots(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	docs := filepath.Join(base, "docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(docs, "README.md"), "# Readme")
	writeTestFile(t, filepath.Join(docs, "guide.md"), "# Guide")

	t.Run("same directory twice", func(t *testing.T) {
		_, err := newWorkspace(
			[]files.Input{
				resolveTestInput(t, docs),
				resolveTestInput(t, docs+string(os.PathSeparator)),
			},
			files.DiscoverOptions{Depth: 4},
		)
		if err == nil || !strings.Contains(err.Error(), "duplicate workspace root") {
			t.Fatalf("duplicate directory error = %v", err)
		}
	})

	t.Run("same file twice", func(t *testing.T) {
		_, err := newWorkspace(
			[]files.Input{
				resolveTestInput(t, filepath.Join(docs, "README.md")),
				resolveTestInput(t, filepath.Join(docs, "README.md")),
			},
			files.DiscoverOptions{Depth: 4},
		)
		if err == nil || !strings.Contains(err.Error(), "duplicate workspace root") {
			t.Fatalf("duplicate file error = %v", err)
		}
	})

	t.Run("symlink alias to the same tree", func(t *testing.T) {
		alias := filepath.Join(base, "alias")
		if err := os.Symlink(docs, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := newWorkspace(
			[]files.Input{
				resolveTestInput(t, docs),
				resolveTestInput(t, alias),
			},
			files.DiscoverOptions{Depth: 4},
		)
		if err == nil || !strings.Contains(err.Error(), "duplicate workspace root") {
			t.Fatalf("symlink alias error = %v", err)
		}
	})
}
