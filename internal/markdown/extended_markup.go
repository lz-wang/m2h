package markdown

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// markupKind distinguishes the inline delimiter-based emphasis that
// ExtendedMarkupExtension introduces: ==...== marks text and ^^...^^ inserts
// it. They share one AST node kind and are told apart by this field.
type markupKind uint8

const (
	markupMark markupKind = iota
	markupInsert
)

// KindMarkup is the shared AST node kind for ==mark== and ^^insert^^ nodes.
var KindMarkup = ast.NewNodeKind("Markup")

// markupNode is the inline container produced by the markup delimiter
// processor. Its children are parsed by Goldmark like any emphasis, so
// nested bold, links and other inline markup work inside ==...== / ^^...^^.
type markupNode struct {
	ast.BaseInline
	Markup markupKind
}

// Dump implements ast.Node.Dump.
func (n *markupNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements ast.Node.Kind.
func (n *markupNode) Kind() ast.NodeKind { return KindMarkup }

// markupDelimiterProcessor implements parser.DelimiterProcessor for a single
// trigger byte. Two instances exist: '=' for mark and '^' for insert.
type markupDelimiterProcessor struct {
	char byte
	kind markupKind
}

func (p *markupDelimiterProcessor) IsDelimiter(b byte) bool { return b == p.char }

func (p *markupDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (p *markupDelimiterProcessor) OnMatch(_ int) ast.Node {
	return &markupNode{Markup: p.kind}
}

// markupParser scans a run of its trigger byte and, when exactly two long,
// pushes a delimiter so Goldmark can match openers and closers the same way it
// matches ~~strikethrough~~. Runs of one or more than two are left untouched so
// `key=value` and `===foo===` stay literal.
type markupParser struct {
	char      byte
	processor parser.DelimiterProcessor
}

// newMarkupParser returns an InlineParser for the given trigger byte and
// resulting markup kind.
func newMarkupParser(char byte, kind markupKind) parser.InlineParser {
	return &markupParser{char: char, processor: &markupDelimiterProcessor{char: char, kind: kind}}
}

func (p *markupParser) Trigger() []byte { return []byte{p.char} }

func (p *markupParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()

	delimiter := parser.ScanDelimiter(line, before, 2, p.processor)
	// Require exactly two delimiter characters and refuse a leading run of
	// the same byte. The before check mirrors Goldmark's strikethrough parser:
	// after rejecting ===, Goldmark consumes one byte and retries, so without
	// it the inner == of === would match and split ===foo=== unpredictably.
	if delimiter == nil || delimiter.OriginalLength != 2 || before == rune(p.char) {
		return nil
	}

	delimiter.Segment = segment.WithStop(segment.Start + 2)
	block.Advance(2)
	pc.PushDelimiter(delimiter)
	return delimiter
}

func (*markupParser) CloseBlock(ast.Node, parser.Context) {}

// renderMarkup emits <mark> for ==...== and <ins> for ^^^...^^^, letting
// Goldmark render the wrapped children between the open and close tags.
func renderMarkup(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		n := node.(*markupNode)
		switch n.Markup {
		case markupMark:
			_, _ = w.WriteString("</mark>")
		case markupInsert:
			_, _ = w.WriteString("</ins>")
		}
		return ast.WalkContinue, nil
	}
	n := node.(*markupNode)
	switch n.Markup {
	case markupMark:
		_, _ = w.WriteString("<mark>")
	case markupInsert:
		_, _ = w.WriteString("<ins>")
	}
	return ast.WalkContinue, nil
}

// ExtendedMarkupExtension adds m2h's extra inline and block markup on top of
// GFM: ==mark==, ^^insert^^, PyMdown-style Critic markup and ++keys++. Convert
// and Web preview both pick it up automatically because they share newEngine.
var ExtendedMarkupExtension goldmark.Extender = &extendedMarkupExtension{}

type extendedMarkupExtension struct{}

func (e *extendedMarkupExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(newMarkupParser('=', markupMark), 500),
		util.Prioritized(newMarkupParser('^', markupInsert), 500),
		util.Prioritized(newCriticInlineParser(), 200),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&extendedMarkupHTMLRenderer{}, 500),
	))
}

// extendedMarkupHTMLRenderer renders every node kind introduced by
// ExtendedMarkupExtension. Capabilities added in later phases (Critic, Keys)
// register their render funcs here too.
type extendedMarkupHTMLRenderer struct{}

func (r *extendedMarkupHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindMarkup, renderMarkup)
	reg.Register(KindCriticInline, renderCriticInline)
}
