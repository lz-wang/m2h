package check

import (
	"fmt"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkReferenceRules reports reference-definition integrity the parser has
// already judged: explicit uses whose label no definition resolves, and
// definitions no use ever references. Both facts come from the shared
// inspection, so what these rules see is exactly what rendering sees.
func checkReferenceRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	if rules.Enabled(RuleReferenceUndefined) {
		for _, use := range inspection.UndefinedReferences {
			diagnostics = append(diagnostics, current.diagnosticAt(SeverityError, RuleReferenceUndefined,
				fmt.Sprintf("reference label %q is not defined", use.Label), use.Position))
		}
	}
	if rules.Enabled(RuleReferenceUnused) {
		used := make(map[string]struct{}, len(inspection.References))
		for _, reference := range inspection.References {
			if reference.ReferenceLabel == "" {
				continue
			}
			used[markdown.NormalizeReferenceLabel(reference.ReferenceLabel)] = struct{}{}
		}
		for _, definition := range inspection.ReferenceDefinitions {
			if _, referenced := used[markdown.NormalizeReferenceLabel(definition.Label)]; !referenced {
				diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleReferenceUnused,
					fmt.Sprintf("reference definition %q is never used", definition.Label), definition.Position))
			}
		}
	}
	return diagnostics
}
