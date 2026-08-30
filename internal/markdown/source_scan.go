package markdown

import (
	"bytes"
	"sort"

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
		if _, rejected := missing[normalizeReferenceLabel([]byte(candidate.label))]; rejected {
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
						if normalizeReferenceLabel(label) != "" {
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
