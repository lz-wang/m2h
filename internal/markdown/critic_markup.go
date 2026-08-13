package markdown

import (
	"bytes"
	stdhtml "html"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// criticKind enumerates the five PyMdown Critic inline variants plus the three
// block variants. Block highlight/insert/delete reuse the same values.
type criticKind uint8

const (
	criticHighlight criticKind = iota
	criticComment
	criticDelete
	criticInsert
	criticSubstitution
)

// criticInlineMarkers pairs each Critic opener with its closer and kind. The
// single-pass parser only triggers on '{', so consuming the whole span up front
// avoids delimiter conflicts with ==, ++, ~~ and the Keys parser.
var criticInlineMarkers = []struct {
	open  string
	close string
	kind  criticKind
}{
	{"{==", "==}", criticHighlight},
	{"{>>", "<<}", criticComment},
	{"{--", "--}", criticDelete},
	{"{++", "++}", criticInsert},
	{"{~~", "~~}", criticSubstitution},
}

// KindCriticInline is the AST node kind for inline Critic markup.
var KindCriticInline = ast.NewNodeKind("CriticInline")

// criticInlineNode holds one inline Critic span. Its content is stored as raw
// bytes and HTML-escaped by the renderer; inline Critic intentionally does not
// run nested Markdown (the block form does). Substitution keeps the old and new
// halves separate so it can emit a <del> followed by an <ins>.
type criticInlineNode struct {
	ast.BaseInline
	kind    criticKind
	content []byte
	old     []byte
	new     []byte
}

// Dump implements ast.Node.Dump.
func (n *criticInlineNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements ast.Node.Kind.
func (n *criticInlineNode) Kind() ast.NodeKind { return KindCriticInline }

// criticInlineParser recognizes {==...==}, {>>...<<}, {--...--}, {++...++} and
// {~~old~>new~~} on a single line. When an opener has no matching closer on the
// same line it returns nil so the text stays literal instead of being swallowed.
type criticInlineParser struct{}

// newCriticInlineParser returns the inline Critic parser.
func newCriticInlineParser() parser.InlineParser { return &criticInlineParser{} }

func (*criticInlineParser) Trigger() []byte { return []byte{'{'} }

func (*criticInlineParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	for _, marker := range criticInlineMarkers {
		open := []byte(marker.open)
		if !bytes.HasPrefix(line, open) {
			continue
		}
		rest := line[len(open):]
		close := []byte(marker.close)
		closeIdx := bytes.Index(rest, close)
		if closeIdx < 0 {
			return nil
		}
		content := rest[:closeIdx]
		block.Advance(len(open) + closeIdx + len(close))
		return newCriticInlineNode(marker.kind, content)
	}
	return nil
}

func (*criticInlineParser) CloseBlock(ast.Node, parser.Context) {}

// newCriticInlineNode builds a criticInlineNode, splitting substitution content
// on the '~>' separator. A substitution missing '~>' is treated as malformed
// and collapsed to a single delete half so no bytes are lost.
func newCriticInlineNode(kind criticKind, content []byte) ast.Node {
	node := &criticInlineNode{kind: kind}
	if kind == criticSubstitution {
		if old, sep, ok := bytes.Cut(content, []byte("~>")); ok {
			node.old = append([]byte(nil), old...)
			node.new = append([]byte(nil), sep...)
		} else {
			node.kind = criticDelete
			node.content = append([]byte(nil), content...)
		}
		return node
	}
	node.content = append([]byte(nil), content...)
	return node
}

// renderCriticInline emits the Critic span. It writes the whole element on
// entering; the node carries no children.
func renderCriticInline(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*criticInlineNode)
	switch n.kind {
	case criticHighlight:
		writeCritic(w, "mark", "m2h-critic", n.content)
	case criticComment:
		writeCriticSpan(w, n.content)
	case criticDelete:
		writeCritic(w, "del", "m2h-critic m2h-critic-delete", n.content)
	case criticInsert:
		writeCritic(w, "ins", "m2h-critic m2h-critic-insert", n.content)
	case criticSubstitution:
		writeCritic(w, "del", "m2h-critic m2h-critic-delete", n.old)
		writeCritic(w, "ins", "m2h-critic m2h-critic-insert", n.new)
	}
	return ast.WalkContinue, nil
}

// writeCritic emits <tag class="cls">escaped</tag>.
func writeCritic(w util.BufWriter, tag, cls string, content []byte) {
	_, _ = w.WriteString("<" + tag + ` class="` + cls + `">`)
	_, _ = w.WriteString(stdhtml.EscapeString(string(content)))
	_, _ = w.WriteString("</" + tag + ">")
}

// writeCriticSpan emits the comment variant, whose tag is always <span>.
func writeCriticSpan(w util.BufWriter, content []byte) {
	_, _ = w.WriteString(`<span class="m2h-critic m2h-critic-comment">`)
	_, _ = w.WriteString(stdhtml.EscapeString(string(content)))
	_, _ = w.WriteString("</span>")
}

// criticBlockMarkers lists the three block-level Critic variants. A block opens
// when a line is exactly the opener (ignoring surrounding whitespace) and closes
// on a standalone closer line; the content between is parsed as normal Markdown.
var criticBlockMarkers = []struct {
	open, close string
	kind        criticKind
}{
	{"{==", "==}", criticHighlight},
	{"{++", "++}", criticInsert},
	{"{--", "--}", criticDelete},
}

// KindCriticBlock is the AST node kind for block-level Critic markup.
var KindCriticBlock = ast.NewNodeKind("CriticBlock")

// criticBlockNode is a container whose children are the Markdown blocks written
// between the opener and closer lines.
type criticBlockNode struct {
	ast.BaseBlock
	kind criticKind
}

// Dump implements ast.Node.Dump.
func (n *criticBlockNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements ast.Node.Kind.
func (n *criticBlockNode) Kind() ast.NodeKind { return KindCriticBlock }

// matchCriticBlockOpener reports whether line is a standalone Critic block
// opener and returns the matching kind.
func matchCriticBlockOpener(line []byte) (criticKind, bool) {
	trimmed := bytes.TrimSpace(line)
	for _, marker := range criticBlockMarkers {
		if string(trimmed) == marker.open {
			return marker.kind, true
		}
	}
	return 0, false
}

// criticBlockClose returns the standalone closer for a block kind.
func criticBlockClose(kind criticKind) string {
	for _, marker := range criticBlockMarkers {
		if marker.kind == kind {
			return marker.close
		}
	}
	return ""
}

// criticBlockClass returns the highlight/insert/delete modifier class.
func criticBlockClass(kind criticKind) string {
	switch kind {
	case criticHighlight:
		return "m2h-critic-block-highlight"
	case criticInsert:
		return "m2h-critic-block-insert"
	case criticDelete:
		return "m2h-critic-block-delete"
	}
	return ""
}

// criticBlockParser opens a fence-like container on a standalone {==, {++ or {--
// line and keeps it open until the matching ==}, ++} or --} line. Returning
// Continue|HasChildren for every other line lets Goldmark parse the inner
// Markdown (paragraphs, lists, emphasis) as real children instead of flat text.
type criticBlockParser struct{}

// newCriticBlockParser returns the block Critic parser.
func newCriticBlockParser() parser.BlockParser { return &criticBlockParser{} }

func (*criticBlockParser) Trigger() []byte { return []byte{'{'} }

func (*criticBlockParser) Open(_ ast.Node, reader text.Reader, _ parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	kind, ok := matchCriticBlockOpener(line)
	if !ok {
		return nil, parser.NoChildren
	}
	reader.AdvanceToEOL()
	return &criticBlockNode{kind: kind}, parser.HasChildren
}

func (*criticBlockParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if string(bytes.TrimSpace(line)) == criticBlockClose(node.(*criticBlockNode).kind) {
		reader.AdvanceToEOL()
		return parser.Close
	}
	return parser.Continue | parser.HasChildren
}

func (*criticBlockParser) Close(ast.Node, text.Reader, parser.Context) {}

func (*criticBlockParser) CanInterruptParagraph() bool { return true }

func (*criticBlockParser) CanAcceptIndentedLine() bool { return false }

// renderCriticBlock wraps the parsed children in a themed <div>.
func renderCriticBlock(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*criticBlockNode)
	if entering {
		_, _ = w.WriteString(`<div class="m2h-critic-block ` + criticBlockClass(n.kind) + `">` + "\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("</div>\n")
	return ast.WalkContinue, nil
}
