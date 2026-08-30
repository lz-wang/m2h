package markdown

import (
	"bytes"
	"maps"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// byteRange is one half-open [start, stop) span of the source.
type byteRange struct {
	start int
	stop  int
}

// codeRanges collects the byte ranges Goldmark parsed as code: fenced and
// indented code block contents plus inline code span contents. Everything
// inside them is literal text the link and footnote parsers never saw, so
// the source scan must never report syntax found there. Ranges come from the
// AST rather than a hand-rolled block parser, which keeps the scan in lock
// step with real parsing — including indented code inside list items and
// code spans inside blockquotes.
func codeRanges(document ast.Node) []byteRange {
	ranges := make([]byteRange, 0)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			lines := typed.Lines()
			if lines.Len() == 0 {
				return ast.WalkContinue, nil
			}
			ranges = append(ranges, byteRange{
				start: lines.At(0).Start,
				stop:  lines.At(lines.Len() - 1).Stop,
			})
		case *ast.CodeSpan:
			for child := typed.FirstChild(); child != nil; child = child.NextSibling() {
				if text, ok := child.(*ast.Text); ok {
					ranges = append(ranges, byteRange{start: text.Segment.Start, stop: text.Segment.Stop})
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return ranges
}

// rawHTMLRanges collects the byte ranges Goldmark passes through as raw
// HTML: HTML blocks (comment blocks among them) and inline raw HTML tags.
// The parser never interprets brackets inside them, so the syntax scans must
// never report candidates there. Inline literal-content elements — <code>,
// <kbd>, … — carry Markdown-uninteresting verbatim text between their tags,
// so each open/close pair's whole span joins the protected ranges; the tags
// alone would leave that span scannable.
func rawHTMLRanges(document ast.Node, source []byte) []byteRange {
	ranges := make([]byteRange, 0)
	// rawTag is one inline raw HTML tag kept for literal-element pairing.
	type rawTag struct {
		span    byteRange
		name    string
		closing bool
	}
	inline := make([]rawTag, 0)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.HTMLBlock:
			lines := typed.Lines()
			if lines.Len() == 0 {
				return ast.WalkContinue, nil
			}
			start := lines.At(0).Start
			stop := lines.At(lines.Len() - 1).Stop
			if typed.HasClosure() && typed.ClosureLine.Stop > stop {
				stop = typed.ClosureLine.Stop
			}
			ranges = append(ranges, byteRange{start: start, stop: stop})
		case *ast.RawHTML:
			if typed.Segments.Len() == 0 {
				return ast.WalkContinue, nil
			}
			first := typed.Segments.At(0)
			last := typed.Segments.At(typed.Segments.Len() - 1)
			span := byteRange{start: first.Start, stop: last.Stop}
			ranges = append(ranges, span)
			name, closing, selfClosing := htmlTag(source[first.Start:last.Stop])
			if _, literal := literalElements[name]; !literal || selfClosing {
				return ast.WalkContinue, nil
			}
			inline = append(inline, rawTag{span: span, name: name, closing: closing})
		}
		return ast.WalkContinue, nil
	})
	for open := 0; open < len(inline); open++ {
		if inline[open].closing {
			continue
		}
		for next := open + 1; next < len(inline); next++ {
			if inline[next].closing && inline[next].name == inline[open].name {
				ranges = append(ranges, byteRange{start: inline[open].span.start, stop: inline[next].span.stop})
				open = next
				break
			}
		}
	}
	sort.Slice(ranges, func(left, right int) bool {
		return ranges[left].start < ranges[right].start
	})
	return ranges
}

// literalElements are the inline HTML elements whose content the browser
// renders verbatim, so bracket shapes inside them are quoted syntax, never
// a broken link attempt.
var literalElements = map[string]bool{
	"code":     true,
	"pre":      true,
	"kbd":      true,
	"samp":     true,
	"var":      true,
	"script":   true,
	"style":    true,
	"textarea": true,
	"title":    true,
}

// htmlTag parses one raw HTML tag: its lowercased element name, whether it
// is a closing tag, and whether it closes itself (<br/>).
func htmlTag(raw []byte) (name string, closing bool, selfClosing bool) {
	if len(raw) < 3 || raw[0] != '<' {
		return "", false, false
	}
	index := 1
	if raw[index] == '/' {
		closing = true
		index++
	}
	start := index
	for index < len(raw) && raw[index] != '>' && !isASCIISpace(raw[index]) {
		index++
	}
	selfClosing = raw[len(raw)-2] == '/'
	return asciiLower(string(raw[start:index])), closing, selfClosing
}

// pendingFence is one fenced code block whose opener line the AST gives no
// anchor for — no content lines and no info string — held until the walk
// reaches a later offset that bounds where its opener can sit.
type pendingFence struct {
	index int // position in the fences slice
	lower int // source offset the walk had consumed when the block appeared
}

// extractFences walks the AST's fenced code blocks, recovering each opener
// line's position and span from the source. Whether something is a fence and
// what language it carries is the parser's verdict alone: a line scanner
// guessing at raw text gets containers wrong (blockquote and list item lines
// carry their marker before the fence) and counts fence-looking lines inside
// raw HTML blocks. Only the position is recovered here — the AST keeps a
// fence's content but strips the fence lines themselves. The opener lines
// come back as protected ranges: their info strings are literal text.
func extractFences(document ast.Node, source []byte) ([]CodeFence, []byteRange) {
	locator := newSourceLocator(source)
	fences := make([]CodeFence, 0)
	openers := make([]byteRange, 0)
	cursor := 0
	pending := make([]pendingFence, 0)

	// resolve locates every pending opener. The window since a fence's lower
	// bound holds nothing but that fence's own lines, so the first
	// fence-shaped line in it is the opener; consecutive fences resolve in
	// order, each search resuming past the previous fence's closing line.
	resolve := func(upper int) {
		after := 0
		for _, fence := range pending {
			opener, past, found := locateFencePair(source, locator, max(fence.lower, after), upper)
			if !found {
				continue
			}
			line, column := locator.locate(opener)
			fences[fence.index].Position = Position{Line: line, Column: column}
			openers[fence.index] = lineSpan(source, opener)
			after = past
		}
		pending = pending[:0]
	}

	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if start, ok := nodeOffset(node); ok {
			// A node with a source position bounds every still-pending fence.
			resolve(start)
		}
		if end, ok := nodeSourceEnd(node); ok && end > cursor {
			cursor = end
		}
		block, ok := node.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		index := len(fences)
		fences = append(fences, CodeFence{Language: string(block.Language(source))})
		openers = append(openers, byteRange{})
		if anchor, hasContent := fenceAnchor(block); anchor >= 0 {
			var opener int
			if hasContent {
				// The first content line follows the opener line directly;
				// the segment itself may start mid-line (container prefix).
				opener = locator.previousLineStart(anchor)
			} else {
				// The info string is part of the opener line itself.
				opener = locator.lineStart(anchor)
			}
			line, column := locator.locate(opener)
			fences[index].Position = Position{Line: line, Column: column}
			openers[index] = lineSpan(source, opener)
		} else {
			pending = append(pending, pendingFence{index: index, lower: cursor})
		}
		return ast.WalkContinue, nil
	})
	resolve(len(source))
	return fences, openers
}

// fenceAnchor returns a source offset that locates a fence's opener line:
// the first content line (whose start the opener line directly precedes), or
// the info string (which is part of the opener line). hasContent tells the
// two apart; (-1, false) means the bare fence carries neither.
func fenceAnchor(block *ast.FencedCodeBlock) (anchor int, hasContent bool) {
	if lines := block.Lines(); lines.Len() > 0 {
		return lines.At(0).Start, true
	}
	if block.Info != nil {
		return block.Info.Segment.Start, false
	}
	return -1, false
}

// nodeSourceEnd returns the offset just past a node's own source text, for
// walk-position tracking: text segments for inline nodes, block lines for
// block nodes. Nodes carrying no position (an entity string, an empty fence)
// report false and leave the cursor alone.
func nodeSourceEnd(node ast.Node) (int, bool) {
	switch typed := node.(type) {
	case *ast.Text:
		return typed.Segment.Stop, true
	case *ast.RawHTML:
		if typed.Segments.Len() > 0 {
			return typed.Segments.At(typed.Segments.Len() - 1).Stop, true
		}
	}
	// Lines() panics on inline nodes; only block nodes carry block lines.
	if node.Type() != ast.TypeBlock {
		return 0, false
	}
	if lines := node.Lines(); lines != nil && lines.Len() > 0 {
		return lines.At(lines.Len() - 1).Stop, true
	}
	return 0, false
}

// locateFencePair locates a bare fence's opener line between lower and
// upper, plus the offset just past its closing line. The first line whose
// content — after any container prefix of blockquote and list markers —
// opens a fence run is the opener; the closer is the next line whose run of
// the same character is at least as long, trailing nothing but spaces, and
// an unclosed fence runs to upper.
func locateFencePair(source []byte, locator sourceLocator, lower int, upper int) (int, int, bool) {
	for index := sort.SearchInts(locator.lineStarts, lower); index < len(locator.lineStarts); index++ {
		opener := locator.lineStarts[index]
		if opener >= upper {
			break
		}
		char, run, prefixed := fenceRun(source[opener:lineStartStop(source, opener)])
		if !prefixed || run < 3 {
			continue
		}
		return opener, locateFenceClose(source, locator, index+1, char, run, upper), true
	}
	return 0, 0, false
}

// locateFenceClose returns the offset just past the closing fence line at or
// after the given line index, or upper when the fence never closes.
func locateFenceClose(source []byte, locator sourceLocator, index int, char byte, run int, upper int) int {
	for ; index < len(locator.lineStarts); index++ {
		start := locator.lineStarts[index]
		if start >= upper {
			break
		}
		line := source[start:lineStartStop(source, start)]
		content := line[containerPrefixLength(line):]
		length := 0
		for length < len(content) && content[length] == char {
			length++
		}
		if length >= run && util.IsBlank(content[length:]) {
			return min(start+len(line)+1, len(source))
		}
	}
	return upper
}

// fenceRun reports the fence character and run length a line opens with
// after any container prefix, or prefixed=false when the line does not open
// with a fence run.
func fenceRun(line []byte) (char byte, run int, prefixed bool) {
	content := line[containerPrefixLength(line):]
	if len(content) < 3 {
		return 0, 0, false
	}
	char = content[0]
	if char != '`' && char != '~' {
		return 0, 0, false
	}
	for run < len(content) && content[run] == char {
		run++
	}
	if run < 3 {
		return 0, 0, false
	}
	return char, run, true
}

// containerPrefixLength counts the bytes a container can put before a fence
// opener — blockquote markers, list markers and the indentation around
// them.
func containerPrefixLength(line []byte) int {
	index := 0
	for index < len(line) {
		current := line[index]
		switch {
		case current == ' ' || current == '\t' || current == '>' || current == '-' || current == ')' || current == '.' || (current >= '0' && current <= '9'):
			index++
		default:
			return index
		}
	}
	return index
}

// lineStartStop returns the offset of the newline ending the line that
// starts at start, or the end of source for the last line.
func lineStartStop(source []byte, start int) int {
	if index := bytes.IndexByte(source[start:], '\n'); index >= 0 {
		return start + index
	}
	return len(source)
}

// lineSpan returns the [start, stop) span of the line starting at start,
// newline excluded.
func lineSpan(source []byte, start int) byteRange {
	return byteRange{start: start, stop: lineStartStop(source, start)}
}

// sourceScanner walks one document's source once, recovering syntax the
// final AST can no longer represent: uses the parser rejected. It never
// re-parses what the AST still knows — headings, resolved references, tables
// and fenced code blocks come from the AST — and never reports anything
// inside code, because code is literal text no parser ever interpreted.
type sourceScanner struct {
	source     []byte
	locator    sourceLocator
	code       []byteRange // AST code ranges, sorted by start
	rawHTML    []byteRange // AST raw HTML ranges, sorted by start
	fenceLines []byteRange // the fence opener lines, from the AST fences
}

// CodeFence is one fenced code block the parser accepted. The AST keeps the
// content lines but strips the fence lines, so only the opener position is
// recovered from the source. Position is the opening fence line, column 1.
type CodeFence struct {
	Language string
	Position Position
}

// newSourceScanner prepares a scan over source, skipping the protected code,
// raw HTML and fence line ranges the AST identified.
func newSourceScanner(source []byte, code []byteRange, rawHTML []byteRange, fenceLines []byteRange) *sourceScanner {
	scanner := &sourceScanner{
		source:     source,
		locator:    newSourceLocator(source),
		code:       code,
		rawHTML:    rawHTML,
		fenceLines: fenceLines,
	}
	sort.Slice(scanner.code, func(left, right int) bool {
		return scanner.code[left].start < scanner.code[right].start
	})
	sort.Slice(scanner.rawHTML, func(left, right int) bool {
		return scanner.rawHTML[left].start < scanner.rawHTML[right].start
	})
	return scanner
}

// inCode reports whether offset sits inside a code range or a fence line,
// where nothing the scanner looks for can be real syntax.
func (scanner *sourceScanner) inCode(offset int) bool {
	index := sort.Search(len(scanner.code), func(index int) bool {
		return scanner.code[index].stop > offset
	})
	if index < len(scanner.code) && scanner.code[index].start <= offset {
		return true
	}
	for _, span := range scanner.fenceLines {
		if span.start <= offset && offset < span.stop {
			return true
		}
	}
	return false
}

// protected reports whether offset sits anywhere the Markdown parser never
// interprets brackets: code, fence lines and raw HTML — comment blocks,
// HTML blocks and inline literal-content elements among it. The syntax
// candidate scans stop here; the unicode scan is a source-quality scan and
// keeps a wider domain, using inCode alone.
func (scanner *sourceScanner) protected(offset int) bool {
	if scanner.inCode(offset) {
		return true
	}
	index := sort.Search(len(scanner.rawHTML), func(index int) bool {
		return scanner.rawHTML[index].stop > offset
	})
	return index < len(scanner.rawHTML) && scanner.rawHTML[index].start <= offset
}

// undefinedReferences returns the located reference uses whose labels the
// parser rejected, in source order, at most as many per label as the parser
// rejected — the scan can find bracket pairs the parser never attempted to
// resolve (inside a link title, say), and those must not become diagnostics.
// Only the explicit [text][label] and collapsed [text][] forms express clear
// reference intent; a bare [label] is bracketed prose far more often than a
// broken shortcut link, so it is deliberately not reported (a defined
// [label] still resolves and never reaches this list either way).
func (scanner *sourceScanner) undefinedReferences(missing map[string]int) []ReferenceUse {
	remaining := make(map[string]int, len(missing))
	maps.Copy(remaining, missing)
	uses := make([]ReferenceUse, 0)
	for _, candidate := range scanner.referenceUseCandidates() {
		label := NormalizeReferenceLabel(candidate.label)
		if remaining[label] > 0 {
			remaining[label]--
			uses = append(uses, ReferenceUse{Label: candidate.label, Position: candidate.position})
		}
	}
	return uses
}

// referenceUseCandidate is one [text][label] or [text][] occurrence in
// unprotected source.
type referenceUseCandidate struct {
	label    string
	position Position
}

// referenceUseCandidates finds every explicit reference-style use —
// [text][label] and the collapsed [text][], with an optional ! image
// marker — in unprotected source. Bracket matching is nesting-aware for the
// text part (link text may contain balanced brackets) and first-closing for
// the label part, mirroring how Goldmark closes the two bracket groups.
// Escaped brackets never match, and no bracket search crosses a blank line.
func (scanner *sourceScanner) referenceUseCandidates() []referenceUseCandidate {
	candidates := make([]referenceUseCandidate, 0)
	source := scanner.source
	for offset := 0; offset < len(source); {
		switch current := source[offset]; current {
		case '\\':
			// An escaped character can never open a bracket.
			offset += 2
			continue
		case '[':
			if !scanner.protected(offset) {
				textClose, found := matchBracket(source, offset, true)
				if found && textClose+1 < len(source) && source[textClose+1] == '[' && !scanner.protected(textClose+1) {
					labelOpen := textClose + 1
					if labelClose, ok := matchBracket(source, labelOpen, false); ok {
						label := source[labelOpen+1 : labelClose]
						if len(label) == 0 {
							// Collapsed use: the label is the link text itself.
							label = source[offset+1 : textClose]
						}
						if NormalizeReferenceLabel(string(label)) != "" {
							line, column := scanner.locator.locate(offset)
							candidates = append(candidates, referenceUseCandidate{
								label:    string(label),
								position: Position{Line: line, Column: column},
							})
						}
						offset = labelClose + 1
						continue
					}
				}
			}
			offset++
		default:
			offset++
		}
	}
	return candidates
}

// footnoteUseCandidate is one [^label] occurrence in unprotected source.
type footnoteUseCandidate struct {
	label    string
	position Position
}

// undefinedFootnotes returns the located [^label] uses whose label no
// definition matches. Unlike link references, an unresolved footnote leaves
// plain text with no parser hook to observe, so candidates are compared
// against the collected definitions by exact bytes — the comparison
// Goldmark's own footnote parser applies.
func (scanner *sourceScanner) undefinedFootnotes(definitions []Footnote) []FootnoteReference {
	defined := make(map[string]struct{}, len(definitions))
	for _, footnote := range definitions {
		defined[footnote.Label] = struct{}{}
	}
	uses := make([]FootnoteReference, 0)
	for _, candidate := range scanner.footnoteUseCandidates() {
		if _, resolved := defined[candidate.label]; !resolved {
			uses = append(uses, FootnoteReference{Label: candidate.label, Position: candidate.position})
		}
	}
	return uses
}

// footnoteUseCandidates finds every [^label] form in unprotected source.
// The label closes on the same line — the footnote parser never scans past
// a newline — and escaped brackets never match.
func (scanner *sourceScanner) footnoteUseCandidates() []footnoteUseCandidate {
	candidates := make([]footnoteUseCandidate, 0)
	source := scanner.source
	for offset := 0; offset < len(source); offset++ {
		if source[offset] == '\\' {
			offset++
			continue
		}
		if source[offset] != '[' || offset+1 >= len(source) || source[offset+1] != '^' || scanner.protected(offset) {
			continue
		}
		labelStart := offset + 2
		close := bytes.IndexByte(source[labelStart:], ']')
		if close < 0 {
			continue
		}
		close += labelStart
		// The label must close on the same line it opened.
		if bytes.IndexByte(source[labelStart:close], '\n') >= 0 {
			continue
		}
		label := source[labelStart:close]
		if len(bytes.TrimSpace(label)) == 0 {
			continue
		}
		line, column := scanner.locator.locate(offset)
		candidates = append(candidates, footnoteUseCandidate{
			label:    string(label),
			position: Position{Line: line, Column: column},
		})
		offset = close
	}
	return candidates
}

// reversedLinks finds (text)[destination] forms in unprotected source where
// the destination clearly reads as a URL or path. The parenthesized-then-
// bracketed shape also occurs in ordinary prose — f(x)[0], array[index] —
// so only destinations that obviously name a target count, trading recall
// for precision: a vague case unreported beats a correct one falsely
// flagged.
func (scanner *sourceScanner) reversedLinks() []ReversedLink {
	links := make([]ReversedLink, 0)
	source := scanner.source
	for offset := 0; offset < len(source); offset++ {
		switch source[offset] {
		case '\\':
			offset++
		case '(':
			if scanner.protected(offset) {
				continue
			}
			close := bytes.IndexByte(source[offset+1:], ')')
			if close < 0 {
				continue
			}
			close += offset + 1
			// A link typo sits on one line; text spanning lines is prose.
			if bytes.IndexByte(source[offset+1:close], '\n') >= 0 {
				continue
			}
			destinationOpen := close + 1
			if destinationOpen >= len(source) || source[destinationOpen] != '[' || scanner.protected(destinationOpen) {
				continue
			}
			destinationClose := bytes.IndexByte(source[destinationOpen+1:], ']')
			if destinationClose < 0 {
				continue
			}
			destinationClose += destinationOpen + 1
			if bytes.IndexByte(source[destinationOpen+1:destinationClose], '\n') >= 0 {
				continue
			}
			text := bytes.TrimSpace(source[offset+1 : close])
			destination := string(source[destinationOpen+1 : destinationClose])
			if len(text) == 0 || !looksLikeDestination(destination) {
				continue
			}
			line, column := scanner.locator.locate(offset)
			links = append(links, ReversedLink{
				Text:        string(text),
				Destination: destination,
				Position:    Position{Line: line, Column: column},
			})
			offset = destinationClose
		}
	}
	return links
}

// destinationExtensions are the suffixes that make a dot-suffixed token read
// as a file target. A bare dot is prose far more often than a link — version
// numbers like v1.2, section numbers like 1.2.3 — so only known extensions
// count, trading recall for precision like the rest of this heuristic.
var destinationExtensions = map[string]bool{
	"md": true, "markdown": true, "mdx": true,
	"html": true, "htm": true,
	"pdf": true, "epub": true,
	"png": true, "jpg": true, "jpeg": true, "gif": true, "svg": true,
	"webp": true, "bmp": true, "ico": true, "avif": true,
	"js": true, "mjs": true, "ts": true, "css": true,
	"json": true, "yaml": true, "yml": true, "toml": true, "xml": true,
	"csv": true, "tsv": true, "txt": true, "rst": true, "adoc": true, "tex": true,
	"zip": true, "tar": true, "gz": true, "7z": true,
	"doc": true, "docx": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true,
}

// looksLikeDestination reports whether a bracketed value obviously names a
// link target: a known scheme, an anchor, a rooted or relative path, or a
// token with a known file extension.
func looksLikeDestination(destination string) bool {
	switch {
	case destination == "":
		return false
	case strings.HasPrefix(destination, "http://"),
		strings.HasPrefix(destination, "https://"),
		strings.HasPrefix(destination, "mailto:"),
		strings.HasPrefix(destination, "tel:"),
		strings.HasPrefix(destination, "#"),
		strings.HasPrefix(destination, "./"),
		strings.HasPrefix(destination, "../"),
		strings.HasPrefix(destination, "/"):
		return true
	}
	if strings.ContainsAny(destination, " \t") {
		return false
	}
	if strings.Contains(destination, "/") {
		return true
	}
	dot := strings.LastIndex(destination, ".")
	if dot <= 0 {
		return false
	}
	return destinationExtensions[strings.ToLower(destination[dot+1:])]
}

// mojibakePatterns are multi-character UTF-8 signatures of text that was
// decoded as Latin-1 and re-encoded: curly punctuation ("â€™"), accented
// letters ("Ã©"), an embedded BOM ("ï»¿") and the prefix of a truncated
// four-byte emoji ("ðŸ"). Single characters like a lone Ã or Â never
// appear here — they are legitimate letters in several languages.
var mojibakePatterns = []string{
	"â€™", "â€œ", "â€", "â€“", "â€”", "â€¦",
	"Ã©", "Ã¨", "Ã«", "Ã±", "Ã¼", "Ã¶", "Ã¤", "Ã§",
	"Â«", "Â»", "Â°", "Â ",
	"ï»¿",
	"ðŸ",
}

// invisibleNames maps the invisible characters the scan tracks to their
// code point names. ZWJ (U+200D) and variation selectors (U+FE0F) are
// deliberately absent: emoji sequences like ❤️ and 👩‍❤️‍👨 depend on them.
var invisibleNames = map[rune]string{
	0x00AD: "SOFT HYPHEN",
	0x200B: "ZERO WIDTH SPACE",
	0x2060: "WORD JOINER",
	0xFEFF: "ZERO WIDTH NO-BREAK SPACE",
	// Bidi embeddings and overrides can flip or scramble surrounding text —
	// suspicious wherever they appear.
	0x202A: "LEFT-TO-RIGHT EMBEDDING",
	0x202B: "RIGHT-TO-LEFT EMBEDDING",
	0x202C: "POP DIRECTIONAL FORMATTING",
	0x202D: "LEFT-TO-RIGHT OVERRIDE",
	0x202E: "RIGHT-TO-LEFT OVERRIDE",
	0x2066: "LEFT-TO-RIGHT ISOLATE",
	0x2067: "RIGHT-TO-LEFT ISOLATE",
	0x2068: "FIRST STRONG ISOLATE",
	0x2069: "POP DIRECTIONAL ISOLATE",
}

// alwaysSuspicious reports whether an invisible character is suspicious in
// any position: the bidi embedding and override controls.
func alwaysSuspicious(char rune) bool {
	return (char >= 0x202A && char <= 0x202E) || (char >= 0x2066 && char <= 0x2069)
}

// unicodeFindings reports mojibake signatures and suspicious invisible
// characters outside code. Raw HTML is fair game here — the broken bytes
// ship to the browser inside it too — so the scan skips code alone, a wider
// domain than the syntax scans' protected(). Invisible characters only
// count in suspicious positions — line start or end, next to whitespace, or
// in a consecutive run — because legitimate text uses them in far more
// places than attacks do.
func (scanner *sourceScanner) unicodeFindings() ([]Mojibake, []InvisibleCharacter) {
	mojibake := make([]Mojibake, 0)
	for _, pattern := range mojibakePatterns {
		signature := []byte(pattern)
		for offset := bytes.Index(scanner.source, signature); offset >= 0; {
			if !scanner.inCode(offset) {
				line, column := scanner.locator.locate(offset)
				mojibake = append(mojibake, Mojibake{Pattern: pattern, Position: Position{Line: line, Column: column}})
			}
			next := bytes.Index(scanner.source[offset+1:], signature)
			if next < 0 {
				break
			}
			offset += 1 + next
		}
	}

	invisible := make([]InvisibleCharacter, 0)
	source := scanner.source
	for offset, char := range string(source) {
		name, tracked := invisibleNames[char]
		if !tracked || scanner.inCode(offset) {
			continue
		}
		if alwaysSuspicious(char) || scanner.suspiciousPosition(offset, char) {
			line, column := scanner.locator.locate(offset)
			invisible = append(invisible, InvisibleCharacter{Rune: char, Name: name, Position: Position{Line: line, Column: column}})
		}
	}
	return mojibake, invisible
}

// suspiciousPosition reports whether an invisible character at offset sits
// where legitimate text would not put it: the start or end of a line, next
// to whitespace, or directly beside another invisible character.
func (scanner *sourceScanner) suspiciousPosition(offset int, char rune) bool {
	source := scanner.source
	previous, previousWidth := lastRuneBefore(source, offset)
	next, nextWidth := nextRuneAfter(source, offset+len(string(char)))

	if offset == 0 || previous == '\n' {
		return true // line start
	}
	if offset+len(string(char)) >= len(source) || next == '\n' {
		return true // line end
	}
	if previous == ' ' || previous == '\t' || next == ' ' || next == '\t' {
		return true // adjacent whitespace
	}
	if previousWidth > 0 {
		if _, invisible := invisibleNames[previous]; invisible {
			return true
		}
	}
	if nextWidth > 0 {
		if _, invisible := invisibleNames[next]; invisible {
			return true
		}
	}
	return false
}

// lastRuneBefore decodes the rune ending at offset with its byte width.
func lastRuneBefore(source []byte, offset int) (rune, int) {
	if offset <= 0 {
		return 0, 0
	}
	decoded, size := utf8.DecodeLastRune(source[:offset])
	return decoded, size
}

// nextRuneAfter decodes the rune starting at offset with its byte width.
func nextRuneAfter(source []byte, offset int) (rune, int) {
	if offset >= len(source) {
		return 0, 0
	}
	decoded, size := utf8.DecodeRune(source[offset:])
	return decoded, size
}

// matchBracket returns the index of the ']' closing the '[' at open. With
// nesting, inner '[' increase the depth (balanced link text); without it the
// first ']' closes, mirroring Goldmark's non-nesting reference-label
// closure. Backslash escapes skip their character and a blank line ends the
// search, because no inline construct spans paragraphs.
func matchBracket(source []byte, open int, nesting bool) (int, bool) {
	depth := 1
	for offset := open + 1; offset < len(source); offset++ {
		switch source[offset] {
		case '\\':
			offset++
		case '\n':
			if offset+1 < len(source) && source[offset+1] == '\n' {
				return 0, false
			}
		case '[':
			if nesting {
				depth++
			}
		case ']':
			depth--
			if depth == 0 {
				return offset, true
			}
		}
	}
	return 0, false
}

// lineStart returns the offset where the line containing offset begins.
func (locator sourceLocator) lineStart(offset int) int {
	line, _ := locator.locate(offset)
	return locator.lineStarts[line-1]
}

// previousLineStart returns the offset where the line before the one
// containing offset begins.
func (locator sourceLocator) previousLineStart(offset int) int {
	line, _ := locator.locate(offset)
	if line < 2 {
		return 0
	}
	return locator.lineStarts[line-2]
}
