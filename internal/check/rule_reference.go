package check

import (
	"fmt"
	"strings"

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
	if !rules.Enabled(RuleLinkTextNondescriptive) {
		return diagnostics
	}
	for _, reference := range inspection.References {
		// Only real Markdown links carry screen-reader-visible text worth
		// judging; image alt, raw HTML anchors and autolinks are out of
		// scope. Exact matches only — "click here to read the API docs"
		// carries real information and must stay silent.
		if _, generic := nondescriptiveLinkText[lowerTrim(reference.Text)]; reference.Kind != markdown.ReferenceLink || !generic {
			continue
		}
		diagnostics = append(diagnostics, current.diagnostic(SeverityWarning, RuleLinkTextNondescriptive,
			fmt.Sprintf("link text %q is not descriptive", reference.Text), reference))
	}
	return diagnostics
}

// nondescriptiveLinkText is the exact-match word list of generic link
// texts, lowercased for comparison. Deliberately tiny: anything beyond
// exact matches starts guessing at intent.
var nondescriptiveLinkText = map[string]struct{}{
	"here":       {},
	"click here": {},
	"link":       {},
	"this link":  {},
	"more":       {},
	"read more":  {},
	"details":    {},
	"这里":         {},
	"点击这里":       {},
	"点这里":        {},
	"链接":         {},
	"更多":         {},
	"详情":         {},
	"查看详情":       {},
}

func lowerTrim(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
