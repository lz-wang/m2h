package check

import (
	"bytes"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkTrailingBlankLines requires one terminal LF/CRLF, as displayed by an
// editor with a final empty cursor line. Additional whitespace-only lines are
// rejected; the empty split field after the terminal newline is not a line.
func checkTrailingBlankLines(current *indexedDocument, source []byte, rules RuleSet) []Diagnostic {
	if !rules.Enabled(RuleDocumentTrailingBlankLines) {
		return nil
	}
	lines := bytes.Split(source, []byte{'\n'})
	if len(source) == 0 {
		lines = nil
	}
	terminated := len(source) > 0 && source[len(source)-1] == '\n'
	if terminated {
		lines = lines[:len(lines)-1]
	}
	blank := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if len(bytes.Trim(lines[i], " \t\r")) != 0 {
			break
		}
		blank++
	}
	if terminated && (blank == 0 || bytes.Equal(source, []byte("\n")) || bytes.Equal(source, []byte("\r\n"))) {
		return nil
	}
	position := markdown.Position{Line: len(lines), Column: 1}
	if position.Line < 1 {
		position.Line = 1
	}
	if len(lines) > 0 && !terminated {
		position.Column = len(bytes.TrimSuffix(lines[len(lines)-1], []byte{'\r'})) + 1
	}
	return []Diagnostic{current.diagnosticForRule(RuleDocumentTrailingBlankLines,
		"document must end with a single newline (LF or CRLF), without trailing blank lines", position)}
}
