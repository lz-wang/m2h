package markdown

import (
	"bytes"
	stdhtml "html"
	"strings"

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

// KindKeys is the AST node kind for ++key+key++ sequences.
var KindKeys = ast.NewNodeKind("Keys")

// keysNode holds the raw ++...++ content (without the surrounding plus pairs).
// The renderer splits it on '+' and emits one <kbd> per segment.
type keysNode struct {
	ast.BaseInline
	content []byte
}

// Dump implements ast.Node.Dump.
func (n *keysNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements ast.Node.Kind.
func (n *keysNode) Kind() ast.NodeKind { return KindKeys }

// keyAliases maps a lower-case key name to its canonical label. Unknown keys
// keep their original text and receive no key-* class.
var keyAliases = map[string]string{
	"ctrl":    "Ctrl",
	"control": "Ctrl",
	"alt":     "Alt",
	"option":  "Alt",
	"del":     "Del",
	"delete":  "Del",
	"esc":     "Esc",
	"escape":  "Esc",
	"enter":   "Enter",
	"return":  "Enter",
	"shift":   "Shift",
	"tab":     "Tab",
	"cmd":     "Cmd",
	"command": "Cmd",
	"space":   "Space",
}

// keysParser recognizes ++ctrl+alt+del++ on a single line. It only fires on a
// real pair: the opener must be exactly two pluses, the closer must exist on the
// same line, and the content must be non-empty with no whitespace. Those guards
// keep C++, "a + b" and other prose plus signs literal.
type keysParser struct{}

// newKeysParser returns the Keys inline parser.
func newKeysParser() parser.InlineParser { return &keysParser{} }

func (*keysParser) Trigger() []byte { return []byte{'+'} }

func (*keysParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 3 || line[0] != '+' || line[1] != '+' || line[2] == '+' {
		return nil
	}
	rest := line[2:]
	closeIdx := bytes.Index(rest, []byte("++"))
	if closeIdx < 0 {
		return nil
	}
	content := rest[:closeIdx]
	if !validKeysContent(content) {
		return nil
	}
	block.Advance(2 + closeIdx + 2)
	return &keysNode{content: append([]byte(nil), content...)}
}

func (*keysParser) CloseBlock(ast.Node, parser.Context) {}

// validKeysContent rejects empty content, whitespace (so prose like "C++ C++"
// cannot match) and a leading or trailing '+' that would split into empty keys.
func validKeysContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	for _, b := range content {
		switch b {
		case ' ', '\t', '\n', '\r':
			return false
		}
	}
	return !bytes.HasPrefix(content, []byte("+")) && !bytes.HasSuffix(content, []byte("+"))
}

// renderKeys emits <span class="keys">…</span> with one <kbd> per segment joined
// by <span>+</span>. Known aliases get a key-* class and canonical label;
// unknown segments keep their escaped text.
func renderKeys(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*keysNode)
	_, _ = w.WriteString(`<span class="keys">`)
	for i, segment := range bytes.Split(n.content, []byte("+")) {
		if i > 0 {
			_, _ = w.WriteString("<span>+</span>")
		}
		label := strings.ToLower(string(segment))
		if display, ok := keyAliases[label]; ok {
			_, _ = w.WriteString(`<kbd class="key-` + label + `">` + display + "</kbd>")
			continue
		}
		_, _ = w.WriteString("<kbd>" + stdhtml.EscapeString(string(segment)) + "</kbd>")
	}
	_, _ = w.WriteString("</span>")
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
		util.Prioritized(newKeysParser(), 500),
	), parser.WithBlockParsers(
		util.Prioritized(newCriticBlockParser(), 650),
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
	reg.Register(KindCriticBlock, renderCriticBlock)
	reg.Register(KindKeys, renderKeys)
}
