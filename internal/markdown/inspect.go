package markdown

import (
	"bytes"
	"sort"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

// ReferenceKind classifies a document reference by where it came from.
type ReferenceKind uint8

const (
	// ReferenceLink is a Markdown link destination, inline or reference style.
	ReferenceLink ReferenceKind = iota + 1
	// ReferenceImage is a Markdown image destination.
	ReferenceImage
	// ReferenceRawHTML is a URL inside raw HTML (href, src, poster, data).
	ReferenceRawHTML
)

// Reference is one link, image or raw-HTML URL destination extracted from a
// document. Destination is kept exactly as the Markdown source wrote it — no
// sanitization or URL rewriting — and Line/Column locate it 1-based in the
// source. Inline link and image nodes carry no position of their own, so the
// position comes from the node's first child segment (the link or alt text)
// and, when the node has none, from the first literal occurrence of the
// destination after the previous reference.
type Reference struct {
	Kind        ReferenceKind
	Destination string
	Line        int
	Column      int
}

// Inspection is the check-oriented view of one Markdown document: the same
// heading anchors the WebUI table of contents shows, every reference the web
// renderer would rewrite, and how many H1 headings the document contains.
type Inspection struct {
	Headings   []Heading
	References []Reference
	H1Count    int
}

// Inspect parses source with the exact engine, extensions and
// GitHub-compatible id generator that Render uses, then collects heading
// anchors and reference destinations from the resulting AST. Because
// extraction walks the AST instead of the raw text, URLs inside fenced or
// inline code never become references, and heading ids always match the
// anchors Render would emit.
func Inspect(source []byte) Inspection {
	engine := newEngine()
	context := parser.NewContext(parser.WithIDs(newGitHubIDs()))
	document := engine.Parser().Parse(text.NewReader(source), parser.WithContext(context))

	inspection := Inspection{Headings: extractHeadings(document, source)}
	for _, heading := range inspection.Headings {
		if heading.Level == 1 {
			inspection.H1Count++
		}
	}
	inspection.References = extractReferences(document, source)
	return inspection
}

// referenceCollector walks one document's AST in order, keeping every located
// position at or after the previous one so childless nodes can fall back to a
// forward literal search for their destination.
type referenceCollector struct {
	source     []byte
	locator    sourceLocator
	references []Reference
	searchFrom int
}

func extractReferences(document ast.Node, source []byte) []Reference {
	collector := &referenceCollector{
		source:     source,
		locator:    newSourceLocator(source),
		references: make([]Reference, 0),
	}
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Text:
			// Tracking every text segment keeps the fallback search anchored
			// to the walk position, so a childless node can never match a
			// literal destination that appeared earlier in the document —
			// for example inside an inline code span.
			if end := typed.Segment.Stop; end > collector.searchFrom {
				collector.searchFrom = end
			}
		case *ast.Link:
			collector.markdownReference(ReferenceLink, string(typed.Destination), node)
		case *ast.Image:
			collector.markdownReference(ReferenceImage, string(typed.Destination), node)
		case *ast.HTMLBlock:
			raw := typed.Lines().Value(source)
			if typed.HasClosure() {
				raw = append(raw, typed.ClosureLine.Value(source)...)
			}
			collector.rawReferences(raw, node)
		case *ast.RawHTML:
			collector.rawReferences(typed.Segments.Value(source), node)
		}
		return ast.WalkContinue, nil
	})
	return collector.references
}

// markdownReference records one Markdown link or image at its node's earliest
// descendant segment — the link or alt text. A childless node (an image with
// no alt text) has no segment, so its position falls back to the first
// literal occurrence of the destination after the previous reference.
func (collector *referenceCollector) markdownReference(kind ReferenceKind, destination string, node ast.Node) {
	offset, ok := nodeOffset(node)
	if !ok {
		offset = collector.searchDestination(destination)
	}
	collector.record(kind, destination, offset)
}

// rawReferences records every URL attribute inside one raw HTML block or
// inline snippet at that node's position, using the same attribute list the
// renderer rewrites so raw HTML references can never drift between rendering
// and checking.
func (collector *referenceCollector) rawReferences(raw []byte, node ast.Node) {
	offset, ok := nodeOffset(node)
	if !ok {
		offset = collector.searchFrom
	}
	for _, destination := range extractRawHTMLDestinations(raw) {
		collector.record(ReferenceRawHTML, destination, offset)
	}
	if offset > collector.searchFrom {
		collector.searchFrom = offset
	}
}

func (collector *referenceCollector) record(kind ReferenceKind, destination string, offset int) {
	if offset > collector.searchFrom {
		collector.searchFrom = offset
	}
	line, column := collector.locator.locate(offset)
	collector.references = append(collector.references, Reference{
		Kind:        kind,
		Destination: destination,
		Line:        line,
		Column:      column,
	})
}

// searchDestination finds the first literal occurrence of destination at or
// after the previously recorded position, returning that offset — or the
// previous position when the destination cannot be found (for example when
// entity unescaping changed it). A hit advances the search position past the
// match so repeated childless references with the same destination each
// locate their own occurrence.
func (collector *referenceCollector) searchDestination(destination string) int {
	if destination == "" {
		return collector.searchFrom
	}
	index := bytes.Index(collector.source[collector.searchFrom:], []byte(destination))
	if index < 0 {
		return collector.searchFrom
	}
	offset := collector.searchFrom + index
	collector.searchFrom = offset + len(destination)
	return offset
}

// nodeOffset returns the earliest source offset of a node: its own segment
// when it carries one (text, raw HTML, HTML block lines), otherwise the
// earliest segment among its descendants in document order.
func nodeOffset(node ast.Node) (int, bool) {
	switch typed := node.(type) {
	case *ast.Text:
		return typed.Segment.Start, true
	case *ast.RawHTML:
		if typed.Segments.Len() > 0 {
			return typed.Segments.At(0).Start, true
		}
	case *ast.HTMLBlock:
		if typed.Lines().Len() > 0 {
			return typed.Lines().At(0).Start, true
		}
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if offset, ok := nodeOffset(child); ok {
			return offset, ok
		}
	}
	return 0, false
}

// extractRawHTMLDestinations returns the URL attribute value of every start
// tag in raw, in document order.
func extractRawHTMLDestinations(raw []byte) []string {
	tokenizer := html.NewTokenizer(bytes.NewReader(raw))
	destinations := make([]string, 0)
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return destinations
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			for _, attribute := range token.Attr {
				if _, relevant := isRawHTMLURLAttribute(attribute.Key); relevant {
					destinations = append(destinations, attribute.Val)
				}
			}
		}
	}
}

// sourceLocator maps byte offsets to 1-based line and column positions with
// a binary search over the source's line starts.
type sourceLocator struct {
	lineStarts []int
}

func newSourceLocator(source []byte) sourceLocator {
	lineStarts := make([]int, 1, bytes.Count(source, []byte("\n"))+1)
	for index, current := range source {
		if current == '\n' {
			lineStarts = append(lineStarts, index+1)
		}
	}
	return sourceLocator{lineStarts: lineStarts}
}

// locate returns the 1-based line and column of offset, clamping offsets
// beyond the source to the last line.
func (locator sourceLocator) locate(offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	line := sort.Search(len(locator.lineStarts), func(index int) bool {
		return locator.lineStarts[index] > offset
	})
	line = max(1, min(line, len(locator.lineStarts)))
	return line, offset - locator.lineStarts[line-1] + 1
}
