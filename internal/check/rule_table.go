package check

import (
	"fmt"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkTableRules reports column-count disagreements: data rows the renderer
// would silently pad or truncate, and header/delimiter pairs whose mismatch
// made the parser reject the table outright — in that case nothing tabular
// renders at all, which is why the rule is an error.
func checkTableRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	if !rules.Enabled(RuleTableColumnMismatch) {
		return nil
	}
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	for _, mismatch := range inspection.TableMismatches {
		message := fmt.Sprintf("table row has %d columns; expected %d", mismatch.Actual, mismatch.Expected)
		if mismatch.Kind == markdown.TableMismatchDelimiter {
			message = fmt.Sprintf("table delimiter has %d columns; header has %d", mismatch.Actual, mismatch.Expected)
		}
		diagnostics = append(diagnostics, current.diagnosticAt(SeverityError, RuleTableColumnMismatch, message, mismatch.Position))
	}
	return diagnostics
}
