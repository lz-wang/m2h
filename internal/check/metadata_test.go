package check

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMetadataQualityRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source string
		want         []string
	}{
		{"matching", "---\ntitle: Guide\ntags: [a,b,c,d,e]\n---\n# **Guide**\n", nil},
		{"whitespace", "---\ntitle: '  Hello   World  '\n---\nHello World\n===\n", nil},
		{"mismatch", "---\ntitle: Guide\n---\n# Different\n", []string{"guide.md:2:1 warning frontmatter.title-mismatch"}},
		{"no H1", "---\ntitle: Guide\n---\n## Different\n", nil},
		{"no metadata", "# Guide\n", nil},
		{"no title", "---\ndescription: Guide\n---\n# Guide\n", nil},
		{"empty title", "---\ntitle: ''\n---\n# Guide\n", nil},
		{"unsupported title", "---\ntitle: [Guide]\n---\n# Guide\n", nil},
		{"first H1 only", "---\ntitle: Guide\n---\n# Guide\n\n# Other\n", []string{"guide.md:6:1 warning document.multiple-h1"}},
		{"empty tags", "---\ntags: []\n---\n# Guide\n", []string{"guide.md:2:1 warning frontmatter.tags-count"}},
		{"null tags", "---\ntags: null\n---\n# Guide\n", []string{"guide.md:2:1 warning frontmatter.tags-count"}},
		{"mapping tags", "---\ntags: {a: b}\n---\n# Guide\n", []string{"guide.md:2:1 warning frontmatter.tags-count"}},
		{"six tags", "---\ntags: [a,b,c,d,e,f]\n---\n# Guide\n", []string{"guide.md:2:1 warning frontmatter.tags-count"}},
		{"normalize tags", "---\ntags: [a, a, '', null, ~, {}, [x]]\n---\n# Guide\n", nil},
		{"scalar tag", "---\ntags: 'a,b'\n---\n# Guide\n", nil},
		{"quoted null tag", "---\ntags: 'null'\n---\n# Guide\n", nil},
		{"all empty", "---\ntags: ['', null, ~]\n---\n# Guide\n", []string{"guide.md:2:1 warning frontmatter.tags-count"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectDiagnostics(t, tt.name, tt.source, Options{}, tt.want) })
	}
}

func TestTrailingBlankLinesRawSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source string
		warn         bool
	}{
		{"one LF blank", "text\n\n", false},
		{"one CRLF blank", "text\r\n\r\n", false},
		{"no newline", "text", true},
		{"only line ending", "text\n", true},
		{"two blanks", "text\n\n\n", true},
		{"CRLF extra", "text\r\n\r\n\r\n", true},
		{"spaces in blank", "text\n \t\n", false},
		{"unterminated blank", "text\n \t", true},
		{"whitespace extra", "text\n \n\t\n", true},
		{"empty", "", true},
		{"blank only", "\n", false},
		{"frontmatter only", "---\ntags: [a]\n---\n\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "guide.md")
			writeFile(t, path, tt.source)
			result, err := Run(context.Background(), Options{Input: path})
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if tt.warn {
				want = 1
			}
			if result.Warnings != want || result.Errors != 0 {
				t.Fatalf("unexpected diagnostics: %+v", result)
			}
			if tt.warn && result.Diagnostics[0].Rule != RuleDocumentTrailingBlankLines {
				t.Fatalf("wrong rule: %+v", result)
			}
		})
	}
}

func TestMetadataRulesSelectionAndInvalidYAML(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "guide.md")
	writeFile(t, path, "---\ntitle: Other\ntags: []\n---\n# Guide\n")
	result, err := Run(context.Background(), Options{Input: path})
	if err != nil {
		t.Fatal(err)
	}
	if result.Errors != 0 || result.Warnings != 3 {
		t.Fatalf("warning severity/strict contract: %+v", result)
	}
	result, err = Run(context.Background(), Options{Input: path, EnableRules: []string{"all"}, DisableRules: []string{RuleFrontMatterTitleMismatch, RuleFrontMatterTagsCount, RuleDocumentTrailingBlankLines}})
	if err != nil {
		t.Fatal(err)
	}
	// No body content means section.empty is allowed when all opt-in rules run.
	for _, d := range result.Diagnostics {
		if d.Rule == RuleFrontMatterTitleMismatch || d.Rule == RuleFrontMatterTagsCount || d.Rule == RuleDocumentTrailingBlankLines {
			t.Fatalf("disabled rule: %+v", d)
		}
	}
	writeFile(t, path, "---\ntitle: [\n---\n# Guide\n")
	result, err = Run(context.Background(), Options{Input: path})
	if err != nil {
		t.Fatal(err)
	}
	if result.Errors != 1 || result.Warnings != 1 {
		t.Fatalf("invalid YAML must not hide raw EOF check or emit metadata warnings: %+v", result)
	}
}
