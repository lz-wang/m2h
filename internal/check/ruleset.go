package check

import "fmt"

// RuleDefinition is the static metadata of one check rule: its stable id, the
// severity its diagnostics carry and whether it runs without being named on
// the command line. Message texts stay at the emission sites — a message
// describes the individual finding ("heading level jumps from H1 to H4"), not
// the rule — so this registry only holds data every finding of the rule
// shares.
type RuleDefinition struct {
	ID             string
	Severity       Severity
	DefaultEnabled bool
}

// ruleDefinitions lists every rule the checker knows. Rule ids are the stable
// contract CI and tooling consume and are never renamed; new rules join the
// list in the order they should surface in documentation.
var ruleDefinitions = []RuleDefinition{
	{ID: RuleFrontMatterInvalid, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleLocalTargetMissing, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleLocalTargetNotRegular, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleLocalTargetOutsideRoot, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleMarkdownTargetNotServed, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleAnchorMissing, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleReferenceUndefined, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleFootnoteUndefined, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleFootnoteEmpty, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleTableColumnMismatch, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleHTMLCommentUnclosed, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleLinkReversed, Severity: SeverityError, DefaultEnabled: true},
	{ID: RuleImageAltEmpty, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleDocumentMultipleH1, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleHeadingLevelSkip, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleHeadingDuplicate, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleCodeFenceLanguageMissing, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleFrontMatterDateInvalid, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleLinkEmptyDestination, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleReferenceUnused, Severity: SeverityWarning, DefaultEnabled: true},
	{ID: RuleFootnoteUnused, Severity: SeverityWarning, DefaultEnabled: true},
}

// ruleAll is the special rule name that addresses every known rule, so
// "--enable all" widens the run and "--disable all --enable section.empty"
// narrows it to exactly one rule.
const ruleAll = "all"

// RuleSet is the resolved selection of rules one check run executes.
type RuleSet struct {
	enabled map[string]struct{}
}

// NewRuleSet resolves the rule selection for one run: the default rules, plus
// the rules named by enable, minus the rules named by disable — disable always
// wins. An unknown name fails here, before any file is read, because a typo
// silently changing what a CI run checks is worse than a stopped run.
func NewRuleSet(enable []string, disable []string) (RuleSet, error) {
	set := DefaultRuleSet()
	for _, name := range enable {
		if err := set.add(name); err != nil {
			return RuleSet{}, err
		}
	}
	for _, name := range disable {
		if err := set.remove(name); err != nil {
			return RuleSet{}, err
		}
	}
	return set, nil
}

// DefaultRuleSet selects every rule whose definition is enabled by default.
func DefaultRuleSet() RuleSet {
	enabled := make(map[string]struct{}, len(ruleDefinitions))
	for _, definition := range ruleDefinitions {
		if definition.DefaultEnabled {
			enabled[definition.ID] = struct{}{}
		}
	}
	return RuleSet{enabled: enabled}
}

// Enabled reports whether the rule should run.
func (set RuleSet) Enabled(rule string) bool {
	_, ok := set.enabled[rule]
	return ok
}

// add enables one named rule, or every rule for the special name "all".
func (set *RuleSet) add(name string) error {
	if name == ruleAll {
		for _, definition := range ruleDefinitions {
			set.enabled[definition.ID] = struct{}{}
		}
		return nil
	}
	if _, ok := ruleDefinition(name); !ok {
		return unknownRuleError(name)
	}
	set.enabled[name] = struct{}{}
	return nil
}

// remove disables one named rule, or every rule for the special name "all".
func (set *RuleSet) remove(name string) error {
	if name == ruleAll {
		clear(set.enabled)
		return nil
	}
	if _, ok := ruleDefinition(name); !ok {
		return unknownRuleError(name)
	}
	delete(set.enabled, name)
	return nil
}

// ruleDefinition looks one rule up by id.
func ruleDefinition(id string) (RuleDefinition, bool) {
	for _, definition := range ruleDefinitions {
		if definition.ID == id {
			return definition, true
		}
	}
	return RuleDefinition{}, false
}

// unknownRuleError names the rule the registry does not know, unprefixed, so
// the CLI renders it exactly as `unknown check rule "foo.bar"` — the rule name
// is the actionable part of the message.
func unknownRuleError(name string) error {
	return fmt.Errorf("unknown check rule %q", name)
}
