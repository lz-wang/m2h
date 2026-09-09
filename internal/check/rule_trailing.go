package check

import (
	"bytes"
	"fmt"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkTrailingBlankLines counts physical blank lines after the final content
// line. The empty split field after a terminal newline is not itself a line.
// LF and CRLF are equivalent; a whitespace-only line counts as a blank line.
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
	if blank == 1 && terminated {
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
		fmt.Sprintf("document must end with exactly one blank line (found %d; final newline: %t)", blank, terminated), position)}
}
