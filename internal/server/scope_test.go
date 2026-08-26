package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

func TestNewPreviewScopeSingleFileIgnoresDiscovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "docs", "guide.md")
	writeTestFile(t, source, "# Guide")
	input, err := files.Resolve(source)
	if err != nil {
		t.Fatal(err)
	}

	// Even with depth/glob options, a single file becomes a literal scope so
	// names like foo[1].md are never reinterpreted as a glob.
	scope := newRootScope(input, files.DiscoverOptions{Depth: 5, Pattern: "**/*.md"})
	if !scope.isSingleFile() {
		t.Fatal("single-file scope reports as a directory")
	}
	if scope.root != filepath.Dir(input.Path) {
		t.Fatalf("root = %q, want %q", scope.root, filepath.Dir(input.Path))
	}
	if scope.file != "guide.md" {
		t.Fatalf("file = %q, want guide.md", scope.file)
	}
	if scope.discovery.Depth != 0 || scope.discovery.Pattern != "" {
		t.Fatalf("single-file scope kept discovery options %+v", scope.discovery)
	}
}

func TestNewPreviewScopeDirectoryPreservesDiscovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	input, err := files.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	options := files.DiscoverOptions{Depth: 3, Pattern: "**/*.md"}

	scope := newRootScope(input, options)
	if scope.isSingleFile() {
		t.Fatal("directory scope reports as a single file")
	}
	if scope.root != input.Path {
		t.Fatalf("root = %q, want %q", scope.root, input.Path)
	}
	if scope.file != "" {
		t.Fatalf("file = %q, want empty", scope.file)
	}
	if scope.discovery.Depth != options.Depth || scope.discovery.Pattern != options.Pattern {
		t.Fatalf("discovery = %+v, want %+v", scope.discovery, options)
	}
}

func TestSingleFileScopeDiscoversOnlyOneDocument(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	writeTestFile(t, source, "# Guide")
	// A sibling Markdown must be invisible to a single-file scope.
	writeTestFile(t, filepath.Join(root, "other.md"), "# Other")

	scope := rootScope{root: root, file: "guide.md"}
	discovered, err := scope.discover(context.Background())
	if err != nil {
		t.Fatalf("discover error = %v", err)
	}
	if len(discovered.Markdown) != 1 || discovered.Markdown[0].RelativePath != "guide.md" {
		t.Fatalf("discovery = %+v", discovered.Markdown)
	}
	if discovered.Assets != nil && len(discovered.Assets) != 0 {
		t.Fatalf("single-file discovery exposed assets = %+v", discovered.Assets)
	}
}

func TestSingleFileScopeDiscoverFailures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "subdir", "inside.md"), "# Inside")

	missing := rootScope{root: root, file: "missing.md"}
	if discovered, err := missing.discover(context.Background()); err != nil || len(discovered.Markdown) != 0 {
		t.Fatalf("missing target discovery = %+v, %v; want empty success", discovered, err)
	}

	directory := rootScope{root: root, file: "subdir"}
	if _, err := directory.discover(context.Background()); err == nil {
		t.Fatal("directory target discovery succeeded")
	}
}

func TestDirectoryScopeDiscoverRespectsDepth(t *testing.T) {
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	writeTestFile(t, filepath.Join(root, "deep", "topic", "details.md"), "# Deep")

	shallow := rootScope{root: root, discovery: files.DiscoverOptions{Depth: 0}}
	got, err := shallow.discover(context.Background())
	if err != nil {
		t.Fatalf("depth 0 discover error = %v", err)
	}
	if len(got.Markdown) != 1 || got.Markdown[0].RelativePath != "README.md" {
		t.Fatalf("depth 0 discovery = %+v", got.Markdown)
	}

	deep := rootScope{root: root, discovery: files.DiscoverOptions{Depth: 2}}
	got, err = deep.discover(context.Background())
	if err != nil {
		t.Fatalf("depth 2 discover error = %v", err)
	}
	if len(got.Markdown) != 2 {
		t.Fatalf("depth 2 discovery = %+v", got.Markdown)
	}
}

func TestAllowsDocumentSingleFile(t *testing.T) {
	t.Parallel()

	scope := rootScope{root: t.TempDir(), file: "guide.md"}
	if !scope.allowsDocument("guide.md") {
		t.Fatal("exact file path should be allowed")
	}
	for _, rejected := range []string{"other.md", "GUIDE.MD", "docs/guide.md", "../guide.md", ""} {
		if scope.allowsDocument(rejected) {
			t.Errorf("single-file scope allowed %q", rejected)
		}
	}
}

func TestAllowsDocumentDirectory(t *testing.T) {
	t.Parallel()

	scope := rootScope{root: t.TempDir(), discovery: files.DiscoverOptions{Depth: 1, Pattern: "**/*.md"}}
	if !scope.allowsDocument("guide.md") {
		t.Fatal("in-depth Markdown should be allowed")
	}
	if scope.allowsDocument("deep/topic/details.md") {
		t.Fatal("out-of-depth document was allowed")
	}
	for _, rejected := range []string{"image.png", "notes.txt"} {
		if scope.allowsDocument(rejected) {
			t.Errorf("directory scope allowed non-Markdown %q", rejected)
		}
	}
}

func TestRootScopeKind(t *testing.T) {
	t.Parallel()

	if (rootScope{root: t.TempDir(), file: "guide.md"}).kind() != workspaceSingle {
		t.Fatal("single-file scope kind != single")
	}
	if (rootScope{root: t.TempDir()}).kind() != workspaceDirectory {
		t.Fatal("directory scope kind != directory")
	}
}
