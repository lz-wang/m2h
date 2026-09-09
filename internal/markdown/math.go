package markdown

import (
	"bytes"
	"html"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var kindMathBlock = ast.NewNodeKind("MathBlock")

var kindMathInline = ast.NewNodeKind("MathInline")

// Math is opaque to Markdown. Keeping source segments on the nodes gives both
// rendering and inspection the same literal boundary, including TeX escapes.
type mathBlock struct{ ast.BaseBlock }

func (*mathBlock) Kind() ast.NodeKind { return kindMathBlock }

func (n *mathBlock) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

func (*mathBlock) IsRaw() bool { return true }

type mathInline struct {
	ast.BaseInline
	segment text.Segment
}

func (*mathInline) Kind() ast.NodeKind { return kindMathInline }

func (n *mathInline) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

func (n *mathInline) Text(source []byte) []byte { return n.segment.Value(source) }

type mathBlockParser struct{}

func (*mathBlockParser) Trigger() []byte { return []byte{'$'} }

func (*mathBlockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || !bytes.Equal(bytes.TrimSpace(line[pos:]), []byte("$$")) {
		return nil, parser.NoChildren
	}
	node := &mathBlock{}
	node.Lines().Append(segment.WithStart(segment.Start + pos))
	return node, parser.NoChildren
}

func (*mathBlockParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	node.Lines().Append(segment)
	reader.AdvanceToEOL()
	if bytes.Equal(bytes.TrimSpace(line), []byte("$$")) {
		return parser.Close
	}
	return parser.Continue | parser.NoChildren
}

func (*mathBlockParser) Close(ast.Node, text.Reader, parser.Context) {}

func (*mathBlockParser) CanInterruptParagraph() bool { return true }

func (*mathBlockParser) CanAcceptIndentedLine() bool { return false }

type mathInlineParser struct{}

func (*mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (*mathInlineParser) Parse(_ ast.Node, reader text.Reader, _ parser.Context) ast.Node {
	line, segment := reader.PeekLine()
	if reader.PrecendingCharacter() == '$' {
		return nil
	}
	width := 1
	if len(line) > 1 && line[1] == '$' {
		width = 2
	}
	if len(line) <= width || line[width] == '$' {
		return nil
	}
	first, _ := utf8.DecodeRune(line[width:])
	if width == 1 && unicode.IsSpace(first) {
		return nil
	}
	for i := width; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] != '$' {
			continue
		}
		end := i
		for end < len(line) && line[end] == '$' {
			end++
		}
		if end-i != width {
			i = end - 1
			continue
		}
		previous, _ := utf8.DecodeLastRune(line[:i])
		if width == 1 && (unicode.IsSpace(previous) || (end < len(line) && line[end] >= '0' && line[end] <= '9')) {
			continue
		}
		reader.Advance(end)
		return &mathInline{segment: segment.WithStop(segment.Start + end)}
	}
	return nil
}

func (*mathInlineParser) CloseBlock(ast.Node, parser.Context) {}

type mathExtension struct{}

func (*mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(util.Prioritized(&mathBlockParser{}, 650)),
		parser.WithInlineParsers(util.Prioritized(&mathInlineParser{}, 150)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(&mathRenderer{}, 500)))
}

type mathRenderer struct{}

func (*mathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMathBlock, renderMath)
	reg.Register(kindMathInline, renderMath)
}

func renderMath(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if inline, ok := node.(*mathInline); ok {
		_, _ = w.WriteString(`<span class="m2h-math">`)
		_, _ = w.WriteString(html.EscapeString(string(inline.Text(source))))
		_, _ = w.WriteString("</span>")
	} else {
		_, _ = w.WriteString("<div class=\"m2h-math\">")
		for i := 0; i < node.Lines().Len(); i++ {
			segment := node.Lines().At(i)
			_, _ = w.WriteString(html.EscapeString(string(segment.Value(source))))
		}
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkSkipChildren, nil
}
