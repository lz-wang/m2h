package convert

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/markdown"
)

func writeFixture(t *testing.T, root, relative, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func defaultOptions(input string) Options {
	return Options{
		Input:      input,
		Depth:      2,
		Mode:       markdown.ModeAuto,
		CopyAssets: true,
	}
}

func snapshotDirectory(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestRunConvertsSingleFileToDefaultAndExplicitOutput(t *testing.T) {
	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide\n\n[Next](next.md)")
	writeFixture(t, root, "next.md", "# Next")

	if err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}
	defaultHTML, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>Guide</title>", `href="next.html"`, `class="markdown-body"`} {
		if !bytes.Contains(defaultHTML, []byte(want)) {
			t.Errorf("default output does not contain %q", want)
		}
	}

	explicit := filepath.Join(root, "public", "index.html")
	options := defaultOptions(source)
	options.Output = explicit
	options.Mode = markdown.ModeDark
	if err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	explicitHTML, err := os.ReadFile(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(explicitHTML, []byte(`class="m2h-mode-dark"`)) {
		t.Fatal("explicit output did not use dark mode")
	}
}

func TestRunDirectoryDepthGlobAssetsAndTrailingSlash(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, source, "root.md", "# Root")
	writeFixture(t, source, "a/keep.md", "# Keep\n\n![asset](asset.png)")
	writeFixture(t, source, "a/skip.md", "# Skip")
	writeFixture(t, source, "a/asset.png", "png")
	writeFixture(t, source, "a/b/deep.md", "# Deep")

	outputs := []string{filepath.Join(t.TempDir(), "plain"), filepath.Join(t.TempDir(), "slash")}
	inputs := []string{source, source + string(os.PathSeparator)}
	for index, input := range inputs {
		options := defaultOptions(input)
		options.Output = outputs[index]
		options.Depth = 1
		options.Pattern = "**/{root,keep}.md"
		if err := Run(context.Background(), options); err != nil {
			t.Fatal(err)
		}
	}

	for _, output := range outputs {
		for _, relative := range []string{"root.html", "a/keep.html", "a/asset.png"} {
			if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(relative))); err != nil {
				t.Errorf("missing %s: %v", relative, err)
			}
		}
		for _, relative := range []string{"a/skip.html", "a/b/deep.html"} {
			if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(relative))); !os.IsNotExist(err) {
				t.Errorf("unexpected %s", relative)
			}
		}
	}
	if first, second := snapshotDirectory(t, outputs[0]), snapshotDirectory(t, outputs[1]); !reflect.DeepEqual(first, second) {
		t.Fatalf("directory output differs with trailing slash:\nplain=%v\nslash=%v", first, second)
	}
}

func TestRunDirectoryCanDisableAssetCopy(t *testing.T) {
	source := t.TempDir()
	output := t.TempDir()
	writeFixture(t, source, "guide.md", "# Guide")
	writeFixture(t, source, "image.png", "png")

	options := defaultOptions(source)
	options.Output = output
	options.CopyAssets = false
	if err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "guide.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "image.png")); !os.IsNotExist(err) {
		t.Fatalf("asset exists with copy disabled: %v", err)
	}
}

func TestRunDirectoryAllowsEmptyMarkdownMatch(t *testing.T) {
	source := t.TempDir()
	output := t.TempDir()
	writeFixture(t, source, "guide.md", "# Guide")
	options := defaultOptions(source)
	options.Output = output
	options.Pattern = "**/missing-*.md"
	if err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty match produced output: %v", entries)
	}
}

func TestRunExcludesOutputInsideSource(t *testing.T) {
	source := t.TempDir()
	output := filepath.Join(source, "public")
	writeFixture(t, source, "guide.md", "# Guide")
	writeFixture(t, output, "stale.md", "# Stale")

	options := defaultOptions(source)
	options.Output = output
	if err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "guide.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "stale.html")); !os.IsNotExist(err) {
		t.Fatalf("output subtree was converted recursively: %v", err)
	}
}

func TestRunDetectsDestinationConflictsBeforeWriting(t *testing.T) {
	tests := []struct {
		name    string
		fixture map[string]string
	}{
		{
			name: "two Markdown names",
			fixture: map[string]string{
				"same.md":       "md",
				"same.markdown": "markdown",
			},
		},
		{
			name: "HTML asset",
			fixture: map[string]string{
				"same.md":   "md",
				"same.html": "asset",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			output := t.TempDir()
			for relative, contents := range test.fixture {
				writeFixture(t, source, relative, contents)
			}
			options := defaultOptions(source)
			options.Output = output
			if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "conflict") {
				t.Fatalf("Run() error = %v, want conflict", err)
			}
			entries, err := os.ReadDir(output)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("wrote output before reporting conflict: %v", entries)
			}
		})
	}
}

func TestRunSingleFileRejectsDirectoryOnlyOptions(t *testing.T) {
	source := writeFixture(t, t.TempDir(), "guide.md", "# Guide")
	tests := []struct {
		name   string
		mutate func(*Options)
		want   string
	}{
		{name: "glob", mutate: func(options *Options) { options.Pattern = "*.md"; options.PatternSet = true }, want: "--glob can only be used when converting a directory"},
		{name: "depth", mutate: func(options *Options) { options.DepthSet = true }, want: "--depth can only be used when converting a directory"},
		{name: "copy assets", mutate: func(options *Options) { options.CopyAssetsSet = true }, want: "--copy-assets can only be used when converting a directory"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			options := defaultOptions(source)
			test.mutate(&options)
			if err := Run(context.Background(), options); err == nil || err.Error() != test.want {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunRejectsInvalidInputAndOutputTypes(t *testing.T) {
	root := t.TempDir()
	markdownFile := writeFixture(t, root, "guide.md", "# Guide")
	textFile := writeFixture(t, root, "guide.txt", "text")

	tests := []struct {
		name    string
		options Options
		want    string
	}{
		{name: "missing input option", options: Options{Depth: 2, Mode: markdown.ModeAuto}, want: "input path is required"},
		{name: "non Markdown file", options: defaultOptions(textFile), want: "expected a Markdown file"},
		{name: "same output", options: func() Options { options := defaultOptions(markdownFile); options.Output = markdownFile; return options }(), want: "output conflicts with input"},
		{name: "file output is directory", options: func() Options { options := defaultOptions(markdownFile); options.Output = t.TempDir(); return options }(), want: "is a directory"},
		{name: "directory output is file", options: func() Options { options := defaultOptions(root); options.Output = textFile; return options }(), want: "is not a directory"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if err := Run(context.Background(), test.options); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunDirectoryDefaultsToInPlaceOutput(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, source, "guide.md", "# Guide")
	writeFixture(t, source, "asset.png", "asset")

	if err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(source, "guide.html")); err != nil {
		t.Fatal(err)
	}
	asset, err := os.ReadFile(filepath.Join(source, "asset.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(asset) != "asset" {
		t.Fatalf("in-place conversion changed asset: %q", asset)
	}
}

func TestRunValidatesOptionsBeforeInputAccess(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, options := range []Options{
		{Input: missing, Depth: -1, Mode: markdown.ModeAuto},
		{Input: missing, Depth: 2, Pattern: "[", Mode: markdown.ModeAuto},
		{Input: missing, Depth: 2, Mode: "sepia"},
	} {
		if err := Run(context.Background(), options); err == nil || strings.Contains(err.Error(), "missing") {
			t.Fatalf("Run() error = %v, want option error before input error", err)
		}
	}
}

func TestRunSupportsRootAndInternalSafeSymlinkFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	source := t.TempDir()
	writeFixture(t, source, "real.md", "# Real")
	if err := os.Symlink(filepath.Join(source, "real.md"), filepath.Join(source, "alias.md")); err != nil {
		t.Fatal(err)
	}
	rootLink := filepath.Join(t.TempDir(), "source-link")
	if err := os.Symlink(source, rootLink); err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	options := defaultOptions(rootLink)
	options.Output = output
	if err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alias.html", "real.html"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestRunHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(ctx, defaultOptions(t.TempDir())); err == nil {
		t.Fatal("Run() ignored cancellation")
	}
}

func TestChangeExtensionUsesNormalizedRelativePaths(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"guide.md":                "guide.html",
		"design/guide.markdown":   "design/guide.html",
		`design\windows\guide.MD`: "design/windows/guide.html",
	}
	for input, want := range tests {
		if got := changeExtension(input); got != want {
			t.Errorf("changeExtension(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPlannedDestinationsAreStable(t *testing.T) {
	t.Parallel()

	plans := []plannedFile{{destination: "z"}, {destination: "a"}}
	sortPlans(plans)
	if got, want := []string{plans[0].destination, plans[1].destination}, []string{"a", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted plans = %#v, want %#v", got, want)
	}
	if samePath(string([]byte{0}), "valid") {
		t.Fatal("samePath() accepted an invalid path")
	}
}

func TestAtomicHelpersReturnErrorsAndCleanTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	blocker := writeFixture(t, root, "blocker", "file")
	existingDirectory := filepath.Join(root, "existing")
	if err := os.Mkdir(existingDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	source := writeFixture(t, root, "asset.txt", "asset")

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "write parent is file", run: func() error { return writeAtomic(filepath.Join(blocker, "child"), []byte("data"), 0o644) }},
		{name: "write destination is directory", run: func() error { return writeAtomic(existingDirectory, []byte("data"), 0o644) }},
		{name: "copy source missing", run: func() error { return copyAtomic(filepath.Join(root, "missing"), filepath.Join(root, "out"), 0o644) }},
		{name: "copy source is directory", run: func() error { return copyAtomic(root, filepath.Join(root, "directory-copy"), 0o644) }},
		{name: "copy parent is file", run: func() error { return copyAtomic(source, filepath.Join(blocker, "child"), 0o644) }},
		{name: "copy destination is directory", run: func() error { return copyAtomic(source, existingDirectory, 0o644) }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatal("helper succeeded, want error")
			}
		})
	}

	matches, err := filepath.Glob(filepath.Join(root, ".m2h-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned: %v", matches)
	}
}

func TestConversionReportsReadAndWritePermissionErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}

	t.Run("source unreadable", func(t *testing.T) {
		source := writeFixture(t, t.TempDir(), "guide.md", "# Guide")
		if err := os.Chmod(source, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(source, 0o644) })
		if err := Run(context.Background(), defaultOptions(source)); err == nil || !strings.Contains(err.Error(), "read Markdown") {
			t.Fatalf("Run() error = %v, want read error", err)
		}
	})

	t.Run("output directory unwritable", func(t *testing.T) {
		root := t.TempDir()
		source := writeFixture(t, root, "guide.md", "# Guide")
		outputDirectory := filepath.Join(root, "locked")
		if err := os.Mkdir(outputDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(outputDirectory, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(outputDirectory, 0o755) })
		options := defaultOptions(source)
		options.Output = filepath.Join(outputDirectory, "guide.html")
		if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "write HTML") {
			t.Fatalf("Run() error = %v, want write error", err)
		}
	})

	t.Run("output parent is file", func(t *testing.T) {
		root := t.TempDir()
		source := writeFixture(t, root, "guide.md", "# Guide")
		blocker := writeFixture(t, root, "blocker", "file")
		options := defaultOptions(source)
		options.Output = filepath.Join(blocker, "guide.html")
		if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "output") {
			t.Fatalf("Run() error = %v, want output error", err)
		}
	})

	t.Run("directory source unreadable", func(t *testing.T) {
		source := t.TempDir()
		markdownFile := writeFixture(t, source, "guide.md", "# Guide")
		if err := os.Chmod(markdownFile, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(markdownFile, 0o644) })
		options := defaultOptions(source)
		options.Output = t.TempDir()
		if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "read Markdown") {
			t.Fatalf("Run() error = %v, want directory read error", err)
		}
	})

	t.Run("directory output unwritable", func(t *testing.T) {
		source := t.TempDir()
		writeFixture(t, source, "guide.md", "# Guide")
		output := t.TempDir()
		if err := os.Chmod(output, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(output, 0o755) })
		options := defaultOptions(source)
		options.Output = output
		if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "write") {
			t.Fatalf("Run() error = %v, want directory write error", err)
		}
	})
}

func TestRunReportsMissingInputAndInvalidOutputParent(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := Run(context.Background(), defaultOptions(missing)); err == nil || !strings.Contains(err.Error(), "input") {
		t.Fatalf("Run() error = %v, want missing input error", err)
	}

	source := t.TempDir()
	writeFixture(t, source, "guide.md", "# Guide")
	blocker := writeFixture(t, t.TempDir(), "blocker", "file")
	options := defaultOptions(source)
	options.Output = filepath.Join(blocker, "output")
	if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "output") {
		t.Fatalf("Run() error = %v, want invalid output parent", err)
	}
}

func TestRunConvertPreservesRichContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md",
		"# Guide\n\nInline $E = mc^2$ here.\n\n```mermaid\nflowchart LR\n    A-->B\n```\n")
	if err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	for _, want := range []string{
		`href=".m2h/katex.min.css"`,
		`src=".m2h/rich-content.js"`,
		`class="language-mermaid"`,
		"$E = mc^2$",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("convert output missing %q", want)
		}
	}
}

func TestRunConvertWritesRichAssetsNextToSingleFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide\n\n$E=mc^2$")
	if err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `href=".m2h/katex.min.css"`) {
		t.Errorf("single-file HTML missing relative rich-content link: %s", html)
	}
	if !strings.Contains(string(html), `src=".m2h/rich-content.js"`) {
		t.Errorf("single-file HTML missing rich-content bootstrap: %s", html)
	}
	for _, rel := range []string{
		".m2h/katex.min.css",
		".m2h/mermaid.min.js",
		".m2h/rich-content.js",
		".m2h/fonts/KaTeX_AMS-Regular.woff2",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("single-file convert did not write %q: %v", rel, err)
		}
	}
}

func TestRunDirectoryWritesSharedRichAssetsWithDepthAwareBase(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	output := t.TempDir()
	writeFixture(t, source, "index.md", "# Index")
	writeFixture(t, source, "a/b/deep.md", "# Deep")

	options := defaultOptions(source)
	options.Output = output
	if err := Run(context.Background(), options); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(output, ".m2h", "katex.min.css")); err != nil {
		t.Fatalf("directory convert did not write shared .m2h/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "a", "b", assets.RichAssetDir)); !os.IsNotExist(err) {
		t.Fatalf("nested %s should not exist, got %v", assets.RichAssetDir, err)
	}

	root, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), `href=".m2h/katex.min.css"`) {
		t.Errorf("root HTML missing .m2h base: %s", root)
	}

	nested, err := os.ReadFile(filepath.Join(output, "a", "b", "deep.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nested), `href="../../.m2h/katex.min.css"`) {
		t.Errorf("nested HTML missing depth-aware base: %s", nested)
	}
}

func TestRunDirectoryRichRuntimeStableAcrossRuns(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	writeFixture(t, source, "index.md", "# Index")
	writeFixture(t, source, "image.png", "png")
	options := defaultOptions(source) // output == source

	if err := Run(context.Background(), options); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	// Re-running into the same source must stay stable: the previously written
	// .m2h/ runtime is excluded from discovery rather than reprocessed.
	if err := Run(context.Background(), options); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	richDirs := 0
	err := filepath.WalkDir(source, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == assets.RichAssetDir {
			richDirs++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if richDirs != 1 {
		t.Fatalf("expected exactly one %s directory after rerun, got %d", assets.RichAssetDir, richDirs)
	}
}
