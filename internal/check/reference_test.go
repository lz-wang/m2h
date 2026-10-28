package check

import (
	"testing"

	"github.com/lz-wang/m2h/internal/markdown"
)

func TestCheckReferenceWarningsDoNotNeedTargetResolver(t *testing.T) {
	t.Parallel()

	current := &indexedDocument{document: document{relative: "guide.md", display: "guide.md"}}
	rules := RuleSet{enabled: map[string]struct{}{RuleImageAltEmpty: {}}}
	diagnostics, err := checkReference(documentScope{}, nil, nil, current, markdown.Reference{
		Kind:        markdown.ReferenceImage,
		Destination: "missing.png",
		Line:        3,
		Column:      4,
	}, rules)
	if err != nil {
		t.Fatal(err)
	}

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

func TestDirectoryLinksShareDocumentSelection(t *testing.T) {
	t.Parallel()
	result, err := runCheck(t, map[string]string{
		"index.md":        "# Index\n\n[Topic](topic/#intro) <a href=\"topic/\">Topic</a> [No slash](topic)\n",
		"topic/README.md": "# Intro\n",
		"topic/other.md":  "# Other\n",
	}, Options{Depth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("directory links rejected: %+v", result.Diagnostics)
	}
	result, err = runCheck(t, map[string]string{
		"index.md":        "# Index\n\n[Topic](topic/#missing)\n",
		"topic/README.md": "# Intro\n",
	}, Options{Depth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Rule != RuleAnchorMissing {
		t.Fatalf("anchor not checked: %+v", result.Diagnostics)
	}
}

func TestTabbedReferencesAndDuplicateAnchors(t *testing.T) {
	result, err := runCheck(t, map[string]string{
		"index.md": "# Index\n\n[second](tabs.md#安装-1)\n",
		"tabs.md":  "# Tabs\n\n=== \"One\"\n\n    ## 安装\n\n    [missing](missing.md)\n\n=== \"Two\"\n\n    ## 安装\n\n    ```text\n    [ignored](ignored.md)\n    ```\n",
	}, Options{Depth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 2 || result.Diagnostics[0].Rule != RuleLocalTargetMissing || result.Diagnostics[0].Line != 7 || result.Diagnostics[1].Rule != RuleHeadingDuplicate {
		t.Fatalf("diagnostics: %+v", result.Diagnostics)
	}
}

func TestResponsiveImageCandidatesShareAssetChecks(t *testing.T) {
	result, err := runCheck(t, map[string]string{
		"index.md": "# Images\n\n<picture><source srcset=\"ok.png 1x, missing.png 2x\"><img alt=\"Image\" srcset=\"data:image/png;base64,AAAA,BBBB 1x, ../outside.png 2x\"></picture>\n",
		"ok.png":   "image",
	}, Options{Depth: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 2 || result.Diagnostics[0].Rule != RuleLocalTargetMissing || result.Diagnostics[1].Rule != RuleLocalTargetOutsideRoot {
		t.Fatalf("candidate checks: %+v", result.Diagnostics)
	}
}
