package check

import "fmt"

// checkSyntaxRules reports malformed syntax the renderer passes through or
// drops: HTML comment blocks that never close and swallow the rest of the
// document, reversed Markdown link syntax that renders as literal
// parentheses, and fenced code blocks without a language, which the
// highlighter cannot style.
func checkSyntaxRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	if rules.Enabled(RuleHTMLCommentUnclosed) {
		for _, comment := range inspection.UnclosedComments {
			diagnostics = append(diagnostics, current.diagnosticForRule(RuleHTMLCommentUnclosed,
				"unclosed HTML comment; everything after it renders as comment content", comment.Position))
		}
	}
	if rules.Enabled(RuleLinkReversed) {
		for _, link := range inspection.ReversedLinks {
			diagnostics = append(diagnostics, current.diagnosticForRule(RuleLinkReversed,
				fmt.Sprintf("looks like reversed Markdown link syntax; use [%s](%s)", link.Text, link.Destination),
				link.Position))
		}
	}
	if rules.Enabled(RuleCodeFenceLanguageMissing) {
		for _, fence := range inspection.CodeFences {
			if fence.Language == "" {
				diagnostics = append(diagnostics, current.diagnosticForRule(RuleCodeFenceLanguageMissing,
					"fenced code block has no language", fence.Position))
			}
		}
	}
	return diagnostics
}
