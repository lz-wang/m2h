package files

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func entryPaths(entries []Entry) []string {
	if len(entries) == 0 {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.RelativePath)
	}
	return paths
}

func TestResolveAcceptsFilesDirectoriesAndRootSymlinks(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "document.md")
	writeTestFile(t, file, "# Title")

	directoryInput, err := Resolve(root + string(os.PathSeparator))
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if directoryInput.Kind != KindDirectory || directoryInput.Path != resolvedRoot {
		t.Fatalf("directory input = %+v", directoryInput)
	}

	fileInput, err := Resolve(file)
	if err != nil {
		t.Fatal(err)
	}
	resolvedFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if fileInput.Kind != KindFile || fileInput.Path != resolvedFile {
		t.Fatalf("file input = %+v", fileInput)
	}

	if runtime.GOOS == "windows" {
		return
	}
	directoryLink := filepath.Join(t.TempDir(), "directory-link")
	if err := os.Symlink(root, directoryLink); err != nil {
		t.Fatal(err)
	}
	linkedDirectory, err := Resolve(directoryLink)
	if err != nil {
		t.Fatal(err)
	}
	if linkedDirectory.Kind != KindDirectory || linkedDirectory.Path != resolvedRoot {
		t.Fatalf("linked directory = %+v", linkedDirectory)
	}

	fileLink := filepath.Join(t.TempDir(), "file-link.md")
	if err := os.Symlink(file, fileLink); err != nil {
		t.Fatal(err)
	}
	linkedFile, err := Resolve(fileLink)
	if err != nil {
		t.Fatal(err)
	}
	if linkedFile.Kind != KindFile || linkedFile.Path != resolvedFile {
		t.Fatalf("linked file = %+v", linkedFile)
	}
}

func TestResolveRejectsMissingAndUnsupportedInputs(t *testing.T) {
	root := t.TempDir()
	if _, err := Resolve(" "); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("Resolve() error = %v, want required path", err)
	}
	if _, err := Resolve(filepath.Join(root, "missing")); err == nil {
		t.Fatal("Resolve() accepted a missing input")
	}
	if runtime.GOOS != "windows" {
		socketFile, err := os.CreateTemp("", "m2h-sock-")
		if err != nil {
			t.Fatal(err)
		}
		socketPath := socketFile.Name()
		if err := socketFile.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(socketPath); err != nil {
			t.Fatal(err)
		}
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = listener.Close()
			_ = os.Remove(socketPath)
		})
		if _, err := Resolve(socketPath); err == nil || !strings.Contains(err.Error(), "regular file or directory") {
			t.Fatalf("Resolve() error = %v, want unsupported type", err)
		}
	}
}

func TestDiscoverRejectsFileRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "guide.md")
	writeTestFile(t, file, "# Guide")
	if _, err := Discover(context.Background(), file, DiscoverOptions{Depth: 2}); err == nil || !strings.Contains(err.Error(), "expected a directory") {
		t.Fatalf("Discover() error = %v, want directory error", err)
	}
}

func TestDiscoverSkipsNonRegularFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets are unavailable")
	}
	root, err := os.MkdirTemp("", "m2h-files-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	writeTestFile(t, filepath.Join(root, "guide.md"), "# Guide")
	socketPath := filepath.Join(root, "short.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	result, err := Discover(context.Background(), root, DiscoverOptions{Depth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(result.Markdown), []string{"guide.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Markdown = %#v, want %#v", got, want)
	}
	if len(result.Assets) != 0 {
		t.Fatalf("non-regular socket was returned as asset: %+v", result.Assets)
	}
}

func TestDiscoverReportsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	writeTestFile(t, filepath.Join(locked, "guide.md"), "# Guide")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := Discover(context.Background(), root, DiscoverOptions{Depth: 2}); err == nil || !strings.Contains(err.Error(), "walk") {
		t.Fatalf("Discover() error = %v, want walk permission error", err)
	}
}

func TestDiscoverAppliesDepthGlobAndStableSorting(t *testing.T) {
	root := t.TempDir()
	for path, contents := range map[string]string{
		"root.md":            "root",
		"a/one.md":           "one",
		"a/two.txt":          "asset",
		"a/b/two.md":         "two",
		"a/b/c/three.md":     "three",
		"z/last.markdown":    "last",
		"z/image/demo.png":   "image",
		"z/image/other.webp": "image",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(path)), contents)
	}

	tests := []struct {
		name         string
		options      DiscoverOptions
		wantMarkdown []string
		wantAssets   []string
	}{
		{
			name:         "depth zero",
			options:      DiscoverOptions{Depth: 0},
			wantMarkdown: []string{"root.md"},
		},
		{
			name:         "depth two",
			options:      DiscoverOptions{Depth: 2},
			wantMarkdown: []string{"a/b/two.md", "a/one.md", "root.md", "z/last.markdown"},
			wantAssets:   []string{"a/two.txt", "z/image/demo.png", "z/image/other.webp"},
		},
		{
			name:         "glob after depth",
			options:      DiscoverOptions{Depth: 3, Pattern: "**/t*.md"},
			wantMarkdown: []string{"a/b/c/three.md", "a/b/two.md"},
			wantAssets:   []string{"a/two.txt", "z/image/demo.png", "z/image/other.webp"},
		},
		{
			name:         "single star root only",
			options:      DiscoverOptions{Depth: 3, Pattern: "*.md"},
			wantMarkdown: []string{"root.md"},
			wantAssets:   []string{"a/two.txt", "z/image/demo.png", "z/image/other.webp"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := Discover(context.Background(), root, test.options)
			if err != nil {
				t.Fatal(err)
			}
			if got := entryPaths(result.Markdown); !reflect.DeepEqual(got, test.wantMarkdown) {
				t.Fatalf("Markdown = %#v, want %#v", got, test.wantMarkdown)
			}
			if got := entryPaths(result.Assets); !reflect.DeepEqual(got, test.wantAssets) {
				t.Fatalf("Assets = %#v, want %#v", got, test.wantAssets)
			}
		})
	}
}

func TestDiscoverSkipsInternalSymlinkDirectoriesAndEscapes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeTestFile(t, filepath.Join(root, "real", "inside.md"), "inside")
	writeTestFile(t, outside, "outside")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real", "inside.md"), filepath.Join(root, "alias.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing.md"), filepath.Join(root, "broken.md")); err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	result, err := Discover(context.Background(), root, DiscoverOptions{Depth: 3, Log: &log})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(result.Markdown), []string{"alias.md", "real/inside.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Markdown = %#v, want %#v", got, want)
	}
	for _, want := range []string{"escape.md", "escapes root", "broken.md"} {
		if !strings.Contains(log.String(), want) {
			t.Fatalf("log does not contain %q: %q", want, log.String())
		}
	}
}

func TestDiscoverSkipHiddenAppliesToSymlinkTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".secret.md"), "secret")
	writeTestFile(t, filepath.Join(root, "docs", "real.md"), "real")
	writeTestFile(t, filepath.Join(root, ".private", "data.json"), "data")
	writeTestFile(t, filepath.Join(root, "hidden-target.png"), "placeholder")
	if err := os.Symlink(filepath.Join(root, ".secret.md"), filepath.Join(root, "public.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".private", "data.json"), filepath.Join(root, "data-alias.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "docs", "real.md"), filepath.Join(root, "alias.md")); err != nil {
		t.Fatal(err)
	}

	// Without SkipHidden the aliases stay: check still sees every hidden
	// document and the links pointing at them.
	visible, err := Discover(context.Background(), root, DiscoverOptions{Depth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(visible.Markdown), []string{".secret.md", "alias.md", "docs/real.md", "public.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipHidden=false Markdown = %#v, want %#v", got, want)
	}

	// With SkipHidden every alias whose canonical target is hidden
	// disappears from both listings; a visible-target alias survives.
	hidden, err := Discover(context.Background(), root, DiscoverOptions{Depth: 4, SkipHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(hidden.Markdown), []string{"alias.md", "docs/real.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipHidden=true Markdown = %#v, want %#v", got, want)
	}
	if got, want := entryPaths(hidden.Assets), []string{"hidden-target.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipHidden=true Assets = %#v, want %#v", got, want)
	}
}

func TestDiscoverExcludesOutputSubtree(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "public")
	writeTestFile(t, filepath.Join(root, "source.md"), "source")
	writeTestFile(t, filepath.Join(output, "generated.md"), "generated")

	result, err := Discover(context.Background(), root, DiscoverOptions{Depth: 3, ExcludeRoot: output})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(result.Markdown), []string{"source.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Markdown = %#v, want %#v", got, want)
	}
}

func TestDiscoverHonorsExcludes(t *testing.T) {
	root := t.TempDir()
	runtime := filepath.Join(root, ".m2h")
	writeTestFile(t, filepath.Join(root, "guide.md"), "guide")
	writeTestFile(t, filepath.Join(root, "image.png"), "image")
	writeTestFile(t, filepath.Join(runtime, "katex.min.css"), "katex")
	writeTestFile(t, filepath.Join(runtime, "fonts", "KaTeX_AMS-Regular.woff2"), "font")

	result, err := Discover(context.Background(), root, DiscoverOptions{
		Depth:    3,
		Excludes: []string{runtime},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(result.Markdown), []string{"guide.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Markdown = %#v, want %#v", got, want)
	}
	if got, want := entryPaths(result.Assets), []string{"image.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Assets = %#v, want %#v (runtime leaked through Excludes)", got, want)
	}
}

func TestDiscoverRejectsUnresolvableExclude(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "guide.md"), "guide")
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	missing := filepath.Join(locked, "missing")
	if _, err := Discover(context.Background(), root, DiscoverOptions{Depth: 2, Excludes: []string{missing}}); err == nil {
		t.Fatal("Discover accepted an unresolvable exclude path")
	}
}

func TestDiscoverValidatesBeforeFilesystemAccess(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := Discover(context.Background(), missing, DiscoverOptions{Depth: 2, Pattern: "["}); err == nil || !strings.Contains(err.Error(), "invalid glob") {
		t.Fatalf("Discover() error = %v, want invalid glob", err)
	}
	if _, err := Discover(context.Background(), missing, DiscoverOptions{Depth: -1}); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("Discover() error = %v, want depth error", err)
	}
}

func TestDiscoverHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Discover(ctx, t.TempDir(), DiscoverOptions{Depth: 2}); err == nil {
		t.Fatal("Discover() ignored cancellation")
	}
}

func TestNormalizeRelativePathUsesSlashSeparators(t *testing.T) {
	t.Parallel()

	if got, want := NormalizeRelativePath(`design\windows\guide.md`), "design/windows/guide.md"; got != want {
		t.Fatalf("NormalizeRelativePath() = %q, want %q", got, want)
	}
}

func TestIsHiddenPathMatchesDotPrefixedComponents(t *testing.T) {
	t.Parallel()

	for _, hidden := range []string{
		".env",
		".env.production",
		".git/config",
		".github/workflows/release.yml",
		".secret/data.json",
		"foo/.private/file.pdf",
		"foo/.hidden.md",
	} {
		if !IsHiddenPath(hidden) {
			t.Errorf("IsHiddenPath(%q) = false, want true", hidden)
		}
	}
	for _, visible := range []string{
		"guide.md",
		"images/logo.png",
		"assets/manual.pdf",
		"download/archive.zip",
		"a/b.md",
	} {
		if IsHiddenPath(visible) {
			t.Errorf("IsHiddenPath(%q) = true, want false", visible)
		}
	}
}

func TestDiscoverSkipsHiddenPaths(t *testing.T) {
	root := t.TempDir()
	for path, contents := range map[string]string{
		"guide.md":                 "guide",
		"images/logo.png":          "logo",
		".env":                     "secret",
		".hidden.md":               "hidden guide",
		".git/config":              "git config",
		".github/workflows/ci.yml": "workflow",
		".secret/data.json":        "secret data",
		"foo/.private/file.pdf":    "private pdf",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(path)), contents)
	}

	// Without SkipHidden nothing changes: static analysis still sees every
	// hidden Markdown and asset, so the web policy never narrows check.
	visible, err := Discover(context.Background(), root, DiscoverOptions{Depth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(visible.Markdown), []string{".hidden.md", "guide.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipHidden=false Markdown = %#v, want %#v", got, want)
	}
	if len(visible.Assets) == 0 {
		t.Fatal("SkipHidden=false discovery lost all assets")
	}

	// With SkipHidden every dot-prefixed component disappears from both the
	// Markdown and the asset listings.
	hidden, err := Discover(context.Background(), root, DiscoverOptions{Depth: 4, SkipHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := entryPaths(hidden.Markdown), []string{"guide.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipHidden=true Markdown = %#v, want %#v", got, want)
	}
	if got, want := entryPaths(hidden.Assets), []string{"images/logo.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipHidden=true Assets = %#v, want %#v", got, want)
	}
}

func TestMatchesHonorsSkipHidden(t *testing.T) {
	t.Parallel()

	options := DiscoverOptions{Depth: 4, SkipHidden: true}
	if !Matches("guide.md", options) {
		t.Fatal("visible path rejected under SkipHidden")
	}
	for _, hidden := range []string{".hidden.md", ".git/README.md", "foo/.private/notes.md"} {
		if Matches(hidden, options) {
			t.Errorf("Matches(%q) = true under SkipHidden", hidden)
		}
	}
	if !Matches(".hidden.md", DiscoverOptions{Depth: 4}) {
		t.Fatal("Matches() changed default behavior without SkipHidden")
	}
}

func TestPathHelpersHandleBoundaries(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(os.PathSeparator), "workspace", "docs")
	if !IsWithin(root, root) || !IsWithin(root, filepath.Join(root, "guide.md")) {
		t.Fatal("IsWithin() rejected an in-root path")
	}
	if IsWithin(root, filepath.Join(filepath.Dir(root), "outside.md")) {
		t.Fatal("IsWithin() accepted an outside path")
	}
	if directoryDepth(".") != 0 || directoryDepth("a/b") != 2 {
		t.Fatal("directoryDepth() returned an unexpected value")
	}
	if IsMarkdown("image.png") || !IsMarkdown("GUIDE.MD") {
		t.Fatal("IsMarkdown() extension handling is incorrect")
	}
}

func TestCanonicalPathPreservesMissingSuffix(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "missing", "output.html")
	got, err := CanonicalPath(want)
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(resolvedRoot, "missing", "output.html") {
		t.Fatalf("CanonicalPath() = %q", got)
	}
}

func TestCanonicalPathRejectsInvalidPath(t *testing.T) {
	t.Parallel()

	if _, err := CanonicalPath(string([]byte{0})); err == nil {
		t.Fatal("CanonicalPath() accepted a NUL path")
	}
}
