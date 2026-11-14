package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// runCheckOnFile runs a single-file check over a path holding raw fixture
// bytes that the map-based helpers would normalize.
func runCheckOnFile(t *testing.T, path string, options Options) (Result, error) {
	t.Helper()
	options.Input = path
	return Run(context.Background(), options)
}

// TestCheckInvalidUTF8 covers the byte-level pre-check. Fixtures are built
// from raw []byte and written into temporary files — invalid UTF-8 must never
// enter the repository as a literal file.
func TestCheckInvalidUTF8(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  []byte
		options Options
		want    []string
	}{
		{
			name:   "plain chinese document passes",
			source: []byte("# 指南\n\n正文内容。\n"),
		},
		{
			name:   "utf-8 bom passes",
			source: append([]byte{0xEF, 0xBB, 0xBF}, []byte("# Guide\n")...),
		},
		{
			name:   "legal replacement character passes",
			source: []byte("# Guide\n\nreplacement: " + string([]byte{0xEF, 0xBF, 0xBD}) + "\n"),
		},
		{
			name:   "invalid single byte reports at its position",
			source: []byte("# Guide\n\nbroken \xff here\n"),
			want:   []string{fmt.Sprintf("guide.md:3:8 error %s", RuleDocumentInvalidUTF8)},
		},
		{
			name:   "incomplete multi-byte sequence reports",
			source: []byte("text\n\xe4\xb8\n"),
			want:   []string{fmt.Sprintf("guide.md:2:1 error %s", RuleDocumentInvalidUTF8)},
		},
		{
			name:   "incomplete four-byte sequence reports",
			source: []byte("# Guide\n\nword \xf0\x9f\n"),
			want:   []string{fmt.Sprintf("guide.md:3:6 error %s", RuleDocumentInvalidUTF8)},
		},
		{
			name:   "invalid byte in frontmatter reports utf8 only",
			source: []byte("---\ntitle: \xff\n---\n# Guide\n"),
			want:   []string{fmt.Sprintf("guide.md:2:8 error %s", RuleDocumentInvalidUTF8)},
		},
		{
			name:   "crlf file locates line and column",
			source: []byte("# Guide\r\nbroken \xff\r\n"),
			want:   []string{fmt.Sprintf("guide.md:2:8 error %s", RuleDocumentInvalidUTF8)},
		},
		{
			name:   "several invalid bytes report only the first",
			source: []byte("\xffirst\nsecond \xff\n"),
			want:   []string{fmt.Sprintf("guide.md:1:1 error %s", RuleDocumentInvalidUTF8)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "guide.md")
			if err := os.WriteFile(path, test.source, 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := runCheckOnFile(t, path, test.options)
			summary := summarize(t, result, err)
			want := test.want
			if want == nil {
				want = []string{}
			}
			if !slices.Equal(summary, want) {
				t.Fatalf("%s: diagnostics = %v, want %v", test.name, summary, want)
			}
			if test.want != nil && result.Errors != 1 {
				t.Fatalf("%s: Errors = %d, want 1", test.name, result.Errors)
			}
		})
	}
}

func TestCheckInvalidUTF8Disable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "guide.md")
	if err := os.WriteFile(path, []byte("# Guide\n\nbroken \xff here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := runCheckOnFile(t, path, Options{DisableRules: []string{RuleDocumentInvalidUTF8}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none with the rule disabled", result.Diagnostics)
	}
}

// TestCheckInvalidUTF8StopsBeforeFrontmatter pins the early-return contract:
// an invalid encoding must not leak derived frontmatter or body diagnostics,
// while the raw trailing-newline check still applies to the source bytes.
func TestCheckInvalidUTF8StopsBeforeFrontmatter(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "guide.md")
	if err := os.WriteFile(path, []byte("---\ntitle: [\xff\n---\n# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := runCheckOnFile(t, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rules := make([]string, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		rules = append(rules, diagnostic.Rule)
	}
	slices.Sort(rules)
	want := []string{RuleDocumentInvalidUTF8, RuleDocumentTrailingBlankLines}
	if !slices.Equal(rules, want) {
		t.Fatalf("rules = %v, want exactly %v without frontmatter.invalid", rules, want)
	}
	if result.Files != 1 {
		t.Fatalf("Files = %d, want 1: the invalid document still counts", result.Files)
	}
}
