package markdown

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// AlertType enumerates the five GitHub alert variants.
type AlertType string

const (
	AlertNote      AlertType = "note"
	AlertTip       AlertType = "tip"
	AlertImportant AlertType = "important"
	AlertWarning   AlertType = "warning"
	AlertCaution   AlertType = "caution"
)

var alertKind = ast.NewNodeKind("Alert")

// Alert is a block node produced from a GitHub alert blockquote, e.g.
// "> [!NOTE]\n> ...". Blockquotes whose first line is not a known alert marker
// are left as ordinary blockquotes. The Variant field is named to avoid
// clashing with the ast.Node.Type method.
type Alert struct {
	ast.BaseBlock
	Variant AlertType
}

// Kind implements ast.Node.
func (a *Alert) Kind() ast.NodeKind { return alertKind }

// Dump implements ast.Node for debugging.
func (a *Alert) Dump(source []byte, level int) {
	ast.DumpHelper(a, source, level, map[string]string{"variant": string(a.Variant)}, nil)
}

// AlertExtension is a goldmark.Extender that recognises GitHub alert syntax in
// blockquotes and renders them with GitHub-compatible markup. It only matches
// the exact markers GitHub supports; everything else stays a blockquote.
var AlertExtension goldmark.Extender = &alertExtension{}

type alertExtension struct{}

func (e *alertExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&alertTransformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&alertHTMLRenderer{}, 100),
	))
}

type alertTransformer struct{}

func (t *alertTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	type matched struct {
		node *ast.Blockquote
		typ  AlertType
	}
	var matches []matched
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		bq, ok := n.(*ast.Blockquote)
		if !ok {
			return ast.WalkContinue, nil
		}
		typ, ok := detectAlert(bq, source)
		if !ok {
			return ast.WalkContinue, nil
		}
		matches = append(matches, matched{bq, typ})
		return ast.WalkSkipChildren, nil
	})
	for _, m := range matches {
		convertAlert(m.node, m.typ)
	}
}

func detectAlert(bq *ast.Blockquote, source []byte) (AlertType, bool) {
	first := bq.FirstChild()
	if first == nil {
		return "", false
	}
	para, ok := first.(*ast.Paragraph)
	if !ok {
		return "", false
	}
	lines := para.Lines()
	if lines.Len() == 0 {
		return "", false
	}
	firstLine := lines.At(0)
	return parseAlertMarker(firstLine.Value(source))
}

// parseAlertMarker reports whether line is exactly
// "[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)]" (case-insensitive). The marker must
// occupy its own line as the first line of the blockquote, matching GitHub.
func parseAlertMarker(line []byte) (AlertType, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) < 5 || trimmed[0] != '[' || trimmed[1] != '!' || trimmed[len(trimmed)-1] != ']' {
		return "", false
	}
	switch string(bytes.ToLower(trimmed[2 : len(trimmed)-1])) {
	case "note":
		return AlertNote, true
	case "tip":
		return AlertTip, true
	case "important":
		return AlertImportant, true
	case "warning":
		return AlertWarning, true
	case "caution":
		return AlertCaution, true
	}
	return "", false
}

// convertAlert rewrites a matched blockquote in place: it drops the marker
// line, moves the remaining children into a new Alert node, and replaces the
// blockquote with it.
func convertAlert(bq *ast.Blockquote, typ AlertType) {
	para := bq.FirstChild().(*ast.Paragraph)
	lines := para.Lines()
	if lines.Len() <= 1 {
		lines.Clear()
	} else {
		lines.SetSliced(1, lines.Len())
	}
	if para.Lines().Len() == 0 && !para.HasChildren() {
		bq.RemoveChild(bq, para)
	}

	alert := &Alert{Variant: typ}
	for child := bq.FirstChild(); child != nil; {
		next := child.NextSibling()
		bq.RemoveChild(bq, child)
		alert.AppendChild(alert, child)
		child = next
	}
	if parent := bq.Parent(); parent != nil {
		parent.ReplaceChild(parent, bq, alert)
	}
}

type alertHTMLRenderer struct{}

func (r *alertHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(alertKind, r.renderAlert)
}

func (r *alertHTMLRenderer) renderAlert(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</div>\n")
		return ast.WalkContinue, nil
	}
	alert := n.(*Alert)
	_, _ = fmt.Fprintf(w, "<div class=\"markdown-alert markdown-alert-%s\">\n", alert.Variant)
	_, _ = w.WriteString("<p class=\"markdown-alert-title\">")
	_, _ = w.Write(alertIconSVG(alert.Variant))
	_, _ = w.WriteString(alertTitleText(alert.Variant))
	_, _ = w.WriteString("</p>\n")
	return ast.WalkContinue, nil
}

func alertTitleText(typ AlertType) string {
	switch typ {
	case AlertNote:
		return "Note"
	case AlertTip:
		return "Tip"
	case AlertImportant:
		return "Important"
	case AlertWarning:
		return "Warning"
	case AlertCaution:
		return "Caution"
	}
	return ""
}

const (
	alertSVGOpen  = `<svg class="octicon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true"><path d="`
	alertSVGClose = `"/></svg>`
)

// alertIconPaths holds the GitHub octicon path data used for each alert kind,
// so the renderer emits trusted inline SVG without CDN requests.
var alertIconPaths = map[AlertType]string{
	AlertNote:      "M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z",
	AlertTip:       "M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z",
	AlertImportant: "M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z",
	AlertWarning:   "M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z",
	AlertCaution:   "M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z",
}

func alertIconSVG(typ AlertType) []byte {
	path, ok := alertIconPaths[typ]
	if !ok {
		return nil
	}
	return []byte(alertSVGOpen + path + alertSVGClose)
}
