package check

import (
	"fmt"
	"slices"
	"testing"
)

// expectDiagnostics runs one check over a single-document workspace and
// asserts the exact "path:line:col severity rule" summaries.
func expectDiagnostics(t *testing.T, name string, source string, options Options, want []string) {
	t.Helper()

	result, err := runCheck(t, map[string]string{"guide.md": source}, options)
	if err != nil {
		t.Fatalf("%s: Run() returned error: %v", name, err)
	}
	summary := summarize(t, result, err)
	if want == nil {
		want = []string{}
	}
	if !slices.Equal(summary, want) {
		t.Fatalf("%s: diagnostics = %v, want %v", name, summary, want)
	}
}

func TestCheckHeadingLevelSkip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "downward skip reports the deeper heading",
			source: "# API\n\n#### Parameters\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleHeadingLevelSkip)},
		},
		{
			name:   "single level steps stay clean",
			source: "# API\n\n## Usage\n\n### Flags\n",
			want:   nil,
		},
		{
			name:   "closing sections upward is legal",
			source: "### Details\n\n## Usage\n\n# API\n",
			want:   nil,
		},
		{
			name:   "first heading may start deep",
			source: "### Deep start\n\n#### Deeper\n",
			want:   nil,
		},
		{
			name:   "second heading skip after upward close",
			source: "## A\n\n#### B\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleHeadingLevelSkip)},
		},
		{
			name:   "frontmatter shifts the line",
			source: "---\ntitle: Guide\n---\n\n## A\n\n#### B\n",
			want:   []string{fmt.Sprintf("guide.md:7:1 warning %s", RuleHeadingLevelSkip)},
		},
		{
			name:   "disable silences the rule",
			source: "# A\n\n#### B\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			options := Options{}
			if test.name == "disable silences the rule" {
				options.DisableRules = []string{RuleHeadingLevelSkip}
			}
			expectDiagnostics(t, test.name, test.source, options, test.want)
		})
	}
}

func TestCheckReferenceUndefined(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "full reference without definition",
			source: "# Guide\n\nSee the [Guide][guide].\n",
			want:   []string{fmt.Sprintf("guide.md:3:9 error %s", RuleReferenceUndefined)},
		},
		{
			name:   "collapsed reference without definition",
			source: "# Guide\n\nSee [missing][].\n",
			want:   []string{fmt.Sprintf("guide.md:3:5 error %s", RuleReferenceUndefined)},
		},
		{
			name:   "image reference without definition",
			source: "# Guide\n\n![logo][logo]\n",
			want:   []string{fmt.Sprintf("guide.md:3:2 error %s", RuleReferenceUndefined)},
		},
		{
			name:   "defined labels stay clean",
			source: "# Guide\n\n[Guide][guide] and [collapsed][] and [shortcut].\n\n[guide]: https://example.com/g\n[collapsed]: https://example.com/c\n[shortcut]: https://example.com/s\n",
			want:   nil,
		},
		{
			name:   "shortcut brackets without definition stay prose",
			source: "# Guide\n\nIndex with [brackets] only.\n",
			want:   nil,
		},
		{
			name:   "code content is never a use",
			source: "# Guide\n\nUse `[Guide][guide]` and:\n\n```md\n[Guide][guide]\n```\n",
			want:   nil,
		},
		{
			name:   "frontmatter shifts the line",
			source: "---\ntitle: Guide\n---\n\n[Guide][guide]\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 error %s", RuleReferenceUndefined)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckReferenceUnused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "definition nothing references",
			source: "# Guide\n\n[used]: https://example.com/used\n[unused]: https://example.com/unused\n\nSee [Used][used].\n",
			want:   []string{fmt.Sprintf("guide.md:4:1 warning %s", RuleReferenceUnused)},
		},
		{
			name:   "all definitions referenced",
			source: "# Guide\n\n[used]: https://example.com/used\n\nSee [Used][used].\n",
			want:   nil,
		},
		{
			name:   "shortcut use counts as a reference",
			source: "# Guide\n\n[used]: https://example.com/used\n\nSee [used].\n",
			want:   nil,
		},
		{
			name:   "case folded use counts",
			source: "# Guide\n\n[used]: https://example.com/used\n\nSee [Used][USED].\n",
			want:   nil,
		},
		{
			name:   "image use counts",
			source: "# Guide\n\n[logo]: https://example.com/logo.png\n\n![Logo][logo]\n",
			want:   nil,
		},
		{
			name:   "empty document definitions are unused",
			source: "# Guide\n\n[used]: https://example.com/used\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleReferenceUnused)},
		},
		{
			name:   "frontmatter shifts the line",
			source: "---\ntitle: Guide\n---\n\n[used]: https://example.com/used\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 warning %s", RuleReferenceUnused)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckFootnoteRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "undefined marker",
			source: "# Guide\n\nText[^missing].\n",
			want:   []string{fmt.Sprintf("guide.md:3:5 error %s", RuleFootnoteUndefined)},
		},
		{
			name:   "unused definition",
			source: "# Guide\n\nText[^a].\n\n[^a]: note\n[^b]: orphan\n",
			want:   []string{fmt.Sprintf("guide.md:6:1 warning %s", RuleFootnoteUnused)},
		},
		{
			name:   "empty definition",
			source: "# Guide\n\nText[^e].\n\n[^e]:\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 error %s", RuleFootnoteEmpty)},
		},
		{
			name:   "multiline definition is not empty",
			source: "# Guide\n\nText[^m].\n\n[^m]: first\n    second\n",
			want:   nil,
		},
		{
			name:   "multiple uses keep the definition used",
			source: "# Guide\n\nText[^a] and again[^a].\n\n[^a]: note\n",
			want:   nil,
		},
		{
			name:   "markers inside code stay silent",
			source: "# Guide\n\nUse `[^x]` and:\n\n```md\n[^y]\n```\n\n[^z]: never referenced\n",
			want:   []string{fmt.Sprintf("guide.md:9:1 warning %s", RuleFootnoteUnused)},
		},
		{
			name:   "frontmatter shifts definition lines",
			source: "---\ntitle: Guide\n---\n\n[^b]: orphan\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 warning %s", RuleFootnoteUnused)},
		},
		{
			name:   "frontmatter shifts use lines",
			source: "---\ntitle: Guide\n---\n\nText[^missing].\n",
			want:   []string{fmt.Sprintf("guide.md:5:5 error %s", RuleFootnoteUndefined)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckReferenceRulesWorkAlongsideFilesystemChecks(t *testing.T) {
	t.Parallel()

	// An undefined label reports once, and the resolved neighbor's
	// destination is a scheme URL the filesystem rules rightly ignore — the
	// two rule families coexist without double reporting the same use.
	result, err := runCheck(t, map[string]string{
		"guide.md": "# Guide\n\nSee [Good][good] and [Bad][bad].\n\n[good]: https://example.com/good\n[unused]: https://example.com/unused\n",
	}, Options{})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	rules := make([]string, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		rules = append(rules, diagnostic.Rule)
	}
	want := []string{RuleReferenceUndefined, RuleReferenceUnused}
	slices.Sort(rules)
	if !slices.Equal(rules, want) {
		t.Fatalf("rules = %v, want %v", rules, want)
	}
}
