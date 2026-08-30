package markdown

import (
	"bytes"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

// ReferenceRoute classifies a reference by the web route the renderer sends
// it down, decided from the raw destination exactly as written — never from
// the decoded filesystem target, so an encoded `.md` (guide%2Emd) keeps the
// route the renderer actually picked for it.
type ReferenceRoute uint8

const (
	// ReferenceRouteLink is a destination treated like a Markdown link:
	// Markdown documents route to /doc, everything else to /assets.
	ReferenceRouteLink ReferenceRoute = iota + 1
	// ReferenceRouteAsset is a destination always routed to /assets:
	// Markdown images and the src/poster/data attributes of raw HTML.
	// The assets route never serves Markdown files.
	ReferenceRouteAsset
)

// Position is a 1-based source position of one inspection fact.
type Position struct {
	Line   int
	Column int
}

// Reference is one link, image or raw-HTML URL destination extracted from a
// document. Destination is kept exactly as the Markdown source wrote it — no
// sanitization or URL rewriting — and Line/Column locate it 1-based in the
// source. Text is the link text or image alt text (empty for raw HTML).
// ReferenceLabel holds the raw label of a reference-style link or image
// ([text][label], [label][] or the shortcut [label]) and is empty for inline
// links ([text](dest)), so rules can tell how the destination was written.
// Inline link and image nodes carry no position of their own, so the
// position comes from the node's first child segment (the link or alt text)
// and, when the node has none, from the first literal occurrence of the
// destination after the previous reference. Raw-HTML URLs instead locate at
// the attribute value itself inside the tag.
type Reference struct {
	Kind           ReferenceKind
	Route          ReferenceRoute
	Destination    string
	Text           string
	ReferenceLabel string
	Line           int
	Column         int
}

// ReferenceDefinition is one link reference definition ([label]: destination)
// the parser accepted. Goldmark keeps the first definition of a label and
// silently ignores later duplicates, so facts keep the first position too.
type ReferenceDefinition struct {
	Label       string
	Destination string
	Position    Position
}

// ReferenceUse is one explicit reference-style use — [text][label] or
// [text][] — whose label the parser could not resolve to a definition. The
// judgement is Goldmark's (see inspectionContext); the source scan only
// recovers where the rejected use sits. Shortcut [label] alone never
// becomes a ReferenceUse: in plain prose a bracketed word is usually text,
// not a link attempt.
type ReferenceUse struct {
	Label    string
	Position Position
}

// Footnote is one footnote definition ([^label]: content) with what the
// parser knew about it at transform time: whether any use referenced it and
// whether it carries any content at all.
type Footnote struct {
	Label    string
	Used     bool
	Empty    bool
	Position Position
}

// FootnoteReference is one [^label] use whose label no definition matches.
// Footnote labels compare by exact bytes — Goldmark never folds case or
// whitespace for them — so the candidate scan compares the same way.
type FootnoteReference struct {
	Label    string
	Position Position
}

// Inspection is the check-oriented view of one Markdown document: the same
// heading anchors the WebUI table of contents shows, every reference the web
// renderer would rewrite, reference definitions and the uses the parser
// rejected, and how many H1 headings the document contains. It reports what
// the Markdown contains, never what a rule should conclude — rule ids and
// severities live in the check package.
type Inspection struct {
	Headings             []Heading
	References           []Reference
	ReferenceDefinitions []ReferenceDefinition
	UndefinedReferences  []ReferenceUse
	UndefinedFootnotes   []FootnoteReference
	Footnotes            []Footnote
	CodeFences           []CodeFence
	H1Count              int
}

// ShiftLines moves every fact position by delta lines. Callers that inspect
// the Markdown body after splitting off a frontmatter block pass
// FrontMatterLineOffset(source)-1 so every body fact enters the rest of the
// pipeline carrying file-level lines; columns are untouched because the
// offset is always whole lines.
func (inspection *Inspection) ShiftLines(delta int) {
	for index := range inspection.Headings {
		inspection.Headings[index].Line += delta
	}
	for index := range inspection.References {
		inspection.References[index].Line += delta
	}
	for index := range inspection.ReferenceDefinitions {
		inspection.ReferenceDefinitions[index].Position.Line += delta
	}
	for index := range inspection.UndefinedReferences {
		inspection.UndefinedReferences[index].Position.Line += delta
	}
	for index := range inspection.UndefinedFootnotes {
		inspection.UndefinedFootnotes[index].Position.Line += delta
	}
	for index := range inspection.Footnotes {
		inspection.Footnotes[index].Position.Line += delta
	}
	for index := range inspection.CodeFences {
		inspection.CodeFences[index].Position.Line += delta
	}
}

// inspectionContext wraps the parse context to observe reference lookups.
// Goldmark itself decides whether a reference-style link resolves; a failed
// lookup leaves plain text with no AST trace, so the fact must be recorded
// while the parse is running. Labels are recorded in Goldmark's normalized
// form (see NormalizeReferenceLabel) exactly as the parser looked them up.
type inspectionContext struct {
	parser.Context

	missingReferences map[string]struct{}
}

// Reference intercepts every label lookup during the parse and records the
// labels Goldmark rejected. Keeping the judgement inside the real parser is
// what makes the later "undefined" facts exact: escaped brackets, code
// spans and other constructs never trigger a lookup, so they can never be
// reported.
func (context *inspectionContext) Reference(label string) (parser.Reference, bool) {
	reference, ok := context.Context.Reference(label)
	if !ok {
		context.missingReferences[label] = struct{}{}
	}
	return reference, ok
}

// Inspect parses source with the exact engine, extensions and
// GitHub-compatible id generator that Render uses, then collects heading
// anchors and reference destinations from the resulting AST. Because
// extraction walks the AST instead of the raw text, URLs inside fenced or
// inline code never become references, and heading ids always match the
// anchors Render would emit.
//
// The same parse feeds a source scan that recovers what the final AST can
// no longer represent: reference uses the parser rejected, and the fenced
// code blocks whose fence lines the AST strips away.
func Inspect(source []byte) Inspection {
	engine := newEngine()
	var footnotes []Footnote
	// Observe the footnote list one priority step below Goldmark's own
	// footnote transformer (999), while every definition — used or not — is
	// still attached and each definition's Index already tells whether a use
	// referenced it.
	engine.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&footnoteInspectionTransformer{footnotes: &footnotes}, 998),
	))
	context := &inspectionContext{
		Context:           parser.NewContext(parser.WithIDs(newGitHubIDs())),
		missingReferences: make(map[string]struct{}),
	}
	document := engine.Parser().Parse(text.NewReader(source), parser.WithContext(context))

	inspection := Inspection{Headings: extractHeadings(document, source)}
	for _, heading := range inspection.Headings {
		if heading.Level == 1 {
			inspection.H1Count++
		}
	}
	inspection.References = extractReferences(document, source)
	inspection.ReferenceDefinitions = extractReferenceDefinitions(document, source)

	scanner := newSourceScanner(source, codeRanges(document))
	inspection.UndefinedReferences = scanner.undefinedReferences(context.missingReferences)
	inspection.UndefinedFootnotes = scanner.undefinedFootnotes(footnotes)
	inspection.Footnotes = footnotes
	inspection.CodeFences = scanner.fences
	return inspection
}

// footnoteInspectionTransformer records the footnote facts before Goldmark's
// own footnote transformer runs: after priority 999 the unreferenced
// definitions are gone from the AST, which is exactly the information the
// check rules need. It only observes — the parse output stays identical.
type footnoteInspectionTransformer struct {
	footnotes *[]Footnote
}

// Transform implements parser.ASTTransformer.
func (transformer *footnoteInspectionTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	locator := newSourceLocator(source)
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		list, ok := child.(*extensionast.FootnoteList)
		if !ok {
			return ast.WalkContinue, nil
		}
		for definition := list.FirstChild(); definition != nil; definition = definition.NextSibling() {
			footnote, ok := definition.(*extensionast.Footnote)
			if !ok {
				continue
			}
			line, column := locator.locate(max(footnote.Pos(), 0))
			*transformer.footnotes = append(*transformer.footnotes, Footnote{
				Label: string(footnote.Ref),
				// Index stays -1 until the first use references the definition.
				Used: footnote.Index >= 0,
				// A definition without children carries no content at all —
				// not on its first line nor on any indented continuation.
				Empty:    footnote.ChildCount() == 0,
				Position: Position{Line: line, Column: column},
			})
		}
		return ast.WalkContinue, nil
	})
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
			collector.markdownReference(ReferenceLink, ReferenceRouteLink, string(typed.Destination), referenceLabel(typed.Reference), node)
		case *ast.Image:
			collector.markdownReference(ReferenceImage, ReferenceRouteAsset, string(typed.Destination), referenceLabel(typed.Reference), node)
		case *ast.HTMLBlock:
			lines := typed.Lines()
			if lines.Len() == 0 {
				return ast.WalkContinue, nil
			}
			first := lines.At(0)
			stop := lines.At(lines.Len() - 1).Stop
			if typed.HasClosure() {
				stop = typed.ClosureLine.Stop
			}
			// The block's lines are contiguous source bytes; taking the whole
			// span keeps byte offsets true for tags that span several lines.
			collector.rawReferences(source[first.Start:stop], first.Start)
		case *ast.RawHTML:
			// Goldmark splits one multi-line tag into whitespace-separated
			// segments; the full span between the first and the last maps
			// byte-for-byte onto the source, so offsets stay exact.
			if typed.Segments.Len() == 0 {
				return ast.WalkContinue, nil
			}
			first := typed.Segments.At(0)
			last := typed.Segments.At(typed.Segments.Len() - 1)
			collector.rawReferences(source[first.Start:last.Stop], first.Start)
		}
		return ast.WalkContinue, nil
	})
	return collector.references
}

// markdownReference records one Markdown link or image at its node's earliest
// descendant segment — the link or alt text. A childless node (an image with
// no alt text) has no segment, so its position falls back to the first
// literal occurrence of the destination after the previous reference.
func (collector *referenceCollector) markdownReference(kind ReferenceKind, route ReferenceRoute, destination string, label string, node ast.Node) {
	offset, ok := nodeOffset(node)
	if !ok {
		offset = collector.searchDestination(destination)
	}
	collector.record(kind, route, destination, string(node.Text(collector.source)), label, offset)
}

// rawReferences records every URL attribute inside one raw HTML snippet —
// raw is the snippet's exact source bytes and base its source offset — using
// the same attribute list the renderer rewrites so raw HTML references can
// never drift between rendering and checking. Each reference keeps the
// attribute's own route intent and locates at the attribute value inside the
// tag rather than at the snippet start.
func (collector *referenceCollector) rawReferences(raw []byte, base int) {
	for _, found := range extractRawHTMLReferences(raw) {
		route := ReferenceRouteLink
		if found.asset {
			route = ReferenceRouteAsset
		}
		collector.record(ReferenceRawHTML, route, found.destination, "", "", base+found.offset)
	}
	if base > collector.searchFrom {
		collector.searchFrom = base
	}
}

func (collector *referenceCollector) record(kind ReferenceKind, route ReferenceRoute, destination string, text string, label string, offset int) {
	if offset > collector.searchFrom {
		collector.searchFrom = offset
	}
	line, column := collector.locator.locate(offset)
	collector.references = append(collector.references, Reference{
		Kind:           kind,
		Route:          route,
		Destination:    destination,
		Text:           text,
		ReferenceLabel: label,
		Line:           line,
		Column:         column,
	})
}

// referenceLabel returns the raw reference-style label of a link or image —
// the part inside the second bracket pair of [text][label], the label of a
// collapsed [label][] use, or the text of a shortcut [label] use — or the
// empty string for an inline link written as [text](destination).
func referenceLabel(reference *ast.ReferenceLink) string {
	if reference == nil {
		return ""
	}
	return string(reference.Value)
}

// extractReferenceDefinitions collects the reference definitions Goldmark
// accepted, in document order. Duplicate labels keep the first definition —
// the one Goldmark resolves uses against — matching the parser's own
// first-wins rule.
func extractReferenceDefinitions(document ast.Node, source []byte) []ReferenceDefinition {
	definitions := make([]ReferenceDefinition, 0)
	seen := make(map[string]struct{})
	locator := newSourceLocator(source)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		definition, ok := node.(*ast.LinkReferenceDefinition)
		if !ok {
			return ast.WalkContinue, nil
		}
		label := NormalizeReferenceLabel(string(definition.Label))
		if _, duplicate := seen[label]; duplicate {
			return ast.WalkContinue, nil
		}
		seen[label] = struct{}{}
		line, column := 1, 1
		if definition.Lines().Len() > 0 {
			line, column = locator.locate(definition.Lines().At(0).Start)
		}
		definitions = append(definitions, ReferenceDefinition{
			Label:       string(definition.Label),
			Destination: string(definition.Destination),
			Position:    Position{Line: line, Column: column},
		})
		return ast.WalkContinue, nil
	})
	return definitions
}

// NormalizeReferenceLabel applies Goldmark's link reference normalization —
// trim, full Unicode case folding, whitespace collapsed to single spaces —
// so callers comparing labels against parser lookups can never disagree
// about label identity.
func NormalizeReferenceLabel(label string) string {
	return util.ToLinkReference([]byte(label))
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

// rawHTMLReference is one URL attribute inside raw HTML: the value as the
// source wrote it, whether the attribute marks an asset (src/poster/data)
// rather than a link (href), and the byte offset of the value inside the raw
// snippet so diagnostics can point at the URL itself.
type rawHTMLReference struct {
	destination string
	asset       bool
	offset      int
}

// extractRawHTMLReferences returns every URL attribute of every start tag in
// raw, in document order. Attribute semantics come from the same table the
// renderer rewrites with, so route intent can never drift between rendering
// and checking.
func extractRawHTMLReferences(raw []byte) []rawHTMLReference {
	tokenizer := html.NewTokenizer(bytes.NewReader(raw))
	references := make([]rawHTMLReference, 0)
	cursor := 0
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return references
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			tag := tokenizer.Raw()
			tagStart := cursor
			if index := bytes.Index(raw[cursor:], tag); index >= 0 {
				tagStart = cursor + index
				cursor = tagStart
			}
			for _, attribute := range token.Attr {
				asset, relevant := isRawHTMLURLAttribute(attribute.Key)
				if !relevant {
					continue
				}
				references = append(references, rawHTMLReference{
					destination: attribute.Val,
					asset:       asset,
					offset:      tagStart + attributeValueOffset(tag, attribute.Key),
				})
			}
			cursor += len(tag)
		}
	}
}

// attributeValueOffset returns the byte offset of an attribute's value inside
// its start tag, or 0 when it cannot be located. It points just past the
// opening quote, the way a Markdown link position points at its destination.
// Unquoted values point at the value's first byte.
func attributeValueOffset(tag []byte, key string) int {
	masked := maskQuoted(asciiLower(string(tag)))
	maskedKey := asciiLower(key)
	for search := 0; ; {
		index := strings.Index(masked[search:], maskedKey)
		if index < 0 {
			return 0
		}
		start := search + index
		after := start + len(maskedKey)
		// The occurrence must be a whole attribute name — preceded by tag or
		// attribute whitespace — followed by whitespace and an '=', not text
		// that merely looks like one inside another attribute's value.
		if (start == 0 || isASCIISpace(masked[start-1])) && attributeContinues(masked, after) {
			return attributeValueStart(masked, after)
		}
		search = start + 1
	}
}

// attributeContinues reports whether the name ending at after is followed by
// optional whitespace and the '=' that introduces a value.
func attributeContinues(masked string, after int) bool {
	for after < len(masked) && isASCIISpace(masked[after]) {
		after++
	}
	return after < len(masked) && masked[after] == '='
}

// attributeValueStart returns the offset of the value that follows the '=' at
// after: skip whitespace, the '=', more whitespace, then one opening quote.
func attributeValueStart(masked string, after int) int {
	for after < len(masked) && isASCIISpace(masked[after]) {
		after++
	}
	after++ // '='
	for after < len(masked) && isASCIISpace(masked[after]) {
		after++
	}
	if after < len(masked) && (masked[after] == '"' || masked[after] == '\'') {
		after++
	}
	return min(after, len(masked))
}

// maskQuoted replaces bytes inside quoted attribute values with NUL so a key
// search can never match text that only appears inside another attribute's
// value. Byte length and positions are preserved.
func maskQuoted(value string) string {
	masked := []byte(value)
	var quote byte
	for index, current := range masked {
		switch {
		case quote != 0:
			if current == quote {
				quote = 0
			} else {
				masked[index] = 0
			}
		case current == '"' || current == '\'':
			quote = current
		}
	}
	return string(masked)
}

// asciiLower lowercases ASCII letters only, preserving byte length so offsets
// into the result stay valid for the original.
func asciiLower(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, value)
}

func isASCIISpace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	}
	return false
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
