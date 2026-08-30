package check

import (
	"testing"

	"github.com/lz-wang/m2h/internal/markdown"
)

func TestCheckReferenceWarningsDoNotNeedTargetResolver(t *testing.T) {
	t.Parallel()

	current := &indexedDocument{document: document{relative: "guide.md", display: "guide.md"}}
	rules := RuleSet{enabled: map[string]struct{}{RuleImageAltEmpty: {}}}
	diagnostics := checkReference(documentScope{}, nil, nil, current, markdown.Reference{
		Kind:        markdown.ReferenceImage,
		Destination: "missing.png",
		Line:        3,
		Column:      4,
	}, rules)

	if len(diagnostics) != 1 || diagnostics[0].Rule != RuleImageAltEmpty {
		t.Fatalf("diagnostics = %+v, want only %s", diagnostics, RuleImageAltEmpty)
	}
}

func TestDiagnosticForRulePanicsOnUnknownInternalRule(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != "unknown internal check rule: typo.rule" {
			t.Fatalf("panic = %v, want unknown internal rule failure", recovered)
		}
	}()
	current := &indexedDocument{}
	current.diagnosticForRule("typo.rule", "message", markdown.Position{Line: 1, Column: 1})
}
