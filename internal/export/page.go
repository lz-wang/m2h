package export

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

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
const (
	katexVersion     = "0.18.4"
	mermaidVersion   = "11.16.1"
	tablesortVersion = "5.3.0"
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
	TablesortJS     string
}

func newRuntimeURLs() runtimeURLs {
	return runtimeURLs{
		KatexCSS:        fmt.Sprintf("%s/katex@%s/dist/katex.min.css", cdnBase, katexVersion),
		KatexJS:         fmt.Sprintf("%s/katex@%s/dist/katex.min.js", cdnBase, katexVersion),
		KatexAutoRender: fmt.Sprintf("%s/katex@%s/dist/contrib/auto-render.min.js", cdnBase, katexVersion),
		MermaidJS:       fmt.Sprintf("%s/mermaid@%s/dist/mermaid.min.js", cdnBase, mermaidVersion),
		TablesortJS:     fmt.Sprintf("%s/tablesort@%s/dist/tablesort.min.js", cdnBase, tablesortVersion),
	}
}

// runtimeFragments builds the page fragments that deliver the rich-content
// runtime: ExtraHead carries the KaTeX stylesheet link and ExtraBody carries
// the CDN scripts plus a small inline bootstrap, each loaded only when the
// rendered body uses it.
func runtimeFragments(body string) (template.HTML, template.HTML) {
	urls := newRuntimeURLs()
	needs := inspectRuntimeNeeds(body)
	var head strings.Builder
	var scripts strings.Builder
	if needs.math {
		fmt.Fprintf(&head, "  <link rel=\"stylesheet\" href=\"%s\">\n", urls.KatexCSS)
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.KatexJS)
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.KatexAutoRender)
	}
	if needs.mermaid {
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.MermaidJS)
	}
	if containsSortableTable(body) {
		fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", urls.TablesortJS)
	}
	scripts.WriteString(exportBootstrapScript)
	return template.HTML(head.String()), template.HTML(scripts.String())
}

// Match the browser's /^\s*zenuml\b/ after trimming ECMAScript whitespace.
var zenumlKeyword = regexp.MustCompile(`^zenuml\b`)

const browserWhitespace = " \t\n\v\f\r\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

type runtimeNeeds struct {
	math    bool
	mermaid bool
}

// Parse the rendered HTML once to detect eligible math text and real
// pre > code.language-mermaid nodes. Entities, comments and nested markup
// follow browser semantics; math inside code or other ignored regions does
// not trigger KaTeX. Parser-owned m2h-math containers remain eligible.
func inspectRuntimeNeeds(body string) runtimeNeeds {
	nodes, err := html.ParseFragment(strings.NewReader(body), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return runtimeNeeds{}
	}
	var needs runtimeNeeds
	var visit func(*html.Node, bool)
	visit = func(node *html.Node, mathIgnored bool) {
		if node.Type == html.ElementNode {
			switch node.DataAtom {
			case atom.Script, atom.Noscript, atom.Style, atom.Textarea, atom.Pre, atom.Code, atom.Option:
				mathIgnored = true
			}
			mathIgnored = mathIgnored || hasHTMLClass(node, "katex") || hasHTMLClass(node, "m2h-literal-dollar")
		}
		if !mathIgnored && node.Type == html.TextNode && containsMathDelimiter(node.Data) {
			needs.math = true
		}
		if !needs.mermaid && node.Type == html.ElementNode && node.DataAtom == atom.Code && node.Parent != nil && node.Parent.DataAtom == atom.Pre && hasHTMLClass(node, "language-mermaid") {
			var source strings.Builder
			var textContent func(*html.Node)
			textContent = func(child *html.Node) {
				if child.Type == html.TextNode {
					source.WriteString(child.Data)
				}
				for descendant := child.FirstChild; descendant != nil; descendant = descendant.NextSibling {
					textContent(descendant)
				}
			}
			textContent(node)
			needs.mermaid = !zenumlKeyword.MatchString(strings.TrimLeft(source.String(), browserWhitespace))
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child, mathIgnored)
		}
	}
	for _, node := range nodes {
		visit(node, false)
	}
	return needs
}

func hasHTMLClass(node *html.Node, name string) bool {
	for _, attr := range node.Attr {
		if attr.Key != "class" {
			continue
		}
		for _, class := range strings.FieldsFunc(attr.Val, func(r rune) bool { return strings.ContainsRune(" \t\n\r\f", r) }) {
			if class == name {
				return true
			}
		}
	}
	return false
}

// containsSortableTable reports whether the rendered body contains a plain GFM
// table. Goldmark emits a bare "<table>" for Markdown tables, while user-authored
// raw HTML tables carry attributes such as "<table class=...>" and are
// deliberately left out of the client-side sorting enhancement.
func containsSortableTable(body string) bool {
	return strings.Contains(body, "<table>")
}

// containsMathDelimiter reports whether eligible text could contain KaTeX
// math. It matches the delimiter set handed to auto-render — every math span
// contains "$", "\(" or "\[" — so the check can never miss real math; a false
// positive (for example a literal dollar sign) only costs one extra runtime.
func containsMathDelimiter(text string) bool {
	return strings.Contains(text, "$") ||
		strings.Contains(text, `\(`) ||
		strings.Contains(text, `\[`)
}
