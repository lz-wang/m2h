package export

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

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

func TestRunExportsSingleFileToDefaultAndExplicitOutput(t *testing.T) {
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
	for _, want := range []string{"<!doctype html>", "<title>Guide</title>", `href="next.md"`, `class="markdown-body"`, `class="m2h-mode-auto"`, `data-width="standard"`} {
		if !bytes.Contains(defaultHTML, []byte(want)) {
			t.Errorf("default output does not contain %q", want)
		}
	}

	// Output is a filename in the source directory — the exported HTML keeps
	// referencing the source tree's images and links.
	options := defaultOptions(source)
	options.Output = "index.html"
	options.Mode = markdown.ModeDark
	named, err := Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := named.Output, filepath.Join(resolvedRoot, "index.html"); got != want {
		t.Fatalf("named output = %q, want %q", got, want)
	}
	namedHTML, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(namedHTML, []byte(`class="m2h-mode-dark"`)) {
		t.Fatal("named output did not use dark mode")
	}
}

func TestRunRejectsDirectoryInput(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, source, "guide.md", "# Guide")

	_, err := Run(context.Background(), defaultOptions(source))
	if err == nil || !strings.Contains(err.Error(), "export requires a Markdown file") {
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
		{name: "non Markdown file", options: defaultOptions(textFile), want: "export requires a Markdown file"},
		{name: "same output", options: func() Options { options := defaultOptions(markdownFile); options.Output = "guide.md"; return options }(), want: "output conflicts with input"},
		{name: "output subdirectory", options: func() Options {
			options := defaultOptions(markdownFile)
			options.Output = "public/index.html"
			return options
		}(), want: "must be a plain filename, not a path"},
		{name: "output escapes directory", options: func() Options {
			options := defaultOptions(markdownFile)
			options.Output = "../guide.html"
			return options
		}(), want: "must be a plain filename, not a path"},
		{name: "absolute output", options: func() Options {
			options := defaultOptions(markdownFile)
			options.Output = "/tmp/guide.html"
			return options
		}(), want: "must be a plain filename, not a path"},
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

func TestExportReportsReadAndWritePermissionErrors(t *testing.T) {
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

	t.Run("source directory unwritable", func(t *testing.T) {
		root := t.TempDir()
		source := writeFixture(t, root, "guide.md", "# Guide")
		if err := os.Chmod(root, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
		if _, err := Run(context.Background(), defaultOptions(source)); err == nil || !strings.Contains(err.Error(), "write HTML") {
			t.Fatalf("Run() error = %v, want write error", err)
		}
	})
}

func TestRunReportsMissingInput(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := Run(context.Background(), defaultOptions(missing)); err == nil || !strings.Contains(err.Error(), "input") {
		t.Fatalf("Run() error = %v, want missing input error", err)
	}
}

func TestRunExportPreservesRichContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md",
		"# Guide\n\nInline $E = mc^2$ here.\n\n```mermaid\nflowchart LR\n    A-->B\n```\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n")
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
		`<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.18.4/dist/katex.min.css">`,
		`<script src="https://cdn.jsdelivr.net/npm/katex@0.18.4/dist/katex.min.js"></script>`,
		`<script src="https://cdn.jsdelivr.net/npm/katex@0.18.4/dist/contrib/auto-render.min.js"></script>`,
		`<script src="https://cdn.jsdelivr.net/npm/mermaid@11.16.1/dist/mermaid.min.js"></script>`,
		`<script src="https://cdn.jsdelivr.net/npm/tablesort@5.3.0/dist/tablesort.min.js"></script>`,
		"renderMathInElement",
		`var LITERAL_DOLLAR_CLASS = "m2h-literal-dollar"`,
		"protectLiteralDollars(root)",
		"ignoredClasses: [LITERAL_DOLLAR_CLASS]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("export output missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"data:font/woff2",
		`src="data:`,
		`.m2h/`,
		// Export loads only the Tablesort core from the CDN; the typed
		// comparators the WebUI embeds stay Web-only, so no comparator URL
		// (the pre-simplification dist/tablesort.date.js was invalid anyway —
		// upstream ships comparators under dist/sorts/) may appear.
		"tablesort.date",
		"tablesort.dotsep",
		"tablesort.filesize",
		"tablesort.monthname",
		"tablesort.number",
		"dist/sorts",
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("CDN output unexpectedly embeds %q", unwanted)
		}
	}
}

func TestRunExportRegistersZenUMLPluginOnDemand(t *testing.T) {
	t.Parallel()

	// A zenuml diagram needs both Mermaid Core and the external-diagram
	// plugin: the page pins the core CDN script and carries the plugin module
	// URL the bootstrap imports at runtime.
	root := t.TempDir()
	source := writeFixture(t, root, "zenuml.md",
		"# ZenUML\n\n```mermaid\nzenuml\n    Alice->John: Hello\n```\n")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(root, "zenuml.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	for _, want := range []string{
		`<script src="https://cdn.jsdelivr.net/npm/mermaid@11.16.1/dist/mermaid.min.js"></script>`,
		`window.m2hZenUMLModuleURL = "https://cdn.jsdelivr.net/npm/@mermaid-js/mermaid-zenuml@0.2.3/dist/mermaid-zenuml.esm.min.mjs"`,
		`mermaid.startOnLoad = false`,
		`function withoutAddedHostStylesheets(operation)`,
		`if (!retained.has(stylesheet))`,
		`function applyZenUMLTheme(root, dark)`,
		`data-m2h-zenuml-theme-style`,
		`fill: #1f2020`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("zenuml export missing %q", want)
		}
	}

	// A plain flowchart must not carry the plugin URL: the inlined bootstrap
	// always contains the detector text and the global's read site, but only
	// the CDN assignment makes the browser download the multi-megabyte plugin.
	flowRoot := t.TempDir()
	flow := writeFixture(t, flowRoot, "flow.md",
		"# Flow\n\n```mermaid\nflowchart LR\n    A-->B\n```\n")
	if _, err := Run(context.Background(), defaultOptions(flow)); err != nil {
		t.Fatal(err)
	}
	flowHTML, err := os.ReadFile(filepath.Join(flowRoot, "flow.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(flowHTML), `https://cdn.jsdelivr.net/npm/mermaid@11.16.1/dist/mermaid.min.js`) {
		t.Error("flowchart export missing the Mermaid core CDN script")
	}
	if strings.Contains(string(flowHTML), "@mermaid-js/mermaid-zenuml@") {
		t.Error("flowchart export unexpectedly carries the ZenUML plugin URL")
	}
}

func TestRunExportEmbedsVegaLiteRuntimeOnDemand(t *testing.T) {
	t.Parallel()

	// A chart needs the whole dependency chain: vega attaches the runtime,
	// vega-lite compiles against it, and vega-embed receives both as globals —
	// so the page pins all three CDN scripts in exactly that order and the
	// bootstrap embeds under the same host policy as the WebUI.
	root := t.TempDir()
	source := writeFixture(t, root, "chart.md",
		"# Chart\n\n```vega-lite\n{\"data\":{\"values\":[{\"a\":1}]},\"mark\":\"bar\",\"encoding\":{\"x\":{\"field\":\"a\",\"type\":\"quantitative\"}}}\n```\n")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	html, err := os.ReadFile(filepath.Join(root, "chart.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	vega := `<script src="https://cdn.jsdelivr.net/npm/vega@6.4.0/build/vega.min.js"></script>`
	vegaLite := `<script src="https://cdn.jsdelivr.net/npm/vega-lite@6.4.3/build/vega-lite.min.js"></script>`
	vegaEmbed := `<script src="https://cdn.jsdelivr.net/npm/vega-embed@7.1.0/build/vega-embed.min.js"></script>`
	for _, want := range []string{
		`class="language-vega-lite"`,
		vega, vegaLite, vegaEmbed,
		"embedVegaLiteCharts",
		`mode: "vega-lite"`,
		`renderer: "svg"`,
		"actions: false",
		"external Vega-Lite data loading is not supported",
		"function withoutEmbedOptions(spec)",
		"vegaLiteHostConfig",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("vega-lite export missing %q", want)
		}
	}
	// The dependency order is load-bearing: each script reads its
	// predecessor's window global when it evaluates.
	if !((strings.Index(body, vega) < strings.Index(body, vegaLite)) &&
		(strings.Index(body, vegaLite) < strings.Index(body, vegaEmbed))) {
		t.Error("vega CDN scripts are not emitted in dependency order")
	}

	// The `vegalite` alias drives the same trio through the same detection.
	aliasRoot := t.TempDir()
	alias := writeFixture(t, aliasRoot, "alias.md",
		"# Alias\n\n```vegalite\n{\"data\":{\"values\":[{\"a\":1}]},\"mark\":\"bar\",\"encoding\":{\"x\":{\"field\":\"a\",\"type\":\"quantitative\"}}}\n```\n")
	if _, err := Run(context.Background(), defaultOptions(alias)); err != nil {
		t.Fatal(err)
	}
	aliasHTML, err := os.ReadFile(filepath.Join(aliasRoot, "alias.html"))
	if err != nil {
		t.Fatal(err)
	}
	aliasBody := string(aliasHTML)
	if !strings.Contains(aliasBody, `class="language-vegalite"`) {
		t.Error("alias export missing the vegalite language class")
	}
	for _, want := range []string{vega, vegaLite, vegaEmbed} {
		if !strings.Contains(aliasBody, want) {
			t.Errorf("alias export missing CDN script %q", want)
		}
	}

	// A document that only mentions the string "vega-lite" in plain or fenced
	// code must not trigger the runtime trio.
	plainRoot := t.TempDir()
	plain := writeFixture(t, plainRoot, "plain.md",
		"# Plain\n\nThe word vega-lite here is prose.\n\n```text\nvega-lite mentioned in code\n```\n\n```html\n<pre><code class=\"language-vega-lite\">escaped</code></pre>\n```\n")
	if _, err := Run(context.Background(), defaultOptions(plain)); err != nil {
		t.Fatal(err)
	}
	plainHTML, err := os.ReadFile(filepath.Join(plainRoot, "plain.html"))
	if err != nil {
		t.Fatal(err)
	}
	plainBody := string(plainHTML)
	// Goldmark escapes the raw-HTML sample's quotes, so a bare class
	// attribute never appears; prose and fenced text carry no class at all.
	for _, unwanted := range []string{
		`<script src="https://cdn.jsdelivr.net/npm/vega@`,
		`<script src="https://cdn.jsdelivr.net/npm/vega-lite@`,
		`<script src="https://cdn.jsdelivr.net/npm/vega-embed@`,
	} {
		if strings.Contains(plainBody, unwanted) {
			t.Errorf("plain vega-lite mention unexpectedly loads %q", unwanted)
		}
	}
}

func TestRunExportWithoutRichContentStaysLean(t *testing.T) {
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
	if !strings.Contains(body, "renderMathInElement") {
		t.Errorf("plain document missing the bootstrap enhancer")
	}
	if strings.Contains(body, "cdn.jsdelivr.net") {
		t.Errorf("plain document unexpectedly loads CDN runtimes")
	}
	if strings.Contains(body, "m2hZenUMLModuleURL = ") {
		t.Errorf("plain document unexpectedly carries the ZenUML plugin URL")
	}
}

func TestRunExportWritesOnlyTheHTMLFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := writeFixture(t, root, "guide.md", "# Guide\n\n$E=mc^2$")
	if _, err := Run(context.Background(), defaultOptions(source)); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Name() != "guide.md" {
			names = append(names, entry.Name())
		}
	}
	if !reflect.DeepEqual(names, []string{"guide.html"}) {
		t.Fatalf("export wrote extra files: %v", names)
	}
}

func TestRunExportKeepsImageReferences(t *testing.T) {
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
	// Local, remote, and missing images all keep their original references —
	// the HTML relies on the files staying where the Markdown found them.
	for _, keep := range []string{`src="img/diagram.png"`, `src="https://example.com/x.png"`, `src="img/nope.png"`} {
		if !strings.Contains(body, keep) {
			t.Errorf("output missing preserved reference %q", keep)
		}
	}
	if strings.Contains(body, "base64") {
		t.Errorf("images should not be embedded: %s", body)
	}
}
