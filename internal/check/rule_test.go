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
			name:   "inline link title before real use keeps real position",
			source: "# Guide\n\n[a](# \"example [fake][missing]\") and [Real][missing].\n",
			want:   []string{fmt.Sprintf("guide.md:3:38 error %s", RuleReferenceUndefined)},
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

func TestCheckTableColumnMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "long data row",
			source: "# Guide\n\n| A | B |\n|---|---|\n| 1 | 2 | 3 |\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 error %s", RuleTableColumnMismatch)},
		},
		{
			name:   "short data row",
			source: "# Guide\n\n| A | B |\n|---|---|\n| 1 |\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 error %s", RuleTableColumnMismatch)},
		},
		{
			name:   "short header padded by the parser",
			source: "# Guide\n\n| A |\n|---|---|\n| 1 | 2 |\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 error %s", RuleTableColumnMismatch)},
		},
		{
			name:   "rejected header/delimiter pair",
			source: "# Guide\n\n| A | B | C |\n|---|---|\n",
			want:   []string{fmt.Sprintf("guide.md:4:1 error %s", RuleTableColumnMismatch)},
		},
		{
			name:   "consistent table stays clean",
			source: "# Guide\n\n| A | B |\n|---|---|\n| 1 | 2 |\n| 1 \\| 2 | w |\n",
			want:   nil,
		},
		{
			name:   "frontmatter shifts the row",
			source: "---\ntitle: Guide\n---\n\n| A | B |\n|---|---|\n| 1 |\n",
			want:   []string{fmt.Sprintf("guide.md:7:1 error %s", RuleTableColumnMismatch)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckHTMLCommentUnclosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "unclosed comment swallows content",
			source: "# A\n\n<!-- forgotten\n\n## B\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 error %s", RuleHTMLCommentUnclosed)},
		},
		{
			name:   "closed comment stays clean",
			source: "# A\n\n<!-- note -->\n\n## B\n",
			want:   nil,
		},
		{
			name:   "comment in fenced code is code",
			source: "# A\n\n```html\n<!-- oops\n```\n",
			want:   nil,
		},
		{
			name:   "comment in inline code is text",
			source: "# A\n\nUse `<!-- oops` here.\n",
			want:   nil,
		},
		{
			name:   "frontmatter value is YAML, not Markdown",
			source: "---\ndescription: \"<!-- example\"\n---\n\n# A\n",
			want:   nil,
		},
		{
			name:   "frontmatter shifts the opener",
			source: "---\ntitle: A\n---\n\n<!-- broken\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 error %s", RuleHTMLCommentUnclosed)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckLinkReversed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "https destination",
			source: "# Guide\n\n(OpenAI)[https://openai.com]\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 error %s", RuleLinkReversed)},
		},
		{
			name:   "markdown file destination",
			source: "# Guide\n\nSee (Guide)[guide.md].\n",
			want:   []string{fmt.Sprintf("guide.md:3:5 error %s", RuleLinkReversed)},
		},
		{
			name:   "anchor destination",
			source: "# Guide\n\n(Section)[#setup]\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 error %s", RuleLinkReversed)},
		},
		{
			name:   "indexing stays prose",
			source: "# Guide\n\nCall f(x)[0] and array[index].\n",
			want:   nil,
		},
		{
			name:   "code protects the shape",
			source: "# Guide\n\nUse `(x)[y.md]`.\n\n```md\n(x)[y.md]\n```\n",
			want:   nil,
		},
		{
			name:   "frontmatter shifts the line",
			source: "---\ntitle: Guide\n---\n\n(x)[y.md]\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 error %s", RuleLinkReversed)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckCodeFenceLanguageMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "plain fence reports",
			source: "# Guide\n\n```\necho hi\n```\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleCodeFenceLanguageMissing)},
		},
		{
			name:   "named fences stay clean",
			source: "# Guide\n\n```go\nx\n```\n\n~~~mermaid\nx\n~~~\n\n```text\nx\n```\n",
			want:   nil,
		},
		{
			name:   "info string with attributes keeps the language",
			source: "# Guide\n\n```go title=x\ny\n```\n",
			want:   nil,
		},
		{
			name:   "indented code is not a fence",
			source: "# Guide\n\n    plain code\n",
			want:   nil,
		},
		{
			name:   "unclosed fence reports at its opener",
			source: "# Guide\n\n```\necho hi\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleCodeFenceLanguageMissing)},
		},
		{
			name:   "fence inside a blockquote reports",
			source: "# Guide\n\n> ```\n> code\n> ```\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleCodeFenceLanguageMissing)},
		},
		{
			name:   "named fence inside a list item stays clean",
			source: "# Guide\n\n- ```js\n  code\n  ```\n",
			want:   nil,
		},
		{
			name:   "empty fence inside an asterisk list item reports at opener",
			source: "# Guide\n\n* ```\n  ```\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleCodeFenceLanguageMissing)},
		},
		{
			name:   "fence-looking lines inside an html block stay clean",
			source: "# Guide\n\n<div>\n```\nnot fence\n```\n</div>\n",
			want:   nil,
		},
		{
			name:   "frontmatter shifts the fence",
			source: "---\ntitle: Guide\n---\n\n```\nx\n```\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 warning %s", RuleCodeFenceLanguageMissing)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckHeadingDuplicate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "same text under one parent",
			source: "# API\n\n## Install\n\n## Install\n",
			want:   []string{fmt.Sprintf("guide.md:5:1 warning %s", RuleHeadingDuplicate)},
		},
		{
			name:   "same text under different parents stays clean",
			source: "# Guide\n\n## Client\n\n### Usage\n\n## Server\n\n### Usage\n",
			want:   nil,
		},
		{
			name:   "deeper nesting closes the section",
			source: "# API\n\n## Install\n\n### Details\n\n## Install\n",
			want:   []string{fmt.Sprintf("guide.md:7:1 warning %s", RuleHeadingDuplicate)},
		},
		{
			name:   "duplicate root-level sections without an H1 parent",
			source: "## Install\n\n## Install\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleHeadingDuplicate)},
		},
		{
			name:   "duplicate H1 belongs to multiple-h1 only",
			source: "# Guide\n\n# Guide\n",
			want:   []string{fmt.Sprintf("guide.md:3:1 warning %s", RuleDocumentMultipleH1)},
		},
		{
			name:   "text compared after whitespace normalization",
			source: "# API\n\n## Install\n\n##  Install \n",
			want:   []string{fmt.Sprintf("guide.md:5:1 warning %s", RuleHeadingDuplicate)},
		},
		{
			name:   "frontmatter shifts the duplicate",
			source: "---\ntitle: Guide\n---\n\n# API\n\n## Install\n\n## Install\n",
			want:   []string{fmt.Sprintf("guide.md:9:1 warning %s", RuleHeadingDuplicate)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expectDiagnostics(t, test.name, test.source, Options{}, test.want)
		})
	}
}

func TestCheckOptInRulesStayOffByDefault(t *testing.T) {
	t.Parallel()

	source := "# Guide\n\nIntro.\n\n## Empty\n\n## Next\n\nSee [click here](https://example.com) and CafÃ© and word​word.\n"
	result, err := runCheck(t, map[string]string{"guide.md": source}, Options{})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Rule {
		case RuleSectionEmpty, RuleLinkTextNondescriptive, RuleUnicodeMojibake, RuleUnicodeInvisibleCharacter:
			t.Fatalf("opt-in rule %s fired without --enable: %+v", diagnostic.Rule, diagnostic)
		}
	}
}

func TestCheckSectionEmptyOptIn(t *testing.T) {
	t.Parallel()

	source := "# Title\n\nIntro.\n\n## Empty\n\n## Next\n\ncontent\n"
	enabled := Options{EnableRules: []string{RuleSectionEmpty}}
	expectDiagnostics(t, "enabled", source, enabled,
		[]string{fmt.Sprintf("guide.md:5:1 warning %s", RuleSectionEmpty)})
	expectDiagnostics(t, "default off", source, Options{}, nil)
	expectDiagnostics(t, "thematic break is content",
		"# Title\n\nIntro.\n\n## Divider\n\n***\n\n## Next\n\ncontent\n",
		enabled, nil)

	withFrontmatter := Options{EnableRules: []string{RuleSectionEmpty}}
	expectDiagnostics(t, "frontmatter shifts the heading", "---\ntitle: T\n---\n\n# T\n\n## Empty\n\n## Next\n\ncontent\n",
		withFrontmatter, []string{
			fmt.Sprintf("guide.md:5:1 warning %s", RuleSectionEmpty), // # T with only headings
			fmt.Sprintf("guide.md:7:1 warning %s", RuleSectionEmpty),
		})
}

func TestCheckLinkTextNondescriptiveOptIn(t *testing.T) {
	t.Parallel()

	enabled := Options{EnableRules: []string{RuleLinkTextNondescriptive}}

	expectDiagnostics(t, "english and chinese generic texts",
		"# Guide\n\nSee [click here](https://example.com) and [点击这里](https://example.com).\n",
		enabled, []string{
			fmt.Sprintf("guide.md:3:6 warning %s", RuleLinkTextNondescriptive),
			fmt.Sprintf("guide.md:3:44 warning %s", RuleLinkTextNondescriptive),
		})

	expectDiagnostics(t, "informative text stays silent",
		"# Guide\n\nSee [click here to read the API docs](https://example.com), [the guide](https://example.com) and ![alt](https://example.com/x.png).\n",
		enabled, nil)

	expectDiagnostics(t, "image alt and raw HTML are out of scope",
		"# Guide\n\n![logo](https://example.com/x.png)\n\n<a href=\"https://example.com\">link</a>\n",
		enabled, nil)
}

func TestCheckUnicodeRulesOptIn(t *testing.T) {
	t.Parallel()

	enabled := Options{EnableRules: []string{RuleUnicodeMojibake, RuleUnicodeInvisibleCharacter}}

	expectDiagnostics(t, "mojibake and invisible findings",
		"# Guide\n\nCafÃ© and word ​word.\n",
		enabled, []string{
			fmt.Sprintf("guide.md:3:4 warning %s", RuleUnicodeMojibake),
			fmt.Sprintf("guide.md:3:18 warning %s", RuleUnicodeInvisibleCharacter),
		})

	expectDiagnostics(t, "correct text and emoji stay silent",
		"# Guide\n\nCafé ❤️ and 👩‍❤️‍👨.\n",
		enabled, nil)

	expectDiagnostics(t, "code regions stay silent",
		"# Guide\n\nUse `CafÃ©` and `​`.\n",
		enabled, nil)
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
