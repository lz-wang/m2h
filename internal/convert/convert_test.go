package convert

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
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
		Input: input,
		Mode:  markdown.ModeAuto,
		Width: markdown.WidthStandard,
	}
}

func TestRunConvertsSingleFileToDefaultAndExplicitOutput(t *testing.T) {
	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide\n\n[Next](next.md)")
	writeFixture(t, root, "next.md", "# Next")

	result, err := Run(context.Background(), defaultOptions(source))
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Output, filepath.Join(resolvedRoot, "guide.html"); got != want {
		t.Fatalf("default output = %q, want %q", got, want)
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
	if _, err := Run(context.Background(), options); err != nil {
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

func TestRunRejectsDirectoryInput(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, source, "guide.md", "# Guide")

	_, err := Run(context.Background(), defaultOptions(source))
	if err == nil || !strings.Contains(err.Error(), "convert requires a Markdown file") {
		t.Fatalf("Run() error = %v, want Markdown-file requirement", err)
	}
	if _, err := os.Stat(filepath.Join(source, "guide.html")); !os.IsNotExist(err) {
		t.Fatal("directory input produced output")
	}
}

func TestRunRequiresForceToOverwrite(t *testing.T) {
	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide")
	target := filepath.Join(root, "guide.html")

	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), defaultOptions(source))
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Run() error = %v, want already-exists error", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "existing" {
		t.Fatalf("rejected run overwrote output: %q", contents)
	}

	options := defaultOptions(source)
	options.Force = true
	if _, err := Run(context.Background(), options); err != nil {
		t.Fatalf("Run(--force) returned error: %v", err)
	}
	contents, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte("<title>Guide</title>")) {
		t.Fatal("--force did not replace the output HTML")
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
		{name: "missing input option", options: Options{Mode: markdown.ModeAuto}, want: "input path is required"},
		{name: "non Markdown file", options: defaultOptions(textFile), want: "convert requires a Markdown file"},
		{name: "same output", options: func() Options { options := defaultOptions(markdownFile); options.Output = markdownFile; return options }(), want: "output conflicts with input"},
		{name: "file output is directory", options: func() Options { options := defaultOptions(markdownFile); options.Output = t.TempDir(); return options }(), want: "is a directory"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Run(context.Background(), test.options); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Run() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunValidatesOptionsBeforeInputAccess(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, options := range []Options{
		{Input: missing, Mode: "sepia"},
		{Input: missing, Mode: markdown.ModeAuto, Width: "narrow"},
	} {
		if _, err := Run(context.Background(), options); err == nil || strings.Contains(err.Error(), "missing") {
			t.Fatalf("Run() error = %v, want option error before input error", err)
		}
	}
}

func TestRunResolvesSymlinkedInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	source := t.TempDir()
	writeFixture(t, source, "real.md", "# Real")
	alias := filepath.Join(source, "alias.md")
	if err := os.Symlink(filepath.Join(source, "real.md"), alias); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), defaultOptions(alias)); err != nil {
		t.Fatal(err)
	}
	// The input symlink is resolved to its target, so the HTML lands beside
	// the real file, not the alias.
	if _, err := os.Stat(filepath.Join(source, "real.html")); err != nil {
		t.Fatalf("real.html missing: %v", err)
	}
}

func TestRunHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := writeFixture(t, t.TempDir(), "guide.md", "# Guide")
	if _, err := Run(ctx, defaultOptions(source)); err == nil {
		t.Fatal("Run() ignored cancellation")
	}
}

func TestWriteAtomicReturnsErrorsAndCleansTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	blocker := writeFixture(t, root, "blocker", "file")
	existingDirectory := filepath.Join(root, "existing")
	if err := os.Mkdir(existingDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "write parent is file", run: func() error { return writeAtomic(filepath.Join(blocker, "child"), []byte("data"), 0o644) }},
		{name: "write destination is directory", run: func() error { return writeAtomic(existingDirectory, []byte("data"), 0o644) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

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
		if _, err := Run(context.Background(), defaultOptions(source)); err == nil || !strings.Contains(err.Error(), "read Markdown") {
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
		if _, err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "write HTML") {
			t.Fatalf("Run() error = %v, want write error", err)
		}
	})

	t.Run("output parent is file", func(t *testing.T) {
		root := t.TempDir()
		source := writeFixture(t, root, "guide.md", "# Guide")
		blocker := writeFixture(t, root, "blocker", "file")
		options := defaultOptions(source)
		options.Output = filepath.Join(blocker, "guide.html")
		if _, err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "output") {
			t.Fatalf("Run() error = %v, want output error", err)
		}
	})
}

func TestRunReportsMissingInput(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := Run(context.Background(), defaultOptions(missing)); err == nil || !strings.Contains(err.Error(), "input") {
		t.Fatalf("Run() error = %v, want missing input error", err)
	}
}

// embeddedAssetSnippet returns the opening bytes of one embedded runtime
// script, a fingerprint precise enough to tell which bundles a generated page
// actually carries (the enhancer references the other runtimes by name, so
// plain substring checks on those names are ambiguous).
func embeddedAssetSnippet(t *testing.T, name string, length int) string {
	t.Helper()
	contents, err := assets.RichAssetText(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) < length {
		length = len(contents)
	}
	return contents[:length]
}

func TestRunConvertPreservesRichContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md",
		"# Guide\n\nInline $E = mc^2$ here.\n\n```mermaid\nflowchart LR\n    A-->B\n```\n")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	for _, want := range []string{
		`class="language-mermaid"`,
		"$E = mc^2$",
		// Math pulls in the inline KaTeX stylesheet (fonts as data URIs);
		// the diagram pulls in the Mermaid bundle.
		"data:font/woff2;base64,",
		"m2h-code-copy",
		embeddedAssetSnippet(t, "katex.min.js", 60),
		embeddedAssetSnippet(t, "auto-render.min.js", 60),
		embeddedAssetSnippet(t, "mermaid.min.js", 60),
		embeddedAssetSnippet(t, "rich-content.js", 60),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("convert output missing %q", want)
		}
	}
	for _, unwanted := range []string{
		`.m2h/`,
		"<script src=",
		"url(fonts/",
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("self-contained output unexpectedly contains %q", unwanted)
		}
	}
}

func TestRunConvertWithoutRichContentStaysLean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide\n\nPlain paragraph.\n\n```go\nfmt.Println(1)\n```\n")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	// Only the rich-content enhancer; no KaTeX and no Mermaid bundle.
	if !strings.Contains(body, embeddedAssetSnippet(t, "rich-content.js", 60)) {
		t.Errorf("plain document missing the rich-content enhancer")
	}
	for _, unwanted := range []string{
		embeddedAssetSnippet(t, "katex.min.js", 60),
		embeddedAssetSnippet(t, "mermaid.min.js", 60),
		"data:font/woff2",
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("plain document unexpectedly embeds %q", unwanted)
		}
	}
}

func TestRunConvertWritesNoAssetDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide\n\n$E=mc^2$")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, assets.RichAssetDir)); !os.IsNotExist(err) {
		t.Fatalf("single-file convert wrote %s: %v", assets.RichAssetDir, err)
	}
}

func TestRunConvertInlinesLocalImages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, "img/diagram.png", "\x89PNG\r\n\x1a\nfake-image-bytes")
	source := writeFixture(t, root, "guide.md", "# Guide\n\n![Diagram](img/diagram.png)\n\n![Remote](https://example.com/x.png)\n\n![Missing](img/nope.png)\n")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
	if !strings.Contains(body, want) {
		t.Errorf("local image was not inlined as %q", want)
	}
	// Remote URLs and unreadable images keep their original references.
	for _, keep := range []string{"https://example.com/x.png", "img/nope.png"} {
		if !strings.Contains(body, keep) {
			t.Errorf("output missing preserved reference %q", keep)
		}
	}
}

func TestRunConvertSkipsSymlinkedImageEscape(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	secret := writeFixture(t, outside, "secret.png", "outside-root-image-bytes")
	root := t.TempDir()
	if err := os.Symlink(secret, filepath.Join(root, "linked.png")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	source := writeFixture(t, root, "guide.md", "# Guide\n\n![Secret](linked.png)\n")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "guide.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	if strings.Contains(body, "outside-root-image-bytes") {
		t.Error("image escaping the input root was inlined into the HTML")
	}
	if strings.Contains(body, "base64") {
		t.Errorf("symlinked image should keep its relative reference: %s", body)
	}
}
