package markdown

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"sync"

	"github.com/lz-wang/m2h/internal/assets"
)

// runtimeFragments builds the page fragments that deliver the rich-content
// runtime: ExtraHead carries the stylesheet reference (or inline <style> for
// KaTeX plus its fonts) and ExtraBody carries the script tags. Previews render
// with neither — the React WebUI loads the runtime itself.
func runtimeFragments(options RenderOptions, body string) (template.HTML, template.HTML, error) {
	switch options.Assets {
	case AssetInline:
		return inlineRuntimeFragments(body)
	case AssetShared:
		if options.AssetBase == "" {
			return "", "", nil
		}
		var head strings.Builder
		fmt.Fprintf(&head, "  <link rel=\"stylesheet\" href=\"%skatex.min.css\">\n", options.AssetBase)
		var scripts strings.Builder
		for _, name := range []string{"katex.min.js", "auto-render.min.js", "mermaid.min.js"} {
			fmt.Fprintf(&scripts, "  <script src=\"%s%s\"></script>\n", options.AssetBase, name)
		}
		if containsSortableTable(body) {
			for _, name := range tablesortScripts {
				fmt.Fprintf(&scripts, "  <script src=\"%s%s\"></script>\n", options.AssetBase, name)
			}
		}
		fmt.Fprintf(&scripts, "  <script src=\"%srich-content.js\"></script>\n", options.AssetBase)
		return template.HTML(head.String()), template.HTML(scripts.String()), nil
	default:
		return "", "", &OptionError{Name: "assets", Value: options.Assets.String()}
	}
}

// inlineRuntimeFragments embeds only the runtime pieces the document uses:
// KaTeX (stylesheet, core, auto-render) when math delimiters are present,
// Mermaid when a fenced diagram exists, and always the rich-content enhancer,
// which also provides code copy buttons for plain documents.
func inlineRuntimeFragments(body string) (template.HTML, template.HTML, error) {
	var head strings.Builder
	var scripts strings.Builder
	if containsMathDelimiter(body) {
		stylesheet, err := assets.InlineKatexCSS()
		if err != nil {
			return "", "", err
		}
		head.WriteString("  <style>\n")
		head.WriteString(escapeClosingTag(stylesheet, "style"))
		head.WriteString("\n  </style>\n")
		for _, name := range []string{"katex.min.js", "auto-render.min.js"} {
			if err := writeInlineScript(&scripts, name); err != nil {
				return "", "", err
			}
		}
	}
	if strings.Contains(body, "language-mermaid") {
		if err := writeInlineScript(&scripts, "mermaid.min.js"); err != nil {
			return "", "", err
		}
	}
	if containsSortableTable(body) {
		for _, name := range tablesortScripts {
			if err := writeInlineScript(&scripts, name); err != nil {
				return "", "", err
			}
		}
	}
	if err := writeInlineScript(&scripts, "rich-content.js"); err != nil {
		return "", "", err
	}
	return template.HTML(head.String()), template.HTML(scripts.String()), nil
}

// tablesortScripts lists the client-side table sorter and its comparator
// extensions in load order: the core defines window.Tablesort, the extensions
// only register comparators through Tablesort.extend, and the rich-content
// enhancer that instantiates tables must run after all of them.
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

func writeInlineScript(scripts *strings.Builder, name string) error {
	contents, err := inlineScript(name)
	if err != nil {
		return err
	}
	scripts.WriteString("  <script>\n")
	scripts.WriteString(contents)
	scripts.WriteString("\n  </script>\n")
	return nil
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

// closingTagPattern builds the case-insensitive matcher for a closing tag
// sequence that would otherwise terminate the surrounding element early.
func closingTagPattern(tag string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)</` + tag)
}

var (
	inlineScriptCache sync.Map // script name -> escaped contents
	escapeScriptTag   = closingTagPattern("script")
)

// inlineScript returns one embedded runtime script with its closing script
// tags escaped, cached because escaping the multi-megabyte Mermaid bundle per
// document would dominate conversion time.
func inlineScript(name string) (string, error) {
	if cached, ok := inlineScriptCache.Load(name); ok {
		return cached.(string), nil
	}
	contents, err := assets.RichAssetText(name)
	if err != nil {
		return "", err
	}
	escaped := escapeScriptTag.ReplaceAllString(contents, `<\/`+"script")
	inlineScriptCache.Store(name, escaped)
	return escaped, nil
}

// escapeClosingTag prevents a raw closing tag inside stylesheet text from
// ending the host <style> element prematurely.
func escapeClosingTag(contents, tag string) string {
	return closingTagPattern(tag).ReplaceAllString(contents, `<\/`+tag)
}
