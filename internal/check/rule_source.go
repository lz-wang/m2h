package check

import (
	"fmt"
	"unicode/utf8"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkInvalidUTF8 reports the first byte of the first invalid UTF-8 sequence
// in the raw source. One finding per file is enough: every invalid byte has
// the same fix (repair the encoding), and a corrupted file typically carries
// hundreds of them. Line and column follow the checker convention — 1-based,
// with the column counting source bytes, not display width.
func checkInvalidUTF8(current *indexedDocument, source []byte, rules RuleSet) []Diagnostic {
	if !rules.Enabled(RuleDocumentInvalidUTF8) {
		return nil
	}
	offset := firstInvalidUTF8Offset(source)
	if offset < 0 {
		return nil
	}
	line, column := 1, 1
	for index := range offset {
		if source[index] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return []Diagnostic{current.diagnosticForRule(RuleDocumentInvalidUTF8,
		fmt.Sprintf("document contains an invalid UTF-8 byte sequence (0x%02X)", source[offset]),
		markdown.Position{Line: line, Column: column})}
}

// firstInvalidUTF8Offset returns the byte offset of the first invalid UTF-8
// sequence, or -1 when the whole source is valid. RuneError with size 1 marks
// an actual encoding error; a legal U+FFFD in the input decodes as itself
// with size 3 and must not be reported.
func firstInvalidUTF8Offset(source []byte) int {
	for offset := 0; offset < len(source); {
		r, size := utf8.DecodeRune(source[offset:])
		if r == utf8.RuneError && size == 1 {
			return offset
		}
		offset += size
	}
	return -1
}
