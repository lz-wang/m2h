package markdown

import (
	"bytes"
	"fmt"
	"html"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var kindTabSet = ast.NewNodeKind("TabSet")
var kindTabItem = ast.NewNodeKind("TabItem")

// TabSet and TabItem keep children in the original parse and source coordinate
// system. IDs are document-local ordinals, independent of untrusted labels.
type TabSet struct {
	ast.BaseBlock
	Number   int
	Selected int
}

func (*TabSet) Kind() ast.NodeKind              { return kindTabSet }
func (n *TabSet) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type TabItem struct {
	ast.BaseBlock
	Title   string
	Default bool
	NewSet  bool
	ID      string
	indent  int
}

func (*TabItem) Kind() ast.NodeKind              { return kindTabItem }
func (n *TabItem) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type tabbedParser struct{}

func (*tabbedParser) Trigger() []byte             { return []byte{'='} }
func (*tabbedParser) CanInterruptParagraph() bool { return true }
func (*tabbedParser) CanAcceptIndentedLine() bool { return false }
func (*tabbedParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	if parent.Kind() != ast.KindDocument {
		return nil, parser.NoChildren
	}
	line, segment := reader.PeekLine()
	indent, pos := util.IndentWidth(line, reader.LineOffset())
	if indent > 3 || !bytes.HasPrefix(line[pos:], []byte("===")) {
		return nil, parser.NoChildren
	}
	marker := bytes.TrimSpace(line[pos+3:])
	item := &TabItem{indent: indent + 4}
	if len(marker) > 0 && (marker[0] == '!' || marker[0] == '+') {
		item.NewSet = marker[0] == '!'
		item.Default = marker[0] == '+'
		marker = marker[1:]
	}
	// Whitespace before the quoted, non-empty plain-text label is mandatory.
	raw := line[pos+3:]
	if len(raw) > 0 && (raw[0] == '!' || raw[0] == '+') {
		raw = raw[1:]
	}
	if len(raw) == 0 || (raw[0] != ' ' && raw[0] != '\t') {
		return nil, parser.NoChildren
	}
	marker = bytes.TrimSpace(marker)
	if len(marker) < 3 || marker[0] != '"' || marker[len(marker)-1] != '"' || bytes.ContainsRune(marker[1:len(marker)-1], '"') {
		return nil, parser.NoChildren
	}
	// Reject a marker followed by unindented prose; an empty item at EOF or
	// before another marker is valid. Lookahead never modifies source or reader.
	rest := reader.Source()[segment.Stop:]
	for len(rest) > 0 {
		end := bytes.IndexByte(rest, '\n')
		if end < 0 {
			end = len(rest) - 1
		}
		next := rest[:end+1]
		rest = rest[end+1:]
		if util.IsBlank(next) {
			continue
		}
		width, pos := util.IndentWidth(next, 0)
		if width < item.indent && !(width <= 3 && bytes.HasPrefix(next[pos:], []byte("==="))) {
			return nil, parser.NoChildren
		}
		break
	}
	item.Title = string(marker[1 : len(marker)-1])
	reader.AdvanceToEOL()
	return item, parser.HasChildren
}
func (*tabbedParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		reader.AdvanceToEOL()
		return parser.Continue | parser.HasChildren
	}
	item := node.(*TabItem)
	pos, padding := util.IndentPosition(line, reader.LineOffset(), item.indent)
	if pos < 0 {
		return parser.Close
	}
	reader.AdvanceAndSetPadding(pos, padding)
	return parser.Continue | parser.HasChildren
}
func (*tabbedParser) Close(ast.Node, text.Reader, parser.Context) {}

type tabbedTransformer struct{}

func (*tabbedTransformer) Transform(document *ast.Document, _ text.Reader, _ parser.Context) {
	var set *TabSet
	number := 0
	for node := document.FirstChild(); node != nil; {
		next := node.NextSibling()
		item, ok := node.(*TabItem)
		if !ok {
			set = nil
			node = next
			continue
		}
		if set == nil || item.NewSet {
			number++
			set = &TabSet{Number: number, Selected: -1}
			document.InsertBefore(document, item, set)
		}
		index := set.ChildCount()
		item.ID = fmt.Sprintf("m2h-tab-%d-%d", number, index+1)
		if item.Default && set.Selected < 0 {
			set.Selected = index
		}
		set.AppendChild(set, item)
		node = next
	}
	for node := document.FirstChild(); node != nil; node = node.NextSibling() {
		if set, ok := node.(*TabSet); ok && set.Selected < 0 {
			set.Selected = 0
		}
	}
}

type tabbedExtension struct{}

func (*tabbedExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(&tabbedRenderer{}, 500)))
	m.Parser().AddOptions(parser.WithBlockParsers(util.Prioritized(&tabbedParser{}, 650), util.Prioritized(&tabbedBoundaryParser{BlockParser: parser.NewParagraphParser()}, 999)), parser.WithASTTransformers(util.Prioritized(&tabbedTransformer{}, 900)))
}

// Groups above the CSS limit are readable stacked sections, never inaccessible
// unchecked panels. Rendering and projection share the transformer's IDs.
const maxInteractiveTabs = 32

type tabbedRenderer struct{}

func (*tabbedRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindTabSet, renderTabSet)
	reg.Register(kindTabItem, renderTabItem)
}
func renderTabSet(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	set := node.(*TabSet)
	if !entering {
		_, _ = w.WriteString("</div></div>\n")
		return ast.WalkContinue, nil
	}
	if set.ChildCount() > maxInteractiveTabs {
		_, _ = w.WriteString(`<div class="m2h-tabbed-set m2h-tabbed-stacked"><div class="m2h-tabbed-content">`)
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<div class="m2h-tabbed-set" role="group" aria-label="标签内容">`)
	index := 0
	for child := set.FirstChild(); child != nil; child = child.NextSibling() {
		item := child.(*TabItem)
		checked := ""
		if index == set.Selected {
			checked = " checked"
		}
		_, _ = fmt.Fprintf(w, `<input class="m2h-tabbed-radio" type="radio" name="m2h-tab-set-%d" id="%s"%s>`, set.Number, item.ID, checked)
		index++
	}
	_, _ = w.WriteString(`<div class="m2h-tabbed-labels">`)
	for child := set.FirstChild(); child != nil; child = child.NextSibling() {
		item := child.(*TabItem)
		_, _ = fmt.Fprintf(w, `<label for="%s">%s</label>`, item.ID, html.EscapeString(item.Title))
	}
	_, _ = w.WriteString(`</div><div class="m2h-tabbed-content">`)
	return ast.WalkContinue, nil
}
func renderTabItem(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	item := node.(*TabItem)
	if entering {
		_, _ = fmt.Fprintf(w, `<section class="m2h-tabbed-panel" data-m2h-tab-id="%s"><div class="m2h-tabbed-panel-title">%s</div>`, item.ID, html.EscapeString(item.Title))
	} else {
		_, _ = w.WriteString("</section>\n")
	}
	return ast.WalkContinue, nil
}

// Goldmark normally allows lazy paragraph continuations in containers. Tabbed
// requires four columns, so a root-level prose line opens its own paragraph
// before the default parser can lazily append it to the previous TabItem.
type tabbedBoundaryParser struct{ parser.BlockParser }

func (*tabbedBoundaryParser) CanInterruptParagraph() bool { return true }
func (p *tabbedBoundaryParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	if parent.Kind() != ast.KindDocument {
		return nil, parser.NoChildren
	}
	for _, block := range pc.OpenedBlocks() {
		if block.Node.Kind() == kindTabItem {
			return p.BlockParser.Open(parent, reader, pc)
		}
	}
	return nil, parser.NoChildren
}
