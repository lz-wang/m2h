package convert

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/lz-wang/m2h/internal/assets"
	"github.com/lz-wang/m2h/internal/markdown"
)

// Rich-content runtime versions are pinned to the same releases vendored under
// internal/assets/rich for the WebUI, so exported pages and the WebUI render
// identical rich content.
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
	TablesortJS     []string
}

func newRuntimeURLs() runtimeURLs {
	tablesort := make([]string, 0, len(tablesortScripts))
	for _, name := range tablesortScripts {
		tablesort = append(tablesort, fmt.Sprintf("%s/tablesort@%s/dist/%s", cdnBase, tablesortVersion, name))
	}
	return runtimeURLs{
		KatexCSS:        fmt.Sprintf("%s/katex@%s/dist/katex.min.css", cdnBase, katexVersion),
		KatexJS:         fmt.Sprintf("%s/katex@%s/dist/katex.min.js", cdnBase, katexVersion),
		KatexAutoRender: fmt.Sprintf("%s/katex@%s/dist/contrib/auto-render.min.js", cdnBase, katexVersion),
		MermaidJS:       fmt.Sprintf("%s/mermaid@%s/dist/mermaid.min.js", cdnBase, mermaidVersion),
		TablesortJS:     tablesort,
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
	}
	if containsSortableTable(body) {
		for _, url := range urls.TablesortJS {
			fmt.Fprintf(&scripts, "  <script src=\"%s\"></script>\n", url)
		}
	}
	scripts.WriteString(exportBootstrapScript)
	return template.HTML(head.String()), template.HTML(scripts.String())
}

// tablesortScripts lists the client-side table sorter and its comparator
// extensions in load order: the core defines window.Tablesort, the extensions
// only register comparators through Tablesort.extend, and the bootstrap that
// instantiates tables must run after all of them.
var tablesortScripts = []string{
	"tablesort.min.js",
	"tablesort.date.js",
	"tablesort.dotsep.js",
	"tablesort.filesize.js",
	"tablesort.monthname.js",
	"tablesort.number.js",
}

// containsSortableTable reports whether the rendered body contains a plain GFM
// table. Goldmark emits a bare "<table>" for Markdown tables, while user-authored
// raw HTML tables carry attributes such as "<table class=...>" and are
// deliberately left out of the client-side sorting enhancement.
func containsSortableTable(body string) bool {
	return strings.Contains(body, "<table>")
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

// exportBootstrapScript is the minimal enhancer embedded in every exported
// page: it renders Mermaid diagrams, KaTeX math, and sortable tables and
// nothing else — no lightbox, line numbers, code collapse, heading spy, share,
// or theme switching. Those belong to the WebUI.
const exportBootstrapScript = `  <script>
(function () {
  "use strict";
  var DELIMITERS = [
    { left: "$$", right: "$$", display: true },
    { left: "\\[", right: "\\]", display: true },
    { left: "\\(", right: "\\)", display: false },
    { left: "$", right: "$", display: false }
  ];
  function enhance() {
    var root = document.querySelector(".markdown-body");
    if (root === null) {
      return;
    }
    var nodes = [];
    if (typeof mermaid !== "undefined" && typeof mermaid.initialize === "function") {
      var rootClasses = document.documentElement.classList;
      var dark = rootClasses.contains("m2h-mode-dark") ||
        (!rootClasses.contains("m2h-mode-light") &&
          typeof window.matchMedia === "function" &&
          window.matchMedia("(prefers-color-scheme: dark)").matches);
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default" });
      root.querySelectorAll("pre > code.language-mermaid").forEach(function (code) {
        var pre = code.parentElement;
        if (!(pre instanceof HTMLPreElement)) {
          return;
        }
        var container = document.createElement("div");
        container.className = "mermaid";
        container.textContent = code.textContent || "";
        pre.replaceWith(container);
        nodes.push(container);
      });
    }
    var finish = function () {
      if (typeof renderMathInElement === "function") {
        renderMathInElement(root, { delimiters: DELIMITERS, throwOnError: false });
      }
      if (typeof Tablesort === "function") {
        root.querySelectorAll('table:not([class]):not([data-m2h-sortable="true"])').forEach(function (table) {
          if (!table.tHead || table.tBodies.length === 0 || table.tBodies[0].rows.length <= 1) {
            return;
          }
          table.setAttribute("data-m2h-sortable", "true");
          new Tablesort(table);
        });
      }
    };
    var pending = typeof mermaid !== "undefined" && typeof mermaid.run === "function" && nodes.length > 0
      ? mermaid.run({ nodes: nodes, suppressErrors: true })
      : null;
    if (pending && typeof pending.then === "function") {
      pending.then(finish);
    } else {
      finish();
    }
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
</script>
`
