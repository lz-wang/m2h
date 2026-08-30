package check

import "fmt"

// checkUnicodeRules reports encoding problems outside code, both opt-in
// because their false-positive surface is real: mojibake by multi-character
// signature (a lone Ã or Â is a legitimate letter), and invisible characters
// only in suspicious positions (ZWJ and variation selectors never count —
// emoji depend on them).
func checkUnicodeRules(current *indexedDocument, rules RuleSet) []Diagnostic {
	inspection := &current.inspection
	diagnostics := make([]Diagnostic, 0)
	if rules.Enabled(RuleUnicodeMojibake) {
		for _, finding := range inspection.Mojibake {
			diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleUnicodeMojibake,
				fmt.Sprintf("suspicious mojibake %q", finding.Pattern), finding.Position))
		}
	}
	if rules.Enabled(RuleUnicodeInvisibleCharacter) {
		for _, finding := range inspection.InvisibleCharacters {
			diagnostics = append(diagnostics, current.diagnosticAt(SeverityWarning, RuleUnicodeInvisibleCharacter,
				fmt.Sprintf("suspicious invisible character U+%04X %s", finding.Rune, finding.Name),
				finding.Position))
		}
	}
	return diagnostics
}
