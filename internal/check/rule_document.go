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
	diagnostics = append(diagnostics, checkDuplicateHeadings(current, rules)...)
	diagnostics = append(diagnostics, checkReferenceRules(current, rules)...)
	diagnostics = append(diagnostics, checkFootnoteRules(current, rules)...)
	diagnostics = append(diagnostics, checkTableRules(current, rules)...)
	diagnostics = append(diagnostics, checkSyntaxRules(current, rules)...)
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

// section is one open heading of the duplicate-heading stack, owning the
// normalized texts of its direct child headings.
type section struct {
	level    int
	children map[string]struct{}
}

// checkDuplicateHeadings warns when two sibling headings — direct children
// of the same section — carry the same normalized visible text. The same
// text under different sections stays legitimate ("Usage" under both Client
// and Server), and duplicate H1s are left to document.multiple-h1 so one
// problem never yields two warnings.
func checkDuplicateHeadings(current *indexedDocument, rules RuleSet) []Diagnostic {
	if !rules.Enabled(RuleHeadingDuplicate) {
		return nil
	}
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	stack := []section{{level: 0, children: make(map[string]struct{})}}
	for _, heading := range inspection.Headings {
		for len(stack) > 1 && stack[len(stack)-1].level >= heading.Level {
			stack = stack[:len(stack)-1]
		}
		parent := &stack[len(stack)-1]
		if heading.Level > 1 {
			if _, duplicate := parent.children[heading.Text]; duplicate {
				diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleHeadingDuplicate,
					fmt.Sprintf("duplicate heading %q in the same section", heading.Text),
					markdown.Position{Line: heading.Line, Column: 1}))
			} else {
				parent.children[heading.Text] = struct{}{}
			}
		}
		stack = append(stack, section{level: heading.Level, children: make(map[string]struct{})})
	}
	return diagnostics
}
