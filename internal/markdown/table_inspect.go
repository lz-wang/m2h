package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"
	extensionast "github.com/yuin/goldmark/extension/ast"
)

// extractTableMismatches reports every column-count disagreement: data rows
// of tables the parser accepted — the renderer pads short rows and truncates
// long ones, so neither extreme survives as written — plus header/delimiter
// pairs whose counts differ, which make the parser reject the table
// entirely. Counting happens on the row's own source line with the same pipe
// semantics the table parser applies, never on the final AST cells.
func extractTableMismatches(document ast.Node, source []byte) []TableColumnMismatch {
	mismatches := make([]TableColumnMismatch, 0)
	locator := newSourceLocator(source)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *extensionast.Table:
			mismatches = appendRowMismatches(mismatches, typed, source, locator)
		case *ast.Paragraph:
			mismatches = appendDelimiterMismatches(mismatches, typed, source, locator)
		}
		return ast.WalkContinue, nil
	})
	return mismatches
}

// appendRowMismatches recounts every row of one accepted table — header
// included, because the parser pads a short header with empty cells to fit
// the delimiter and never re-reports it — against the column count the
// delimiter row established.
func appendRowMismatches(
	mismatches []TableColumnMismatch,
	table *extensionast.Table,
	source []byte,
	locator sourceLocator,
) []TableColumnMismatch {
	expected := len(table.Alignments)
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		// Data rows carry their own source position; the header kept the
		// position of the row it absorbed.
		var rowPosition int
		switch typed := child.(type) {
		case *extensionast.TableRow:
			rowPosition = typed.Pos()
		case *extensionast.TableHeader:
			rowPosition = typed.Pos()
		default:
			continue
		}
		actual := countTableColumns(lineAt(source, rowPosition))
		if actual != expected {
			line, column := locator.locate(max(rowPosition, 0))
			mismatches = append(mismatches, TableColumnMismatch{
				Kind:     TableMismatchRow,
				Expected: expected,
				Actual:   actual,
				Position: Position{Line: line, Column: column},
			})
		}
	}
	return mismatches
}

// appendDelimiterMismatches inspects the lines a paragraph kept after table
// conversion: any delimiter row still sitting in a paragraph belongs to a
// table attempt the parser rejected, and when its column count differs from
// the header line above, the mismatch is the reason nothing rendered.
// Setext rules and thematic breaks never reach this path — they close the
// paragraph before transformers run — so the paragraph context alone keeps
// this precise.
func appendDelimiterMismatches(
	mismatches []TableColumnMismatch,
	paragraph *ast.Paragraph,
	source []byte,
	locator sourceLocator,
) []TableColumnMismatch {
	lines := paragraph.Lines()
	for index := 1; index < lines.Len(); index++ {
		delimiterSegment := lines.At(index)
		delimiters, isDelimiter := delimiterRowColumns(delimiterSegment.Value(source))
		if !isDelimiter {
			continue
		}
		headerSegment := lines.At(index - 1)
		header := countTableColumns(headerSegment.Value(source))
		if header != delimiters {
			line, column := locator.locate(delimiterSegment.Start)
			mismatches = append(mismatches, TableColumnMismatch{
				Kind:     TableMismatchDelimiter,
				Expected: header,
				Actual:   delimiters,
				Position: Position{Line: line, Column: column},
			})
		}
	}
	return mismatches
}

// countTableColumns counts the cells a table row line declares, with the
// pipe semantics Goldmark's row parser applies: one leading and one trailing
// pipe never bound a cell, a backslash-escaped pipe never separates cells,
// and a bare pipe inside a code span still does.
func countTableColumns(line []byte) int {
	content := bytes.TrimSpace(line)
	if len(content) == 0 {
		return 0
	}
	if content[0] == '|' {
		content = content[1:]
	}
	if len(content) > 0 && content[len(content)-1] == '|' {
		content = content[:len(content)-1]
	}
	if len(content) == 0 {
		return 0
	}
	columns := 1
	for index := 1; index < len(content); index++ {
		if content[index] == '|' && content[index-1] != '\\' {
			columns++
		}
	}
	return columns
}

// delimiter alignment patterns, mirroring the four Goldmark delimiter cells.
var (
	delimiterLeft   = regexp.MustCompile(`^\s*:-+\s*$`)
	delimiterRight  = regexp.MustCompile(`^\s*-+:\s*$`)
	delimiterCenter = regexp.MustCompile(`^\s*:-+:\s*$`)
	delimiterNone   = regexp.MustCompile(`^\s*-+\s*$`)
)

// delimiterRowColumns reports how many columns a valid GFM delimiter row
// declares, or (!ok) when the line is not a delimiter row at all. Validity
// follows the parser: only spaces, dashes, pipes and colons, at least one
// dash, and every cell matching one of the four alignment forms.
func delimiterRowColumns(line []byte) (int, bool) {
	trimmed := bytes.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return 0, false
	}
	allDashes := true
	for _, current := range trimmed {
		switch current {
		case '-':
		case ' ', '\t', '\r', '\n', '|', ':':
			allDashes = false
		default:
			return 0, false
		}
	}
	if allDashes {
		// A run of dashes alone is a setext rule or thematic break, never a
		// delimiter row.
		return 0, false
	}
	cells := bytes.Split(trimmed, []byte{'|'})
	if len(cells) > 0 && len(bytes.TrimSpace(cells[0])) == 0 {
		cells = cells[1:]
	}
	if len(cells) > 0 && len(bytes.TrimSpace(cells[len(cells)-1])) == 0 {
		cells = cells[:len(cells)-1]
	}
	if len(cells) == 0 {
		return 0, false
	}
	for _, cell := range cells {
		if !(delimiterLeft.Match(cell) || delimiterRight.Match(cell) ||
			delimiterCenter.Match(cell) || delimiterNone.Match(cell)) {
			return 0, false
		}
	}
	return len(cells), true
}

// lineAt returns the source line containing offset, without its newline.
func lineAt(source []byte, offset int) []byte {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(source) {
		return nil
	}
	stop := bytes.IndexByte(source[offset:], '\n')
	if stop < 0 {
		return source[offset:]
	}
	return source[offset : offset+stop]
}
