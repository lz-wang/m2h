package check

import (
	"context"
	"slices"
	"testing"
)

func TestDefaultRuleSetEnablesEveryDefaultRule(t *testing.T) {
	t.Parallel()

	set := DefaultRuleSet()
	for _, definition := range ruleDefinitions {
		if !definition.DefaultEnabled {
			continue
		}
		if !set.Enabled(definition.ID) {
			t.Errorf("default rule set does not enable %q", definition.ID)
		}
	}
}

func TestRuleDefinitionsAreWellFormed(t *testing.T) {
	t.Parallel()

	seen := make(map[string]int, len(ruleDefinitions))
	for _, definition := range ruleDefinitions {
		if definition.ID == "" {
			t.Errorf("rule definition without id: %+v", definition)
		}
		switch definition.Severity {
		case SeverityError, SeverityWarning:
		default:
			t.Errorf("rule %q has unknown severity %q", definition.ID, definition.Severity)
		}
		seen[definition.ID]++
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("rule %q defined %d times, want exactly once", id, count)
		}
	}
}

func TestNewRuleSetSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enable  []string
		disable []string
		want    map[string]bool
	}{
		{
			name:    "disable removes one default",
			enable:  nil,
			disable: []string{RuleImageAltEmpty},
			want:    map[string]bool{RuleImageAltEmpty: false, RuleAnchorMissing: true},
		},
		{
			name:    "enable re-adds a disabled default",
			enable:  []string{RuleImageAltEmpty},
			disable: []string{RuleImageAltEmpty},
			want:    map[string]bool{RuleImageAltEmpty: false},
		},
		{
			name:    "enable all minus one",
			enable:  []string{"all"},
			disable: []string{RuleFrontMatterDateInvalid},
			want:    map[string]bool{RuleFrontMatterDateInvalid: false, RuleAnchorMissing: true},
		},
		{
			name:    "disable all empties the run",
			enable:  nil,
			disable: []string{"all"},
			want: map[string]bool{
				RuleFrontMatterInvalid: false,
				RuleAnchorMissing:      false,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			set, err := NewRuleSet(test.enable, test.disable)
			if err != nil {
				t.Fatalf("NewRuleSet() error = %v", err)
			}
			for rule, want := range test.want {
				if got := set.Enabled(rule); got != want {
					t.Errorf("Enabled(%q) = %v, want %v", rule, got, want)
				}
			}
		})
	}
}

func TestNewRuleSetRejectsUnknownRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enable  []string
		disable []string
		want    string
	}{
		{
			name:   "unknown enable",
			enable: []string{"foo.bar"},
			want:   `unknown check rule "foo.bar"`,
		},
		{
			name:    "unknown disable",
			disable: []string{"nope.nope"},
			want:    `unknown check rule "nope.nope"`,
		},
		{
			name:   "empty name from a trailing comma",
			enable: []string{RuleAnchorMissing, ""},
			want:   `unknown check rule ""`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRuleSet(test.enable, test.disable)
			if err == nil || err.Error() != test.want {
				t.Fatalf("NewRuleSet() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunRejectsUnknownRuleNamesBeforeFilesystem(t *testing.T) {
	t.Parallel()

	// The rule name must fail even when the input path itself is unusable:
	// validation happens before any file is read.
	_, err := Run(context.Background(), Options{Input: "does-not-exist.md", EnableRules: []string{"foo.bar"}})
	if err == nil || err.Error() != `unknown check rule "foo.bar"` {
		t.Fatalf("Run() error = %v, want the unknown rule error", err)
	}
}

func TestRunDisabledRulesStaySilent(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		"guide.md": "# Guide\n\n# Guide\n\n![alt](missing.png)\n\n![](missing.png)\n",
	}

	defaults, err := runCheck(t, sources, Options{})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	defaultRules := slices.DeleteFunc(slices.Clone(defaults.Diagnostics), func(diagnostic Diagnostic) bool {
		return diagnostic.Rule != RuleDocumentMultipleH1 && diagnostic.Rule != RuleImageAltEmpty
	})
	if len(defaultRules) != 2 {
		t.Fatalf("default diagnostics = %+v, want one multiple-h1 and one alt-empty", defaultRules)
	}

	silenced, err := runCheck(t, sources, Options{DisableRules: []string{"all"}})
	if err != nil {
		t.Fatalf("Run() with --disable all returned error: %v", err)
	}
	if len(silenced.Diagnostics) != 0 {
		t.Fatalf("diagnostics with --disable all = %+v, want none", silenced.Diagnostics)
	}
}
