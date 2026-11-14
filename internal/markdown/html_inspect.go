package markdown

import (
	"bytes"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// HTMLBlockMissingBlank is one Type 6/7 HTML block whose top-level element
// has completed while later non-blank text still sits inside the same raw
// block: without a separating blank line the text renders as raw HTML
// content, not Markdown. Position is the affected text's line, column 1 —
// where the missing blank line belongs.
type HTMLBlockMissingBlank struct {
	Position Position
}

// voidHTMLElements are the HTML elements that cannot carry content, so a
// single tag completes them. x/net/html keeps its own table unexported.
var voidHTMLElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "keygen": true, "link": true,
	"meta": true, "param": true, "source": true, "track": true, "wbr": true,
}

// extractHTMLBlockMissingBlanks walks the AST's Type 6/7 HTML blocks and
// reports text that follows the completed top-level element on a later
// physical line. Types 1–5 (script, comment, doctype, …) end on their own
// closing conditions and stay out of scope; blocks nested in blockquotes or
// list items are skipped because their lines carry container prefixes the
// tokenizer would misread. Every ambiguous structure — mismatched close
// tags, stray end tags, unlocatable token bytes — skips the block: the rule
// trades recall for precision, and a missing blank line is a warning no
// legitimate document should have to defend against.
func extractHTMLBlockMissingBlanks(document ast.Node, source []byte) []HTMLBlockMissingBlank {
	findings := make([]HTMLBlockMissingBlank, 0)
	locator := newSourceLocator(source)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		block, ok := node.(*ast.HTMLBlock)
		if !ok || (block.HTMLBlockType != ast.HTMLBlockType6 && block.HTMLBlockType != ast.HTMLBlockType7) {
			return ast.WalkContinue, nil
		}
		// A container's lines carry their marker before the HTML; only
		// document-level blocks keep tag bytes contiguous with positions.
		if block.Parent() != document {
			return ast.WalkContinue, nil
		}
		lines := block.Lines()
		if lines.Len() == 0 {
			return ast.WalkContinue, nil
		}
		findings = append(findings, htmlBlockBlankFindings(source, locator,
			lines.At(0).Start, lines.At(lines.Len()-1).Stop)...)
		return ast.WalkContinue, nil
	})
	sort.SliceStable(findings, func(left, right int) bool {
		return findings[left].Position.Line < findings[right].Position.Line
	})
	return findings
}

// htmlBlockBlankFindings analyzes one HTML block's raw bytes. An element
// stack tracks the top-level element's completion — the end tag that empties
// the stack, or a void/self-closing tag at the top level — and any non-blank
// text token after a completion, starting on a later line, is Markdown the
// author meant to render: that text needs a blank line before it. A physical
// line reports at most once no matter how the tokenizer splits it, and any
// structure the stack cannot reliably match — a mismatched close tag after
// candidates were already gathered — discards the whole block's findings.
func htmlBlockBlankFindings(source []byte, locator sourceLocator, start int, stop int) []HTMLBlockMissingBlank {
	findings := make([]HTMLBlockMissingBlank, 0)
	tokenizer := html.NewTokenizer(bytes.NewReader(source[start:stop]))
	stack := make([]string, 0, 4)
	lastEndedLine := 0
	reportedLine := 0
	cursor := 0
	for {
		token := tokenizer.Next()
		if token == html.ErrorToken {
			return findings
		}
		raw := tokenizer.Raw()
		if len(raw) == 0 {
			continue
		}
		// Anchor the token to its absolute source offset; the tokenizer
		// works on the block slice, so the raw bytes are searched from the
		// previous token's end. A token whose bytes cannot be found ends
		// the analysis of this block.
		offset := bytes.Index(source[start+cursor:stop], raw)
		if offset < 0 {
			return findings
		}
		absolute := start + cursor + offset
		cursor += offset + len(raw)
		switch token {
		case html.TextToken:
			blank := firstBlankPrefix(raw)
			if blank < 0 {
				continue // whitespace-only text is layout, not content
			}
			if len(stack) > 0 {
				continue // text inside an element is that element's content
			}
			line, _ := locator.locate(absolute + blank)
			if line > lastEndedLine && line > reportedLine {
				findings = append(findings, HTMLBlockMissingBlank{Position: Position{Line: line, Column: 1}})
				reportedLine = line
			}
		case html.StartTagToken:
			name := strings.ToLower(tokenizer.Token().Data)
			if voidHTMLElements[name] {
				if len(stack) == 0 {
					line, _ := locator.locate(absolute)
					lastEndedLine = line
				}
				continue
			}
			stack = append(stack, name)
		case html.SelfClosingTagToken:
			if len(stack) == 0 {
				line, _ := locator.locate(absolute)
				lastEndedLine = line
			}
		case html.EndTagToken:
			name := strings.ToLower(tokenizer.Token().Data)
			if len(stack) == 0 || stack[len(stack)-1] != name {
				// Structure we cannot reliably match: skip the block —
				// candidates gathered before the mismatch go with it.
				return nil
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				line, _ := locator.locate(absolute)
				lastEndedLine = line
			}
		}
	}
}

// firstBlankPrefix returns the index of the first byte that is not
// whitespace, or -1 when the whole slice is whitespace.
func firstBlankPrefix(raw []byte) int {
	for index, current := range raw {
		switch current {
		case ' ', '\t', '\r', '\n', '\f', '\v':
		default:
			return index
		}
	}
	return -1
}
