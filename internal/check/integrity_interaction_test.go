package check

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestCheckDocumentIntegrityInteractions pins how the four document-integrity
// rules interact with each other and with the existing rule families: code
// and raw-HTML regions stay exempt, priority order holds on broken inputs,
// and independent problems report independently with exact counts.
func TestCheckDocumentIntegrityInteractions(t *testing.T) {
	t.Parallel()

	t.Run("conflict markers inside an unclosed fence report only the fence", func(t *testing.T) {
		t.Parallel()

		expectDiagnostics(t, "markers are fence content",
			"# Guide\n\n```text\n<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> feature\n",
			Options{}, []string{fmt.Sprintf("guide.md:3:1 error %s", RuleCodeFenceUnclosed)})
	})

	t.Run("conflict followed by a heading reports the conflict once", func(t *testing.T) {
		t.Parallel()

		expectDiagnostics(t, "heading after conflict adds nothing",
			"# Guide\n\n<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> feature\n\n## After\n",
			Options{DisableRules: []string{RuleDocumentMultipleH1}},
			[]string{fmt.Sprintf("guide.md:3:1 error %s", RuleDocumentMergeConflict)})
	})

	t.Run("backticks inside raw html are not a fence", func(t *testing.T) {
		t.Parallel()

		expectDiagnostics(t, "html block swallows fence shapes",
			"# Guide\n\n<div>\n```\nstill html\n```\n</div>\n", Options{}, nil)
	})

	t.Run("conflict example inside an html comment stays silent", func(t *testing.T) {
		t.Parallel()

		expectDiagnostics(t, "comment block swallows conflict markers",
			"# Guide\n\n<!--\n<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> branch\n-->\n", Options{}, nil)
	})

	t.Run("invalid utf8 outranks frontmatter parsing", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "guide.md")
		if err := os.WriteFile(path, []byte("---\ntitle: \xff\n---\n# Guide\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := runCheckOnFile(t, path, Options{})
		if err != nil {
			t.Fatal(err)
		}
		summary := summarize(t, result, err)
		want := []string{fmt.Sprintf("guide.md:2:8 error %s", RuleDocumentInvalidUTF8)}
		if !slices.Equal(summary, want) {
			t.Fatalf("diagnostics = %v, want exactly %v", summary, want)
		}
	})

	t.Run("broken frontmatter without conflicts keeps its single error", func(t *testing.T) {
		t.Parallel()

		expectDiagnostics(t, "yaml error only",
			"---\ntitle: [broken\n---\n# Guide\n", Options{},
			[]string{fmt.Sprintf("guide.md:1:1 error %s", RuleFrontMatterInvalid)})
	})

	t.Run("two independent problems report independently", func(t *testing.T) {
		t.Parallel()

		result, err := runCheck(t, map[string]string{
			"guide.md": "# Guide\n\n![missing](nope.png)\n\n```go\nx\n",
		}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		summary := summarize(t, result, err)
		want := []string{
			fmt.Sprintf("guide.md:3:3 error %s", RuleLocalTargetMissing),
			fmt.Sprintf("guide.md:5:1 error %s", RuleCodeFenceUnclosed),
		}
		if !slices.Equal(summary, want) {
			t.Fatalf("diagnostics = %v, want %v", summary, want)
		}
		if result.Errors != 2 || result.Warnings != 0 {
			t.Fatalf("counts = %d errors, %d warnings, want 2/0", result.Errors, result.Warnings)
		}
	})

	t.Run("an invalid document does not silence its clean neighbor", func(t *testing.T) {
		t.Parallel()

		result, err := runCheck(t, map[string]string{
			"broken.md": "# Broken\n\n<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> f\n",
			"clean.md":  "# Clean\n\n```go\nx\n```\n",
		}, Options{DisableRules: []string{RuleDocumentMultipleH1}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Files != 2 {
			t.Fatalf("Files = %d, want 2", result.Files)
		}
		summary := summarize(t, result, err)
		want := []string{fmt.Sprintf("broken.md:3:1 error %s", RuleDocumentMergeConflict)}
		if !slices.Equal(summary, want) {
			t.Fatalf("diagnostics = %v, want %v", summary, want)
		}
	})
}
