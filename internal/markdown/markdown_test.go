package markdown

import (
	"fmt"
	"os"
	"reflect"
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

func TestRenderRawHTMLAndSanitizesDangerousURLs(t *testing.T) {
	t.Parallel()

	source := []byte("<span data-raw=\"yes\">raw</span>\n\n[danger](javascript:alert(1))")

	result, err := Render(source, RenderOptions{SourcePath: "raw-html.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Body, `<span data-raw="yes">raw</span>`) {
		t.Fatalf("render suppressed raw HTML: %s", result.Body)
	}
	if strings.Contains(strings.ToLower(result.Body), "javascript:") {
		t.Fatalf("render contains a dangerous Markdown URL: %s", result.Body)
	}
}

func TestRenderRewritesPreviewRawHTMLURLs(t *testing.T) {
	t.Parallel()

	source := []byte(`<p align="center">
  <img src="web/public/favicon.svg?raw=1#icon" alt="m2h Logo">
  <a href="docs/guide.md#install">Guide</a>
</p>`)
	preview, err := Render(source, RenderOptions{
		URLMode:    URLWeb,
		SourcePath: "README.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`src="/assets/web/public/favicon.svg?raw=1#icon"`,
		`href="/doc/docs/guide.md#install"`,
	} {
		if !strings.Contains(preview.Body, want) {
			t.Errorf("preview raw HTML missing rewritten URL %q: %s", want, preview.Body)
		}
	}

	exported, err := Render(source, RenderOptions{
		SourcePath: "README.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`src="web/public/favicon.svg?raw=1#icon"`,
		`href="docs/guide.md#install"`,
	} {
		if !strings.Contains(exported.Body, want) {
			t.Errorf("export raw HTML changed URL %q: %s", want, exported.Body)
		}
	}
}

func TestRenderRewritesLocalLinksAtASTLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		urlMode     URLMode
		sourcePath  string
		destination string
		want        string
	}{
		{name: "passthrough relative", urlMode: URLPassthrough, sourcePath: "design/current.md", destination: "guide.md", want: "guide.md"},
		{name: "passthrough dot relative", urlMode: URLPassthrough, sourcePath: "design/current.md", destination: "./guide.md#start", want: "./guide.md#start"},
		{name: "passthrough parent and query", urlMode: URLPassthrough, sourcePath: "design/current.md", destination: "../guide.markdown?mode=full#start", want: "../guide.markdown?mode=full#start"},
		{name: "web relative", urlMode: URLWeb, sourcePath: "design/current.md", destination: "guide.md", want: "/doc/design/guide.md"},
		{name: "web parent", urlMode: URLWeb, sourcePath: "design/current.md", destination: "../guide.md?mode=full#start", want: "/doc/guide.md?mode=full#start"},
		{name: "absolute URL", urlMode: URLWeb, sourcePath: "design/current.md", destination: "https://example.com/a.md", want: "https://example.com/a.md"},
		{name: "mailto", urlMode: URLPassthrough, sourcePath: "current.md", destination: "mailto:user@example.com", want: "mailto:user@example.com"},
		{name: "anchor", urlMode: URLPassthrough, sourcePath: "current.md", destination: "#install", want: "#install"},
		{name: "non markdown", urlMode: URLPassthrough, sourcePath: "current.md", destination: "guide.txt", want: "guide.txt"},
		{name: "web attachment", urlMode: URLWeb, sourcePath: "design/current.md", destination: "files/guide.pdf?download=1", want: "/assets/design/files/guide.pdf?download=1"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := []byte("[link](" + test.destination + ")")
			result, err := Render(source, RenderOptions{
				URLMode:    test.urlMode,
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
	preview, err := Render(source, RenderOptions{URLMode: URLWeb, SourcePath: "design/current.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.Body, `src="/assets/images/diagram.png?raw=1#preview"`) {
		t.Fatalf("preview image was not rewritten: %s", preview.Body)
	}

	exported, err := Render(source, RenderOptions{SourcePath: "design/current.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(exported.Body, `src="../images/diagram.png?raw=1#preview"`) {
		t.Fatalf("export image URL changed: %s", exported.Body)
	}
}

func TestClassifyDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		destination string
		want        DestinationKind
	}{
		{destination: "guide.md", want: DestinationRelative},
		{destination: "../guide.md?mode=full#start", want: DestinationRelative},
		{destination: "/docs/guide.md", want: DestinationRootRelative},
		{destination: "/images/logo.png?raw=1#icon", want: DestinationRootRelative},
		{destination: "#install", want: DestinationFragment},
		{destination: "https://example.com/guide.md", want: DestinationExternal},
		{destination: "mailto:user@example.com", want: DestinationExternal},
		{destination: "tel:123", want: DestinationExternal},
		{destination: "//cdn.example.com/a.js", want: DestinationProtocolRelative},
		{destination: "", want: 0},
		{destination: "missing%zz.png", want: 0},
	}
	for _, test := range tests {
		test := test
		t.Run(test.destination, func(t *testing.T) {
			t.Parallel()

			if got := ClassifyDestination(test.destination); got != test.want {
				t.Fatalf("ClassifyDestination(%q) = %d, want %d", test.destination, got, test.want)
			}
		})
	}
}

func TestParseAndResolveLocalDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sourcePath  string
		rootPath    string
		destination string
		wantLocal   LocalDestination
		wantPath    string
		wantOK      bool
	}{
		{
			name:        "document relative",
			sourcePath:  "docs/current.md",
			destination: "images/logo.png?raw=1#icon",
			wantLocal:   LocalDestination{Path: "images/logo.png", Query: "raw=1", Fragment: "icon", Base: DestinationBaseDocument},
			wantPath:    "docs/images/logo.png",
			wantOK:      true,
		},
		{
			name:        "single root relative",
			sourcePath:  "docs/current.md",
			destination: "/images/logo.png#icon",
			wantLocal:   LocalDestination{Path: "images/logo.png", Fragment: "icon", Base: DestinationBaseRoot},
			wantPath:    "images/logo.png",
			wantOK:      true,
		},
		{
			name:        "multi root relative",
			sourcePath:  "r1/docs/current.md",
			rootPath:    "r1",
			destination: "/images/logo.png",
			wantLocal:   LocalDestination{Path: "images/logo.png", Base: DestinationBaseRoot},
			wantPath:    "r1/images/logo.png",
			wantOK:      true,
		},
		{
			name:        "multi root document escape",
			sourcePath:  "r1/docs/current.md",
			rootPath:    "r1",
			destination: "../../r0/secret.md",
			wantLocal:   LocalDestination{Path: "../../r0/secret.md", Base: DestinationBaseDocument},
			wantOK:      false,
		},
		{
			name:        "root traversal",
			sourcePath:  "docs/current.md",
			destination: "/../secret.md",
			wantLocal:   LocalDestination{Path: "../secret.md", Base: DestinationBaseRoot},
			wantOK:      false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			local, ok := ParseLocalDestination(test.destination)
			if !ok {
				t.Fatalf("ParseLocalDestination(%q) did not parse a local destination", test.destination)
			}
			if !reflect.DeepEqual(local, test.wantLocal) {
				t.Fatalf("ParseLocalDestination(%q) = %+v, want %+v", test.destination, local, test.wantLocal)
			}
			got, ok := ResolveLocalDestination(test.sourcePath, test.rootPath, local)
			if ok != test.wantOK || got != test.wantPath {
				t.Fatalf("ResolveLocalDestination() = %q, %t, want %q, %t", got, ok, test.wantPath, test.wantOK)
			}
		})
	}
}

func TestRenderExtractsFirstH1Title(t *testing.T) {
	t.Parallel()

	result, err := Render(
		[]byte("## Before\n\n# Hello *emphasis* [link](guide.md) `code`\n\n# Later"),
		RenderOptions{SourcePath: "fallback.md"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Title, "Hello emphasis link code"; got != want {
		t.Fatalf("Title = %q, want %q", got, want)
	}
}

func TestRenderUsesFilenameWhenH1IsMissing(t *testing.T) {
	t.Parallel()

	result, err := Render(
		[]byte("No heading"),
		RenderOptions{URLMode: URLWeb, SourcePath: "notes/计划.md"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Title, "计划.md"; got != want {
		t.Fatalf("Title = %q, want %q", got, want)
	}
}

func TestPassthroughAndWebShareRenderedBody(t *testing.T) {
	t.Parallel()

	source := []byte("# Title\n\nText with [guide](guide.md).")
	passthrough, err := Render(source, RenderOptions{SourcePath: "docs/current.md"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := Render(source, RenderOptions{URLMode: URLWeb, SourcePath: "docs/current.md"})
	if err != nil {
		t.Fatal(err)
	}

	normalizedWeb := strings.Replace(web.Body, "/doc/docs/guide.md", "guide.md", 1)
	if passthrough.Body != normalizedWeb {
		t.Fatalf("passthrough and web bodies differ beyond rewritten link:\npassthrough: %s\nweb: %s", passthrough.Body, web.Body)
	}
	// Live reload is owned by the React WebUI; the rendered fragment must
	// never embed a live-reload client itself.
	if strings.Contains(passthrough.Body, "EventSource") || strings.Contains(web.Body, "EventSource") {
		t.Fatal("rendered fragment embeds a live-reload client")
	}
}

func TestRenderRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	for _, options := range []RenderOptions{
		{URLMode: URLMode(9), SourcePath: "doc.md"},
		{SourcePath: "../outside.md"},
		{SourcePath: "/absolute.md"},
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

	passthrough, err := Render(source, RenderOptions{SourcePath: "rich.md"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := Render(source, RenderOptions{URLMode: URLWeb, SourcePath: "rich.md"})
	if err != nil {
		t.Fatal(err)
	}

	// Lists, fenced code and math delimiters must survive the shared renderer so
	// the browser-side rich-content layer (KaTeX/Mermaid) and the list CSS fix
	// have something to operate on. Both URL modes share the same body.
	for _, result := range []Result{passthrough, web} {
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
		SourcePath: "math.md",
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
		SourcePath: "anchors.md",
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

func TestRenderExtractsHeadingsFromSharedAST(t *testing.T) {
	t.Parallel()

	source := []byte("# 标题\n\n## 安装\n\n## 安装\n\n### Homebrew\n\n#### C++ API\n")
	result, err := Render(source, RenderOptions{
		SourcePath: "headings.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []Heading{
		{Level: 1, ID: "标题", Text: "标题", Line: 1},
		{Level: 2, ID: "安装", Text: "安装", Line: 3},
		{Level: 2, ID: "安装-1", Text: "安装", Line: 5},
		{Level: 3, ID: "homebrew", Text: "Homebrew", Line: 7},
		{Level: 4, ID: "c-api", Text: "C++ API", Line: 9},
	}
	if !reflect.DeepEqual(result.Headings, want) {
		t.Fatalf("Headings = %+v, want %+v", result.Headings, want)
	}

	// The extracted ids must match the ids actually rendered on the headings, so
	// the table of contents can never drift from the anchors.
	for _, heading := range result.Headings {
		marker := fmt.Sprintf(`<h%d id=%q>`, heading.Level, heading.ID)
		if !strings.Contains(result.Body, marker) {
			t.Errorf("rendered body missing %q:\n%s", marker, result.Body)
		}
	}
}

func TestRenderExtractsNoHeadingsForPlainDocument(t *testing.T) {
	t.Parallel()

	result, err := Render([]byte("A paragraph without headings."), RenderOptions{
		SourcePath: "plain.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Headings) != 0 {
		t.Fatalf("Headings = %+v, want empty", result.Headings)
	}
}

func TestRenderStripsDangerousImageURL(t *testing.T) {
	t.Parallel()

	result, err := Render([]byte("![danger](javascript:alert(1))"), RenderOptions{
		SourcePath: "doc.md",
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
		URLMode: URLWeb, SourcePath: "design/current.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Body, `src="../../outside.png"`) {
		t.Fatalf("out-of-root image was rewritten instead of left unchanged: %s", result.Body)
	}
}

func TestRenderFootnotes(t *testing.T) {
	t.Parallel()

	source := []byte("正文有一个脚注[^1]，再次引用同一脚注[^1]。\n\n" +
		"[^1]: 脚注内容含 [链接](guide.md) 和 `代码`。\n")
	result, err := Render(source, RenderOptions{
		SourcePath: "footnotes.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="footnote-ref"`,
		`class="footnotes"`,
		`href="#fn:1"`,
		"脚注内容含",
		`href="guide.md"`,
		`<code>代码</code>`,
	} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("body missing %q:\n%s", want, result.Body)
		}
	}
	if got := strings.Count(result.Body, `href="#fn:1"`); got < 2 {
		t.Errorf("expected >=2 references to #fn:1, got %d:\n%s", got, result.Body)
	}
}

func TestRenderEmojiShortcodes(t *testing.T) {
	t.Parallel()

	source := []byte("Hello :smile: :rocket: `:smile:` and :not_a_real_emoji:\n\n" +
		"```\n:smile:\n```\n")
	result, err := Render(source, RenderOptions{
		SourcePath: "emoji.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := result.Body
	if !strings.Contains(body, "Hello 😄 🚀") {
		t.Errorf("shortcode not expanded to Unicode emoji:\n%s", body)
	}
	if !strings.Contains(body, "<code>:smile:</code>") {
		t.Errorf("inline code shortcode was expanded instead of kept literal:\n%s", body)
	}
	if !strings.Contains(body, ":not_a_real_emoji:") {
		t.Errorf("unknown shortcode was removed or expanded:\n%s", body)
	}
}

func TestRenderEmojiDoesNotBreakURLs(t *testing.T) {
	t.Parallel()

	result, err := Render([]byte("https://x.com/:smile:/path\n"), RenderOptions{
		SourcePath: "url.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Body, "https://x.com/:smile:/path") {
		t.Errorf("URL containing a shortcode was broken:\n%s", result.Body)
	}
}

func TestRenderGitHubAlerts(t *testing.T) {
	t.Parallel()

	source := []byte("> [!NOTE]\n> Note body.\n\n" +
		"> [!TIP]\n> Tip body.\n\n" +
		"> [!IMPORTANT]\n> Important body.\n\n" +
		"> [!WARNING]\n> Warning body.\n\n" +
		"> [!CAUTION]\n> Caution body.\n")
	result, err := Render(source, RenderOptions{
		SourcePath: "alerts.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="markdown-alert markdown-alert-note"`,
		"Note body",
		`class="markdown-alert markdown-alert-tip"`,
		"Tip body",
		`class="markdown-alert markdown-alert-important"`,
		"Important body",
		`class="markdown-alert markdown-alert-warning"`,
		"Warning body",
		`class="markdown-alert markdown-alert-caution"`,
		"Caution body",
		`<p class="markdown-alert-title">`,
		`class="octicon"`,
		">NOTE</p>",
		">TIP</p>",
		">IMPORTANT</p>",
		">WARNING</p>",
		">CAUTION</p>",
	} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("body missing %q:\n%s", want, result.Body)
		}
	}

	// Markers must be consumed by the alert title, not echoed into the body.
	for _, marker := range []string{"[!NOTE]", "[!TIP]", "[!IMPORTANT]", "[!WARNING]", "[!CAUTION]"} {
		if strings.Contains(result.Body, marker) {
			t.Errorf("alert marker %q leaked into body:\n%s", marker, result.Body)
		}
	}
}

func TestRenderAlertsFallbackToBlockquote(t *testing.T) {
	t.Parallel()

	source := []byte("> Plain quote.\n\n> [!UNKNOWN]\n> Not a real alert.\n\nText [!NOTE] inline.\n")
	result, err := Render(source, RenderOptions{
		SourcePath: "fallback.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Body, "markdown-alert") {
		t.Errorf("plain or unknown-marker blockquote was converted to an alert:\n%s", result.Body)
	}
	for _, want := range []string{"<blockquote>", "Plain quote", "Not a real alert", "[!NOTE]"} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("body missing %q:\n%s", want, result.Body)
		}
	}
}

func TestRenderAlertWithBlankLineAfterMarker(t *testing.T) {
	t.Parallel()

	// When the marker sits on its own paragraph (blank line before the body),
	// the transformer drops the empty first paragraph instead of slicing lines.
	result, err := Render([]byte("> [!NOTE]\n>\n> Body after a blank line.\n"), RenderOptions{
		SourcePath: "alert-blank.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Body, `markdown-alert-note`) {
		t.Errorf("alert not rendered:\n%s", result.Body)
	}
	if !strings.Contains(result.Body, "Body after a blank line") {
		t.Errorf("alert body lost:\n%s", result.Body)
	}
}

func TestRenderGitHubExtensionsFixture(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("testdata/github-extensions.md")
	if err != nil {
		t.Fatal(err)
	}

	exported, err := Render(source, RenderOptions{SourcePath: "github-extensions.md"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Render(source, RenderOptions{URLMode: URLWeb, SourcePath: "github-extensions.md"})
	if err != nil {
		t.Fatal(err)
	}

	// All four GitHub extensions render identically across export and preview.
	for _, result := range []Result{exported, preview} {
		for _, want := range []string{
			`<h2 id="7-代码">7. 代码</h2>`,
			`class="footnotes"`,
			"这是脚注内容",
			"Hello 😄 🚀",
			`<code>:smile:</code>`,
			`class="markdown-alert markdown-alert-note"`,
			`class="markdown-alert markdown-alert-tip"`,
			`class="markdown-alert markdown-alert-important"`,
			`class="markdown-alert markdown-alert-warning"`,
			`class="markdown-alert markdown-alert-caution"`,
			"Useful information",
			"Negative consequences",
		} {
			if !strings.Contains(result.Body, want) {
				t.Errorf("body missing %q:\n%s", want, result.Body)
			}
		}
	}

	// Fragment links stay in-page and are not rewritten to .html/.md.
	if !strings.Contains(exported.Body, `href="#7-`) {
		t.Errorf("fragment link missing or rewritten in export:\n%s", exported.Body)
	}
}
