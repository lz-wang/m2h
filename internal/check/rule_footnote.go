package check

import "fmt"

// checkFootnoteRules reports footnote integrity: markers whose definition is
// missing, definitions no marker references, and definitions without
// content. All three facts come from the pre-transform footnote observation,
// so what these rules see is exactly what the renderer prunes or emits.
func checkFootnoteRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	if rules.Enabled(RuleFootnoteUndefined) {
		for _, use := range inspection.UndefinedFootnotes {
			diagnostics = append(diagnostics, current.diagnosticAt(SeverityError, RuleFootnoteUndefined,
				fmt.Sprintf("footnote [^%s] is not defined", use.Label), use.Position))
		}
	}
	if rules.Enabled(RuleFootnoteEmpty) {
		for _, footnote := range inspection.Footnotes {
			if footnote.Empty {
				diagnostics = append(diagnostics, current.diagnosticAt(SeverityError, RuleFootnoteEmpty,
					fmt.Sprintf("footnote [^%s] has no content", footnote.Label), footnote.Position))
			}
		}
	}
	if rules.Enabled(RuleFootnoteUnused) {
		for _, footnote := range inspection.Footnotes {
			if !footnote.Used {
				diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleFootnoteUnused,
					fmt.Sprintf("footnote [^%s] is never referenced", footnote.Label), footnote.Position))
			}
		}
	}
	return diagnostics
}
