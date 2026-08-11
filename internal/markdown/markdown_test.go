package markdown

import (
	"os"
	"strings"
	"testing"
)

func TestRenderStandardGFM(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("testdata/gfm.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(source, RenderOptions{
		Mode:       ModeAuto,
		Target:     TargetConvert,
		SourcePath: "gfm.md",
	})
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	for _, want := range []string{
		"<table>",
		`type="checkbox"`,
		"<del>removed</del>",
		`href="https://example.com"`,
		`class="chroma"`,
		`class="kn"`,
		`class="kd"`,
	} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("rendered body does not contain %q:\n%s", want, result.Body)
		}
	}
}

func TestRenderRawHTMLAndDangerousURLs(t *testing.T) {
	t.Parallel()

	source := []byte("<span data-raw=\"yes\">raw</span>\n\n[danger](javascript:alert(1))")

	safe, err := Render(source, RenderOptions{Mode: ModeLight, Target: TargetConvert, SourcePath: "safe.md"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(safe.Body, `<span data-raw="yes">`) {
		t.Fatalf("safe render contains raw HTML: %s", safe.Body)
	}
	if strings.Contains(strings.ToLower(safe.Body), "javascript:") {
		t.Fatalf("safe render contains a dangerous URL: %s", safe.Body)
	}

	unsafe, err := Render(source, RenderOptions{
		Mode:       ModeLight,
		Target:     TargetConvert,
		SourcePath: "unsafe.md",
		UnsafeHTML: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(unsafe.Body, `<span data-raw="yes">raw</span>`) {
		t.Fatalf("unsafe render suppressed raw HTML: %s", unsafe.Body)
	}
	if strings.Contains(strings.ToLower(unsafe.Body), "javascript:") {
		t.Fatalf("unsafe render contains a dangerous Markdown URL: %s", unsafe.Body)
	}
}

func TestRenderRewritesLocalLinksAtASTLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		target      Target
		sourcePath  string
		destination string
		want        string
	}{
		{name: "convert relative", target: TargetConvert, sourcePath: "design/current.md", destination: "guide.md", want: "guide.html"},
		{name: "convert dot relative", target: TargetConvert, sourcePath: "design/current.md", destination: "./guide.md#start", want: "./guide.html#start"},
		{name: "convert parent and query", target: TargetConvert, sourcePath: "design/current.md", destination: "../guide.markdown?mode=full#start", want: "../guide.html?mode=full#start"},
		{name: "preview relative", target: TargetPreview, sourcePath: "design/current.md", destination: "guide.md", want: "/doc/design/guide.md"},
		{name: "preview parent", target: TargetPreview, sourcePath: "design/current.md", destination: "../guide.md?mode=full#start", want: "/doc/guide.md?mode=full#start"},
		{name: "absolute URL", target: TargetPreview, sourcePath: "design/current.md", destination: "https://example.com/a.md", want: "https://example.com/a.md"},
		{name: "mailto", target: TargetConvert, sourcePath: "current.md", destination: "mailto:user@example.com", want: "mailto:user@example.com"},
		{name: "anchor", target: TargetConvert, sourcePath: "current.md", destination: "#install", want: "#install"},
		{name: "non markdown", target: TargetConvert, sourcePath: "current.md", destination: "guide.txt", want: "guide.txt"},
		{name: "preview attachment", target: TargetPreview, sourcePath: "design/current.md", destination: "files/guide.pdf?download=1", want: "/assets/design/files/guide.pdf?download=1"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := []byte("[link](" + test.destination + ")")
			result, err := Render(source, RenderOptions{
				Mode:       ModeAuto,
				Target:     test.target,
				SourcePath: test.sourcePath,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.Body, `href="`+test.want+`"`) {
				t.Fatalf("body does not contain rewritten link %q: %s", test.want, result.Body)
			}
		})
	}
}

func TestRenderRewritesPreviewAssetsAndPreservesConvertAssets(t *testing.T) {
	t.Parallel()

	source := []byte("![diagram](../images/diagram.png?raw=1#preview)")
	preview, err := Render(source, RenderOptions{Mode: ModeAuto, Target: TargetPreview, SourcePath: "design/current.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.Body, `src="/assets/images/diagram.png?raw=1#preview"`) {
		t.Fatalf("preview image was not rewritten: %s", preview.Body)
	}

	convert, err := Render(source, RenderOptions{Mode: ModeAuto, Target: TargetConvert, SourcePath: "design/current.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(convert.Body, `src="../images/diagram.png?raw=1#preview"`) {
		t.Fatalf("convert image URL changed: %s", convert.Body)
	}
}

func TestRenderExtractsFirstH1Title(t *testing.T) {
	t.Parallel()

	result, err := Render(
		[]byte("## Before\n\n# Hello *emphasis* [link](guide.md) `code`\n\n# Later"),
		RenderOptions{Mode: ModeDark, Target: TargetConvert, SourcePath: "fallback.md"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Title, "Hello emphasis link code"; got != want {
		t.Fatalf("Title = %q, want %q", got, want)
	}
	if !strings.Contains(result.HTML, "<title>Hello emphasis link code</title>") {
		t.Fatalf("HTML title is missing: %s", result.HTML)
	}
}

func TestRenderUsesFilenameWhenH1IsMissing(t *testing.T) {
	t.Parallel()

	result, err := Render(
		[]byte("No heading"),
		RenderOptions{Mode: ModeAuto, Target: TargetPreview, SourcePath: "notes/计划.md"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Title, "计划.md"; got != want {
		t.Fatalf("Title = %q, want %q", got, want)
	}
	for _, want := range []string{
		"<!doctype html>",
		"<title>计划.md</title>",
		`class="m2h-mode-auto"`,
		`data-target="preview"`,
		`class="markdown-body"`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Errorf("HTML does not contain %q: %s", want, result.HTML)
		}
	}
}

func TestConvertAndPreviewShareRenderedBody(t *testing.T) {
	t.Parallel()

	source := []byte("# Title\n\nText with [guide](guide.md).")
	convert, err := Render(source, RenderOptions{Mode: ModeAuto, Target: TargetConvert, SourcePath: "docs/current.md"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Render(source, RenderOptions{Mode: ModeAuto, Target: TargetPreview, SourcePath: "docs/current.md"})
	if err != nil {
		t.Fatal(err)
	}

	normalizedPreview := strings.Replace(preview.Body, "/doc/docs/guide.md", "guide.html", 1)
	if convert.Body != normalizedPreview {
		t.Fatalf("convert and preview bodies differ beyond rewritten link:\nconvert: %s\npreview: %s", convert.Body, preview.Body)
	}
	if convert.HTML == preview.HTML {
		t.Fatal("target-specific outer documents are identical")
	}
	if strings.Contains(convert.HTML, "EventSource") {
		t.Fatal("convert HTML contains preview live-reload client")
	}
	for _, want := range []string{"new EventSource(\"/api/events\")", `addEventListener("document-changed"`} {
		if !strings.Contains(preview.HTML, want) {
			t.Errorf("preview HTML does not contain %q: %s", want, preview.HTML)
		}
	}
}

func TestRenderRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	for _, options := range []RenderOptions{
		{Mode: "sepia", Target: TargetConvert, SourcePath: "doc.md"},
		{Mode: ModeAuto, Target: "terminal", SourcePath: "doc.md"},
		{Mode: ModeAuto, Target: TargetPreview, SourcePath: "../outside.md"},
		{Mode: ModeAuto, Target: TargetConvert, SourcePath: "doc.md", Width: "huge"},
	} {
		if _, err := Render([]byte("text"), options); err == nil {
			t.Fatalf("Render() accepted invalid options: %+v", options)
		}
	}
}

func TestRenderRichMarkdownFixture(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("testdata/rich.md")
	if err != nil {
		t.Fatal(err)
	}

	convert, err := Render(source, RenderOptions{
		Mode: ModeAuto, Target: TargetConvert, SourcePath: "rich.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Render(source, RenderOptions{
		Mode: ModeAuto, Target: TargetPreview, SourcePath: "rich.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Lists, fenced code and math delimiters must survive the shared renderer so
	// the browser-side rich-content layer (KaTeX/Mermaid) and the list CSS fix
	// have something to operate on. Convert and preview share the same body.
	for _, result := range []Result{convert, preview} {
		for _, want := range []string{
			"<ul>",
			"<li>A</li>",
			"<li>B.1</li>",
			"<ol>",
			`class="language-mermaid"`,
			`class="chroma"`,
			"$E = mc^2$",
			"$$",
			"\\int_0^\\infty",
			"\\frac{\\sqrt{\\pi}}{2}",
		} {
			if !strings.Contains(result.Body, want) {
				t.Errorf("rendered body missing %q", want)
			}
		}
	}
}

// TestRenderBackslashMathDelimitersAreConsumed locks the known limitation that
// CommonMark backslash escaping consumes the backslash from \( \) and \[ \],
// so only $...$ and $$...$$ reliably reach the KaTeX auto-render layer.
func TestRenderBackslashMathDelimitersAreConsumed(t *testing.T) {
	t.Parallel()

	result, err := Render([]byte(`\( a \) \[ b \]`), RenderOptions{
		Mode: ModeAuto, Target: TargetConvert, SourcePath: "math.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Body, `\(`) || strings.Contains(result.Body, `\[`) {
		t.Fatalf("backslash math delimiters were preserved, expected them consumed: %s", result.Body)
	}
	for _, want := range []string{"( a )", "[ b ]"} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("expected rendered %q in %s", want, result.Body)
		}
	}
}

func TestRenderInjectsRichContentAssets(t *testing.T) {
	t.Parallel()

	source := []byte("# Title\n\nbody")

	withAssets, err := Render(source, RenderOptions{
		Mode: ModeAuto, Target: TargetPreview, SourcePath: "guide.md",
		AssetBase: "/m2h-assets/",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="/m2h-assets/katex.min.css">`,
		`<script src="/m2h-assets/katex.min.js"></script>`,
		`<script src="/m2h-assets/auto-render.min.js"></script>`,
		`<script src="/m2h-assets/mermaid.min.js"></script>`,
		`<script src="/m2h-assets/rich-content.js"></script>`,
	} {
		if !strings.Contains(withAssets.HTML, want) {
			t.Errorf("HTML with AssetBase missing %q", want)
		}
	}

	withoutAssets, err := Render(source, RenderOptions{
		Mode: ModeAuto, Target: TargetConvert, SourcePath: "guide.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"katex.min.css", "mermaid.min.js", "rich-content.js"} {
		if strings.Contains(withoutAssets.HTML, unwanted) {
			t.Errorf("HTML without AssetBase unexpectedly contains %q", unwanted)
		}
	}

	nested, err := Render(source, RenderOptions{
		Mode: ModeAuto, Target: TargetConvert, SourcePath: "guide.md",
		AssetBase: "../../.m2h/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nested.HTML, `href="../../.m2h/katex.min.css"`) {
		t.Errorf("relative asset base not preserved: %s", nested.HTML)
	}
}

func TestTitleUsesSharedASTExtraction(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		source     string
		sourcePath string
		want       string
	}{
		{source: "## Before\n\n# Hello *world* `code`", sourcePath: "guide.md", want: "Hello world code"},
		{source: "No H1", sourcePath: "notes/计划.md", want: "计划.md"},
	} {
		title, err := Title([]byte(test.source), test.sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if title != test.want {
			t.Errorf("Title() = %q, want %q", title, test.want)
		}
	}
	if _, err := Title([]byte("text"), "../outside.md"); err == nil {
		t.Fatal("Title() accepted an escaping source path")
	}
}

func TestRenderGitHubHeadingIDs(t *testing.T) {
	t.Parallel()

	source := []byte("# GitHub Extensions\n\n## 7. 代码\n\n[跳转到第 7 节](#7-代码)\n\n## API\n\n## API\n")
	result, err := Render(source, RenderOptions{
		Mode: ModeAuto, Target: TargetConvert, SourcePath: "anchors.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`<h2 id="7-代码">7. 代码</h2>`,
		`<h2 id="api">API</h2>`,
		`<h2 id="api-1">API</h2>`,
	} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("body missing %q:\n%s", want, result.Body)
		}
	}

	// Fragment links stay in-page and are not rewritten to .html/.md. Goldmark
	// percent-encodes the non-ASCII bytes in the href (standard, browser-safe);
	// browsers decode the fragment before matching the Unicode id, so the anchor
	// still resolves without JavaScript.
	if !strings.Contains(result.Body, `href="#7-`) || !strings.Contains(result.Body, "跳转到第 7 节</a>") {
		t.Errorf("fragment link to #7-代码 missing or rewritten:\n%s", result.Body)
	}
}

func TestRenderStripsDangerousImageURL(t *testing.T) {
	t.Parallel()

	result, err := Render([]byte("![danger](javascript:alert(1))"), RenderOptions{
		Mode: ModeAuto, Target: TargetConvert, SourcePath: "doc.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(result.Body), "javascript:") {
		t.Fatalf("dangerous image URL was not stripped: %s", result.Body)
	}
}

func TestRenderPreviewImageOutsideRootUnchanged(t *testing.T) {
	t.Parallel()

	result, err := Render([]byte("![escape](../../outside.png)"), RenderOptions{
		Mode: ModeAuto, Target: TargetPreview, SourcePath: "design/current.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Body, `src="../../outside.png"`) {
		t.Fatalf("out-of-root image was rewritten instead of left unchanged: %s", result.Body)
	}
}
