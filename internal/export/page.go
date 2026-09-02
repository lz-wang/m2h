package export

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/markdown"

	_ "embed"
)

//go:embed runtime.js
var runtimeJS string

// exportBootstrapScript wraps the embedded minimal enhancer (see runtime.js)
// in the script tag every exported page carries.
var exportBootstrapScript = "\n  <script>\n" + runtimeJS + "\n</script>\n"

// Rich-content runtime versions are pinned to the same releases vendored under
// internal/assets/rich for the WebUI, so exported pages and the WebUI render
// identical rich content. Tablesort is the one exception in shape: export
// loads only the core from the CDN — sorting falls back to its default
// comparison — while the WebUI additionally embeds the five typed
// comparators, whose upstream location (dist/sorts/) differs from the core's.
// ZenUML shares the Mermaid Core script and only adds its plugin module URL
// when the rendered body really contains a zenuml diagram; the plugin itself
// is downloaded by runtime.js, which re-checks the same keyword rule before
// importing. The Vega-Lite trio loads in dependency order (vega → vega-lite
// → vega-embed; each script reads its predecessor's window global), so the
// three script tags are emitted strictly in that sequence.
const (
	katexVersion     = "0.18.4"
	mermaidVersion   = "11.16.1"
	zenumlVersion    = "0.2.3"
	tablesortVersion = "5.3.0"
	vegaVersion      = "6.4.0"
	vegaLiteVersion  = "6.4.3"
	vegaEmbedVersion = "7.1.0"
)

const cdnBase = "https://cdn.jsdelivr.net/npm"

var pageTemplate = template.Must(template.New("document").Parse(`<!doctype html>
<html lang="zh-CN" class="m2h-mode-{{.Mode}}" data-width="{{.Width}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
{{.Styles}}
  </style>
{{.ExtraHead}}</head>
<body class="m2h-page">
  <article class="markdown-body">
{{.Body}}  </article>
{{.ExtraBody}}</body>
</html>
`))

// buildPage wraps one rendered Markdown fragment in a complete HTML page:
// m2h's own Markdown CSS inline, the CDN runtimes the document actually uses,
// and a small inline bootstrap enhancer.
func buildPage(mode markdown.Mode, width markdown.Width, rendered markdown.Result) (string, error) {
	stylesheet, err := assets.Stylesheet(string(mode))
	if err != nil {
		return "", err
	}
	extraHead, extraBody := runtimeFragments(rendered.Body)

	var page strings.Builder
	data := struct {
		Mode      markdown.Mode
		Width     markdown.Width
		Title     string
		Styles    template.CSS
		Body      template.HTML
		ExtraHead template.HTML
		ExtraBody template.HTML
	}{
		Mode:      mode,
		Width:     width,
		Title:     rendered.Title,
		Styles:    template.CSS(stylesheet),
		Body:      template.HTML(rendered.Body),
		ExtraHead: extraHead,
		ExtraBody: extraBody,
	}
	if err := pageTemplate.Execute(&page, data); err != nil {
		return "", err
	}
	return page.String(), nil
}

// runtimeURLs collects every CDN URL an exported page may reference, so no
// CDN string is spelled out anywhere else.
type runtimeURLs struct {
	KatexCSS        string
	KatexJS         string
	KatexAutoRender string
	MermaidJS       string
	ZenUMLJS        string
	TablesortJS     string
	VegaJS          string
	VegaLiteJS      string
	VegaEmbedJS     string
}

func newRuntimeURLs() runtimeURLs {
	return runtimeURLs{
		KatexCSS:        fmt.Sprintf("%s/katex@%s/dist/katex.min.css", cdnBase, katexVersion),
		KatexJS:         fmt.Sprintf("%s/katex@%s/dist/katex.min.js", cdnBase, katexVersion),
		KatexAutoRender: fmt.Sprintf("%s/katex@%s/dist/contrib/auto-render.min.js", cdnBase, katexVersion),
		MermaidJS:       fmt.Sprintf("%s/mermaid@%s/dist/mermaid.min.js", cdnBase, mermaidVersion),
		ZenUMLJS: fmt.Sprintf(
			"%s/@mermaid-js/mermaid-zenuml@%s/dist/mermaid-zenuml.esm.min.mjs",
			cdnBase, zenumlVersion),
		TablesortJS: fmt.Sprintf("%s/tablesort@%s/dist/tablesort.min.js", cdnBase, tablesortVersion),
		VegaJS:      fmt.Sprintf("%s/vega@%s/build/vega.min.js", cdnBase, vegaVersion),
		VegaLiteJS:  fmt.Sprintf("%s/vega-lite@%s/build/vega-lite.min.js", cdnBase, vegaLiteVersion),
		VegaEmbedJS: fmt.Sprintf("%s/vega-embed@%s/build/vega-embed.min.js", cdnBase, vegaEmbedVersion),
	}
}

// runtimeFragments builds the page fragments that deliver the rich-content
// runtime: ExtraHead carries the KaTeX stylesheet link and ExtraBody carries
// the CDN scripts plus a small inline bootstrap, each loaded only when the
// rendered body uses it.
func runtimeFragments(body string) (template.HTML, template.HTML) {
	urls := newRuntimeURLs()
	var head strings.Builder
	var scripts strings.Builder
	if containsMathDelimiter(body) {
		fmt.Fprintf(&head, "  <link rel=\"stylesheet\" href=\"%s\">\n", urls.KatexCSS)
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.KatexJS)
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.KatexAutoRender)
	}
	if strings.Contains(body, "language-mermaid") {
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.MermaidJS)
		if containsZenUML(body) {
			// Not a script tag: the plugin is an ES module, dynamically
			// imported by runtime.js only when a zenuml diagram is actually
			// present. The page merely carries the pinned URL so the version
			// stays owned here; runtime.js re-checks the keyword before
			// importing, so the two gates stay cheap and aligned.
			fmt.Fprintf(&scripts, "  <script>window.m2hZenUMLModuleURL = %q;</script>\n", urls.ZenUMLJS)
		}
	}
	if containsSortableTable(body) {
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.TablesortJS)
	}
	if containsVegaLite(body) {
		// Dependency order matters: vega attaches window.vega, vega-lite
		// compiles against that runtime, and vega-embed receives both as
		// globals when it evaluates.
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.VegaJS)
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.VegaLiteJS)
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.VegaEmbedJS)
	}
	scripts.WriteString(exportBootstrapScript)
	return template.HTML(head.String()), template.HTML(scripts.String())
}

// zenumlBlockPattern anchors the official plugin detector (/^\s*zenuml/) to
// Goldmark's output: the diagram source begins right after the language class
// attribute, so only a block whose source starts with the zenuml keyword
// matches — prose or another diagram merely containing the word never does.
var zenumlBlockPattern = regexp.MustCompile(`language-mermaid">\s*zenuml`)

// containsZenUML reports whether the rendered body contains a ZenUML diagram
// and the exported page therefore needs the external-diagram plugin URL.
func containsZenUML(body string) bool {
	return zenumlBlockPattern.MatchString(body)
}

// containsSortableTable reports whether the rendered body contains a plain GFM
// table. Goldmark emits a bare "<table>" for Markdown tables, while user-authored
// raw HTML tables carry attributes such as "<table class=...>" and are
// deliberately left out of the client-side sorting enhancement.
func containsSortableTable(body string) bool {
	return strings.Contains(body, "<table>")
}

// vegaLiteBlockPattern matches Goldmark's fenced-code class attribute for the
// canonical `vega-lite` fence and its `vegalite` alias. The closing quote is
// part of the pattern: prose that merely mentions "vega-lite" (escaped inside
// another code block's text) never produces a bare class attribute, and a
// longer language like `vega-lite-extra` fails to close the attribute here.
var vegaLiteBlockPattern = regexp.MustCompile(`language-vega-lite"|language-vegalite"`)

// containsVegaLite reports whether the rendered body contains a Vega-Lite
// chart and the exported page therefore needs the runtime trio.
func containsVegaLite(body string) bool {
	return vegaLiteBlockPattern.MatchString(body)
}

// containsMathDelimiter reports whether the rendered body could contain KaTeX
// math. It matches the delimiter set handed to auto-render — every math span
// contains "$", "\(" or "\[" — so the check can never miss real math; a false
// positive (for example a literal dollar sign) only costs one extra runtime.
func containsMathDelimiter(body string) bool {
	return strings.Contains(body, "$") ||
		strings.Contains(body, `\(`) ||
		strings.Contains(body, `\[`)
}
