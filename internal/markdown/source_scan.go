package markdown

import (
	"bytes"
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

// sourceScanner walks one document's source once, recovering syntax the
// final AST can no longer represent: uses the parser rejected and fence
// lines the code block AST strips away. It never re-parses what the AST
// still knows — headings, resolved references and tables come from the AST —
// and never reports anything inside code, because code is literal text no
// parser ever interpreted.
type sourceScanner struct {
	source     []byte
	locator    sourceLocator
	code       []byteRange // AST code ranges, sorted by start
	fences     []CodeFence
	fenceLines []byteRange // the fence opener/closer lines themselves
}

// CodeFence is one fenced code block seen at line level. The AST keeps the
// content lines but strips the fence lines, so the language and the fence
// position come from the scan. Position is the opening fence line, column 1.
type CodeFence struct {
	Language string
	Position Position
}

// newSourceScanner prepares a scan over source, skipping the protected code
// ranges the AST identified.
func newSourceScanner(source []byte, code []byteRange) *sourceScanner {
	scanner := &sourceScanner{
		source:  source,
		locator: newSourceLocator(source),
		code:    code,
	}
	sort.Slice(scanner.code, func(left, right int) bool {
		return scanner.code[left].start < scanner.code[right].start
	})
	scanner.scanFences()
	return scanner
}

// protected reports whether offset sits inside a code range or a fence line,
// where nothing the scanner looks for can be real syntax.
func (scanner *sourceScanner) protected(offset int) bool {
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

// undefinedReferences returns the located reference uses whose labels the
// parser rejected, in source order. Only the explicit [text][label] and
// collapsed [text][] forms express clear reference intent; a bare [label]
// is bracketed prose far more often than a broken shortcut link, so it is
// deliberately not reported (a defined [label] still resolves and never
// reaches this list either way).
func (scanner *sourceScanner) undefinedReferences(missing map[string]struct{}) []ReferenceUse {
	uses := make([]ReferenceUse, 0)
	for _, candidate := range scanner.referenceUseCandidates() {
		if _, rejected := missing[NormalizeReferenceLabel(candidate.label)]; rejected {
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

// looksLikeDestination reports whether a bracketed value obviously names a
// link target: a known scheme, an anchor, a rooted or relative path, or a
// file-like token with an extension.
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
	return dot > 0
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
// characters outside code. Invisible characters only count in suspicious
// positions — line start or end, next to whitespace, or in a consecutive
// run — because legitimate text uses them in far more places than attacks
// do.
func (scanner *sourceScanner) unicodeFindings() ([]Mojibake, []InvisibleCharacter) {
	mojibake := make([]Mojibake, 0)
	for _, pattern := range mojibakePatterns {
		signature := []byte(pattern)
		for offset := bytes.Index(scanner.source, signature); offset >= 0; {
			if !scanner.protected(offset) {
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
		if !tracked || scanner.protected(offset) {
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

// scanFences tracks fenced code blocks line by line. Fence lines open with
// three or more backticks or tildes at up to three spaces of indentation
// and close with a run of the same character at least as long followed by
// nothing but spaces; the fence lines themselves become protected ranges
// (their info strings are literal text), while the content between them is
// already covered by the AST code ranges.
func (scanner *sourceScanner) scanFences() {
	var fenceChar byte
	fenceLength := 0
	lineStart := 0
	for position := 0; position <= len(scanner.source); position++ {
		if position != len(scanner.source) && scanner.source[position] != '\n' {
			continue
		}
		line := scanner.source[lineStart:position]
		char, runStart, runLength := fenceMarker(line)
		if fenceLength > 0 {
			if char == fenceChar && runLength >= fenceLength && util.IsBlank(line[runStart+runLength:]) {
				scanner.fenceLines = append(scanner.fenceLines, byteRange{start: lineStart, stop: position})
				fenceChar, fenceLength = 0, 0
			}
		} else if char != 0 && runLength >= 3 {
			fenceChar, fenceLength = char, runLength
			scanner.fenceLines = append(scanner.fenceLines, byteRange{start: lineStart, stop: position})
			lineNumber, column := scanner.locator.locate(lineStart)
			scanner.fences = append(scanner.fences, CodeFence{
				Language: fenceLanguage(line, runStart+runLength),
				Position: Position{Line: lineNumber, Column: column},
			})
		}
		lineStart = position + 1
	}
}

// fenceMarker reports the fence character of a line, the byte where its run
// starts, and the run's length — or (0, 0, 0) when the line does not begin
// with a fence run after up to three leading spaces.
func fenceMarker(line []byte) (char byte, runStart int, runLength int) {
	indent := 0
	for indent < len(line) && line[indent] == ' ' && indent < 4 {
		indent++
	}
	if indent > 3 || indent >= len(line) {
		return 0, 0, 0
	}
	char = line[indent]
	if char != '`' && char != '~' {
		return 0, 0, 0
	}
	runStart = indent
	for runStart+runLength < len(line) && line[runStart+runLength] == char {
		runLength++
	}
	return char, runStart, runLength
}

// fenceLanguage extracts the fence's language: the first word of the info
// string after the opening run, the way Goldmark's
// FencedCodeBlock.Language does.
func fenceLanguage(line []byte, infoStart int) string {
	info := bytes.TrimSpace(line[min(infoStart, len(line)):])
	if index := bytes.IndexByte(info, ' '); index >= 0 {
		info = info[:index]
	}
	return string(info)
}
