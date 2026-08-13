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
