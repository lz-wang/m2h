package check

import (
	"fmt"

	"github.com/lz-wang/m2h/internal/markdown"
)

// checkDocumentRules runs every rule that examines one document's own
// Markdown — heading structure, reference definitions, footnotes — against
// the indexed facts. It never touches the filesystem: resolving references
// against targets is checkDocumentReferences' separate, second-phase job,
// so per-document rules stay cheap, pure and independently testable.
func checkDocumentRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	diagnostics = append(diagnostics, checkHeadingRules(current, rules)...)
	diagnostics = append(diagnostics, checkReferenceRules(current, rules)...)
	diagnostics = append(diagnostics, checkFootnoteRules(current, rules)...)
	return diagnostics
}

// checkHeadingRules reports document structure problems: more than one H1
// (the sidebar and title use only the first) and headings that skip levels
// on the way down.
func checkHeadingRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	if inspection.H1Count > 1 && rules.Enabled(RuleDocumentMultipleH1) {
		diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleDocumentMultipleH1,
			fmt.Sprintf("document contains %d H1 headings", inspection.H1Count),
			markdown.Position{Line: secondH1Line(inspection.Headings), Column: 1}))
	}
	if !rules.Enabled(RuleHeadingLevelSkip) {
		return diagnostics
	}
	// Only downward skips warn: closing sections by any number of levels is
	// legal, and a document that merely starts deep (its first heading is an
	// H3) is a style choice another rule could cover, not a broken outline.
	for index := 1; index < len(inspection.Headings); index++ {
		previous, heading := inspection.Headings[index-1], inspection.Headings[index]
		if heading.Level > previous.Level+1 {
			diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleHeadingLevelSkip,
				fmt.Sprintf("heading level jumps from H%d to H%d", previous.Level, heading.Level),
				markdown.Position{Line: heading.Line, Column: 1}))
		}
	}
	return diagnostics
}
