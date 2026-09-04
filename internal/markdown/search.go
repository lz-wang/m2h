package markdown

import (
	pathpkg "path"
	"strings"

	emojiast "github.com/yuin/goldmark-emoji/ast"
	"github.com/yuin/goldmark/ast"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// SearchChunkKind classifies one searchable chunk by the semantic role of its
// content. The Goldmark AST node types stay inside this package: consumers see
// only these three roles.
type SearchChunkKind uint8

const (
	// SearchChunkHeading is a heading's own text (H1–H6).
	SearchChunkHeading SearchChunkKind = iota + 1
	// SearchChunkText is visible prose: paragraphs, list items, table rows
	// and blockquotes.
	SearchChunkText
	// SearchChunkCode is fenced or indented code — the diagram sources
	// (Mermaid, Vega-Lite) are fenced code and are included.
	SearchChunkCode
)

// SearchChunk is one semantic unit of searchable document content. Text is the
// visible text: link and image destinations never enter it, while link text,
// image alt text and inline code do. HeadingID/HeadingText carry the section
// the chunk belongs to (its own heading for a heading chunk, the enclosing
// heading for everything else), so a matcher never has to map source
// positions back to sections. An empty HeadingID means "before the first
// anchored heading".
type SearchChunk struct {
	Kind        SearchChunkKind
	Text        string
	HeadingID   string
	HeadingText string
}

// SearchProjection is the search-facing projection of one document. Title is
// the first-H1 fallback title (frontmatter preference stays with the caller
// via PreferredTitle); Chunks are the semantic units in document order. The
// projection is produced from the same single Goldmark parse that yields
// heading anchors, so a caller needing title, anchors and searchable content
// together never parses a document twice.
type SearchProjection struct {
	Title  string
	Chunks []SearchChunk
}

// ProjectForSearch parses source with the exact engine and heading-id context
// used for rendering, and projects the AST into searchable semantic chunks.
// Because the parser context is identical to Render's, chunk heading ids are
// byte-for-byte the rendered anchors — CJK slugs and duplicate -1/-2 suffixes
// included. Raw HTML blocks are deliberately not indexed in this first
// version; inline raw HTML markup is dropped by the text collector.
func ProjectForSearch(source []byte, sourcePath string) (SearchProjection, error) {
	normalized, err := normalizeSourcePath(sourcePath)
	if err != nil {
		return SearchProjection{}, err
	}
	engine := newEngine()
	context := parser.NewContext(parser.WithIDs(newGitHubIDs()))
	document := engine.Parser().Parse(text.NewReader(source), parser.WithContext(context))

	projection := SearchProjection{Chunks: []SearchChunk{}}
	// section carries the nearest anchored heading; every text and code chunk
	// that follows until the next heading inherits it.
	section := SearchChunk{}
	title := ""
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Heading:
			headingText := normalizeTitle(inlineText(typed, source))
			id := headingAnchorID(typed)
			if headingText != "" {
				projection.Chunks = append(projection.Chunks, SearchChunk{
					Kind:        SearchChunkHeading,
					Text:        headingText,
					HeadingID:   id,
					HeadingText: headingText,
				})
			}
			if id != "" {
				section = SearchChunk{
					Kind:        SearchChunkHeading,
					Text:        headingText,
					HeadingID:   id,
					HeadingText: headingText,
				}
			}
			if typed.Level == 1 && title == "" {
				title = headingText
			}
			return ast.WalkSkipChildren, nil
		case *ast.Paragraph, *ast.TextBlock:
			// The two types are the leaf prose blocks: Paragraph for
			// document body and loose list items, TextBlock for tight list
			// items and blockquote text. Their inline children were already
			// consumed by the collector below.
			projection.appendText(inlineText(node, source), section)
			return ast.WalkSkipChildren, nil
		case *extensionast.TableHeader, *extensionast.TableRow:
			// One chunk per row keeps cell context together in a snippet;
			// cell separators become spaces, exactly as in the rendered row.
			// The header row's cells hang directly off TableHeader — there is
			// no TableRow node for it.
			var builder strings.Builder
			for cell := typed.FirstChild(); cell != nil; cell = cell.NextSibling() {
				builder.WriteString(inlineText(cell, source))
				builder.WriteByte(' ')
			}
			projection.appendText(builder.String(), section)
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock:
			projection.appendCode(typed.Lines(), source, section)
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			projection.appendCode(typed.Lines(), source, section)
			return ast.WalkSkipChildren, nil
		case *ast.HTMLBlock:
			// Raw HTML blocks are not indexed: the text-without-markup
			// extraction they would need is a separate projection step.
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	if title == "" {
		title = pathpkg.Base(normalized)
	}
	projection.Title = title
	return projection, nil
}

// appendText adds one prose chunk when its normalized text is non-empty.
func (projection *SearchProjection) appendText(raw string, section SearchChunk) {
	text := normalizeTitle(raw)
	if text == "" {
		return
	}
	projection.Chunks = append(projection.Chunks, SearchChunk{
		Kind:        SearchChunkText,
		Text:        text,
		HeadingID:   section.HeadingID,
		HeadingText: section.HeadingText,
	})
}

// appendCode adds one code chunk from the block's own source lines.
func (projection *SearchProjection) appendCode(lines *text.Segments, source []byte, section SearchChunk) {
	var builder strings.Builder
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		builder.Write(segment.Value(source))
	}
	code := strings.TrimSpace(builder.String())
	if code == "" {
		return
	}
	projection.Chunks = append(projection.Chunks, SearchChunk{
		Kind:        SearchChunkCode,
		Text:        code,
		HeadingID:   section.HeadingID,
		HeadingText: section.HeadingText,
	})
}

// inlineText collects the visible text of an inline subtree. Text nodes and
// String nodes (code span content, emoji) contribute; inline raw HTML markup
// is dropped, while regular links contribute their text and images their alt
// text — destinations are node fields, never children, so they cannot leak
// into the collected text. An autolink contributes its label: `<https://…>`
// and Linkify URLs render the URL itself as the text the reader sees, which
// is also what heading.Text collects for a heading, keeping the search
// projection's fallback title identical to the rendered one.
func inlineText(node ast.Node, source []byte) string {
	var builder strings.Builder
	_ = ast.Walk(node, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := current.(type) {
		case *ast.Text:
			segment := typed.Segment
			builder.Write(segment.Value(source))
			// Both break kinds separate the lines the reader sees; without
			// the space a hard break would glue its two words together.
			if typed.SoftLineBreak() || typed.HardLineBreak() {
				builder.WriteByte(' ')
			}
			return ast.WalkSkipChildren, nil
		case *ast.String:
			builder.Write(typed.Value)
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		case *ast.AutoLink:
			builder.Write(typed.Label(source))
			return ast.WalkSkipChildren, nil
		case *emojiast.Emoji:
			// newEngine renders emoji as Unicode characters; the searched
			// text is the same characters the reader sees.
			builder.WriteString(string(typed.Value.Unicode))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return builder.String()
}

// headingAnchorID returns the anchor id the parser context assigned to the
// heading, or "" when the heading carries none — the same extraction
// extractHeadings applies for the table of contents.
func headingAnchorID(heading *ast.Heading) string {
	rawID, ok := heading.AttributeString("id")
	if !ok {
		return ""
	}
	id, ok := rawID.([]byte)
	if !ok {
		return ""
	}
	return string(id)
}
