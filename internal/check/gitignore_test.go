package check

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckGitignoreNarrowsScopeAndFlagsReferences pins the check command's
// ignore contract: ignored documents leave the scope, references pointing at
// them report the served-workspace verdict, and visible references stay
// clean.
func TestCheckGitignoreNarrowsScopeAndFlagsReferences(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		".gitignore":    "draft/\nscratch.png\n",
		"README.md":     "# Readme\n\n[Draft](draft/note.md)\n",
		"guide.md":      "# Guide\n\n![Logo](logo.png)\n\n![Scratch](scratch.png)\n",
		"draft/note.md": "# Draft\n",
		"logo.png":      "png",
		"scratch.png":   "png",
	}, Options{Depth: 4, Gitignore: true})
	summary := summarize(t, result, err)

	if result.Files != 2 {
		t.Fatalf("Files = %d, want 2 (draft/note.md must leave the scope)", result.Files)
	}
	if len(summary) != 2 {
		t.Fatalf("diagnostics = %v, want exactly the two ignored-target findings", summary)
	}
	for _, line := range summary {
		if !strings.Contains(line, "not-served") && !strings.Contains(line, "local-target.missing") {
			t.Errorf("unexpected diagnostic %q", line)
		}
	}
	found := strings.Join(summary, "\n")
	if !strings.Contains(found, "markdown-target.not-served") {
		t.Errorf("missing the ignored Markdown target finding: %s", found)
	}
	if !strings.Contains(found, "local-target.missing") {
		t.Errorf("missing the ignored asset finding: %s", found)
	}
	for _, diagnostic := range result.Diagnostics {
		if !strings.Contains(diagnostic.Message, ".gitignore rules") {
			t.Errorf("message %q must name the rule source", diagnostic.Message)
		}
	}
}

// TestCheckWithoutGitignoreKeepsEverything pins the off switch: the same
// tree without Gitignore checks every file and reports no broken targets.
func TestCheckWithoutGitignoreKeepsEverything(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		".gitignore":    "draft/\nscratch.png\n",
		"README.md":     "# Readme\n\n[Draft](draft/note.md)\n",
		"draft/note.md": "# Draft\n",
		"scratch.png":   "png",
	}, Options{Depth: 4})
	summary := summarize(t, result, err)

	if result.Files != 2 {
		t.Fatalf("Files = %d, want 2", result.Files)
	}
	if len(summary) != 0 {
		t.Fatalf("diagnostics = %v, want none", summary)
	}
}

// TestCheckGitignoreSingleFileInputIgnoresNothing pins the explicit-input
// boundary for check: a named Markdown file is checked even when its own
// rules say otherwise.
func TestCheckGitignoreSingleFileInputIgnoresNothing(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, ".gitignore"), []byte("chosen.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "chosen.md"), []byte("# Chosen\n\n[Broken](missing.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), Options{
		Input:     filepath.Join(base, "chosen.md"),
		Depth:     4,
		Gitignore: true,
	})
	summary := summarize(t, result, err)

	if result.Files != 1 {
		t.Fatalf("Files = %d, want 1", result.Files)
	}
	if len(summary) != 1 || !strings.Contains(summary[0], "local-target.missing") {
		t.Fatalf("diagnostics = %v, want the broken link of the chosen file", summary)
	}
}

// TestCheckGitignoreNestedRulesFollowServerBehavior exercises a nested rule
// file: the ignored document vanishes from the scope, so a link to it reads
// as unserved, not as missing.
func TestCheckGitignoreNestedRulesFollowServerBehavior(t *testing.T) {
	t.Parallel()

	result, err := runCheck(t, map[string]string{
		"notes/.gitignore": "private.md\n",
		"notes/public.md":  "# Public\n\n[Private](private.md)\n",
		"notes/private.md": "# Private\n",
	}, Options{Depth: 4, Gitignore: true})
	summary := summarize(t, result, err)

	if result.Files != 1 {
		t.Fatalf("Files = %d, want 1 (notes/private.md must leave the scope)", result.Files)
	}
	if len(summary) != 1 || !strings.Contains(summary[0], "markdown-target.not-served") {
		t.Fatalf("diagnostics = %v, want the unserved private.md target", summary)
	}
}
