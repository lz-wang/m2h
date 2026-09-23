package check

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckHiddenScopeMirrorsPublishing pins the check command's hidden
// contract: hidden documents leave the scope by default and references
// pointing at them report not-served with the accurate hidden reason, while
// --hidden brings the documents, their assets and their links back into one
// scope that matches the served workspace.
func TestCheckHiddenScopeMirrorsPublishing(t *testing.T) {
	sources := map[string]string{
		"README.md":        "# Readme\n\n[Guide](.notes/guide.md)\n\n![Pic](.notes/image.png)\n",
		".draft.md":        "# Draft\n\ndraft text\n",
		".notes/guide.md":  "# Guide\n\nnotes guide text\n",
		".notes/image.png": "PNG",
		".git/config":      "[core]",
		".env.production":  "TOKEN=1",
	}

	t.Run("default excludes hidden documents and flags their references", func(t *testing.T) {
		result, err := runCheck(t, sources, Options{Depth: 4})
		summary := summarize(t, result, err)

		if result.Files != 1 {
			t.Fatalf("Files = %d, want 1 (hidden documents leave the scope)", result.Files)
		}
		joined := strings.Join(summary, "\n")
		for _, want := range []string{
			"markdown-target.not-served", // the .notes/guide.md link
			"local-target.missing",       // the .notes/image.png image
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("missing %q finding: %v", want, summary)
			}
		}
		if len(summary) != 2 {
			t.Fatalf("diagnostics = %v, want exactly the two hidden-target findings", summary)
		}
		for _, diagnostic := range result.Diagnostics {
			if !strings.Contains(diagnostic.Message, "excluded as a hidden path") {
				t.Errorf("message %q must name the hidden rule", diagnostic.Message)
			}
		}
	})

	t.Run("hidden brings documents and assets back into one scope", func(t *testing.T) {
		result, err := runCheck(t, sources, Options{Depth: 4, Hidden: true})
		summary := summarize(t, result, err)

		if result.Files != 3 {
			t.Fatalf("Files = %d, want 3 (hidden documents join the scope)", result.Files)
		}
		if len(summary) != 0 {
			t.Fatalf("diagnostics = %v, want none", summary)
		}
	})
}

// TestCheckProtectedPathsStayUnpublishable pins the permanent protection:
// references to .git, .ssh and .env files read as broken with and without
// --hidden, and protected Markdown never joins the scope either way.
func TestCheckProtectedPathsStayUnpublishable(t *testing.T) {
	sources := map[string]string{
		"README.md":       "# Readme\n\n[Env](.env.production)\n\n[Cfg](.git/config)\n\n[Key](.ssh/id_ed25519)\n\n[Envdoc](.env/hosts.md)\n",
		".env/hosts.md":   "# Env doc\n",
		".env.production": "TOKEN=1",
		".git/config":     "[core]",
		".ssh/id_ed25519": "key",
	}

	for _, hidden := range []bool{false, true} {
		result, err := runCheck(t, sources, Options{Depth: 4, Hidden: hidden})
		summary := summarize(t, result, err)

		if result.Files != 1 {
			t.Fatalf("hidden=%v: Files = %d, want 1 (protected Markdown never joins the scope)", hidden, result.Files)
		}
		if len(summary) != 4 {
			t.Fatalf("hidden=%v: diagnostics = %v, want the four protected-target findings", hidden, summary)
		}
		// The protected asset references read as broken through the assets
		// route; the protected Markdown reference reads as not-served.
		wantRules := map[string]int{
			"local-target.missing":       3,
			"markdown-target.not-served": 1,
		}
		for _, diagnostic := range result.Diagnostics {
			wantRules[diagnostic.Rule]--
			if !strings.Contains(diagnostic.Message, "protected from publishing") {
				t.Errorf("hidden=%v: message %q must name the protected rule", hidden, diagnostic.Message)
			}
		}
		for rule, count := range wantRules {
			if count != 0 {
				t.Errorf("hidden=%v: rule %q findings off by %d", hidden, rule, count)
			}
		}
	}
}

// TestCheckHiddenLinkConsistencyWithServer is the release's link-consistency
// test: README references .notes/guide.md, default check reports the target
// outside the publishing scope, --hidden clears the finding — and the
// served workspace answers the same document in the same run shape.
func TestCheckHiddenLinkConsistencyWithServer(t *testing.T) {
	sources := map[string]string{
		"README.md":       "# Readme\n\n[Guide](.notes/guide.md)\n",
		".notes/guide.md": "# Guide\n\n## Install\n",
	}

	defaultRun, err := runCheck(t, sources, Options{Depth: 4})
	summary := summarize(t, defaultRun, err)
	if len(summary) != 1 || !strings.Contains(summary[0], "markdown-target.not-served") {
		t.Fatalf("default diagnostics = %v, want the not-served finding", summary)
	}
	if !strings.Contains(defaultRun.Diagnostics[0].Message, "excluded as a hidden path") {
		t.Errorf("reason %q must name the hidden rule", defaultRun.Diagnostics[0].Message)
	}

	hiddenRun, err := runCheck(t, sources, Options{Depth: 4, Hidden: true})
	summary = summarize(t, hiddenRun, err)
	if len(summary) != 0 {
		t.Fatalf("--hidden diagnostics = %v, want none", summary)
	}
}

// TestCheckRejectsProtectedInputs pins the root boundary on the check side:
// a protected path cannot become the input, directly or as a file beneath a
// protected directory, mirroring the server's refusal.
func TestCheckRejectsProtectedInputs(t *testing.T) {
	t.Parallel()

	base := setupCheckRoot(t, map[string]string{
		"docs/README.md":  "# Docs\n",
		".git/config":     "[core]",
		".ssh/notes.md":   "# Notes\n",
		".env.production": "TOKEN=1",
	})

	for _, protected := range []string{
		filepath.Join(base, ".git"),
		filepath.Join(base, ".ssh", "notes.md"),
		filepath.Join(base, ".env.production"),
	} {
		result, err := Run(context.Background(), Options{Input: protected, Depth: 4, Hidden: true})
		if err == nil || !strings.Contains(err.Error(), "protected path") {
			t.Errorf("Run(%q) error = %v, want protected-path refusal", protected, err)
		}
		if result.Files != 0 {
			t.Errorf("Run(%q) inspected %d files, want none", protected, result.Files)
		}
	}
}

// TestCheckActiveWebAssetsFollowServer pins the assets-route refusal for
// HTML/JS/CSS on both identities: the server refuses active web documents
// so nothing in the published root becomes same-origin executable content,
// and a reference to one reads as broken here instead of falsely valid.
func TestCheckActiveWebAssetsFollowServer(t *testing.T) {
	t.Run("direct references through the assets route", func(t *testing.T) {
		result, err := runCheck(t, map[string]string{
			"README.md":  "# Readme\n\n[App](app.js)\n\n<img src=\"style.css\">\n\n![Manual](manual.pdf)\n",
			"app.js":     "console.log(1)",
			"style.css":  "body{}",
			"manual.pdf": "pdf",
		}, Options{Depth: 4, Hidden: true})
		summary := summarize(t, result, err)

		if len(summary) != 2 {
			t.Fatalf("diagnostics = %v, want exactly the two active-web findings", summary)
		}
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Rule != "local-target.missing" {
				t.Errorf("rule = %q, want local-target.missing", diagnostic.Rule)
			}
			if !strings.Contains(diagnostic.Message, "active web documents") {
				t.Errorf("message %q must name the active-web rule", diagnostic.Message)
			}
		}
	})

	t.Run("passive alias to an active canonical target", func(t *testing.T) {
		base := setupCheckRoot(t, map[string]string{
			"README.md": "# Readme\n\n![Safe](safe.js.txt)\n",
			"app.js":    "console.log(1)",
		})
		if err := os.Symlink(filepath.Join(base, "app.js"), filepath.Join(base, "safe.js.txt")); err != nil {
			t.Fatal(err)
		}

		result, err := Run(context.Background(), Options{Input: base, Depth: 4, Hidden: true})
		summary := summarize(t, result, err)
		if len(summary) != 1 || !strings.Contains(summary[0], "local-target.missing") {
			t.Fatalf("diagnostics = %v, want the canonical active-web finding", summary)
		}
		if !strings.Contains(result.Diagnostics[0].Message, "active web documents") {
			t.Errorf("message %q must name the active-web rule", result.Diagnostics[0].Message)
		}
	})
}

// TestCheckHiddenSymlinkAliasFollowsServer pins the canonical-target rule on
// both routes: a visible alias to a hidden document is refused by default
// and served with --hidden, while an alias to a protected file never serves.
func TestCheckHiddenSymlinkAliasFollowsServer(t *testing.T) {
	t.Run("document alias", func(t *testing.T) {
		sources := map[string]string{
			"README.md":  "# Readme\n\n[Alias](alias.md)\n",
			".secret.md": "# Secret\n",
		}
		base := setupCheckRoot(t, sources)
		if err := os.Symlink(filepath.Join(base, ".secret.md"), filepath.Join(base, "alias.md")); err != nil {
			t.Fatal(err)
		}

		result, err := Run(context.Background(), Options{Input: base, Depth: 4})
		summary := summarize(t, result, err)
		if len(summary) != 1 || !strings.Contains(summary[0], "markdown-target.not-served") {
			t.Fatalf("default diagnostics = %v, want the not-served finding", summary)
		}
		if !strings.Contains(result.Diagnostics[0].Message, "excluded as a hidden path") {
			t.Errorf("reason %q must name the hidden rule", result.Diagnostics[0].Message)
		}

		result, err = Run(context.Background(), Options{Input: base, Depth: 4, Hidden: true})
		if summary := summarize(t, result, err); len(summary) != 0 {
			t.Fatalf("--hidden diagnostics = %v, want none", summary)
		}
	})

	t.Run("asset alias to a protected file", func(t *testing.T) {
		sources := map[string]string{
			"README.md":       "# Readme\n\n![Leak](leak.txt)\n",
			".env.production": "TOKEN=1",
		}
		base := setupCheckRoot(t, sources)
		if err := os.Symlink(filepath.Join(base, ".env.production"), filepath.Join(base, "leak.txt")); err != nil {
			t.Fatal(err)
		}

		// The alias path itself is visible and passive, but its canonical
		// target is protected: the server refuses it either way.
		for _, hidden := range []bool{false, true} {
			result, err := Run(context.Background(), Options{Input: base, Depth: 4, Hidden: hidden})
			summary := summarize(t, result, err)
			if len(summary) != 1 || !strings.Contains(summary[0], "local-target.missing") {
				t.Fatalf("hidden=%v diagnostics = %v, want the protected-target finding", hidden, summary)
			}
			if !strings.Contains(result.Diagnostics[0].Message, "protected from publishing") {
				t.Errorf("hidden=%v reason %q must name the protected rule", hidden, result.Diagnostics[0].Message)
			}
		}
	})
}
