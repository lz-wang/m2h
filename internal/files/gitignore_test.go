package files

import (
	"bufio"
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ignoreFixture describes one root's rule files as slash-relative paths.
// Directories materialize implicitly around the files.
type ignoreFixture map[string]string

func buildIgnoreTree(t *testing.T, fixture ignoreFixture) string {
	t.Helper()
	root := t.TempDir()
	for name, contents := range fixture {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// ignoreDecision is one frozen expectation, validated against the probe run
// and re-validated against real `git check-ignore` whenever git is installed.
type ignoreDecision struct {
	path    string
	isDir   bool
	ignored bool
	// skipGit exempts the decision from the git cross-check:
	// check-ignore cannot express the same-named-file semantics a directory
	// rule needs (an unslashed path is judged as a directory), which is
	// exactly why m2h passes isDir to the matcher.
	skipGit bool
}

// ignoreDecisions is the shared scenario table: it covers nested rules,
// negation, excluded-directory propagation, anchoring, recursive wildcards,
// escapes and directory-only rules.
func ignoreDecisions() []ignoreDecision {
	return []ignoreDecision{
		// directory-only rules
		{path: "draft", isDir: true, ignored: true},
		{path: "draft", isDir: false, ignored: false, skipGit: true}, // same-named file survives
		{path: "draft/unfinished.md", ignored: true},
		// exact file rules
		{path: "images/diagram.png", ignored: false},
		{path: "images/temporary.png", ignored: true},
		// negation
		{path: "app.log", ignored: true},
		{path: "deep/app.log", ignored: true}, // unanchored wildcard reaches every level
		{path: "keep.log", ignored: false},
		// an excluded parent directory defeats negation inside it
		{path: "build", isDir: true, ignored: true},
		{path: "build/result.txt", ignored: true},
		{path: "build/keep.txt", ignored: true},
		// anchoring
		{path: "anchored.md", ignored: true},
		{path: "docs/anchored.md", ignored: false},
		// recursive wildcards, including the zero-directory form
		{path: "sub/deep.md", ignored: true},
		{path: "sub/a/deep.md", ignored: true},
		{path: "sub/a/b/deep.md", ignored: true},
		// unanchored prefix wildcards
		{path: "temporary.txt", ignored: true},
		// escaped negation prefix
		{path: "!literal.md", ignored: true},
		{path: "literal.md", ignored: false},
		// single-level wildcards do not cross separators
		{path: "notes2/a.secret", ignored: true},
		{path: "notes2/deep/a.secret", ignored: false},
		// nested rule files
		{path: "notes/private.md", ignored: true},
		{path: "notes/other.tmp", ignored: true},
		{path: "notes/important.tmp", ignored: false},
		{path: "notes/public.md", ignored: false},
		// a deeper negation overrides a shallower wildcard when no parent
		// directory was excluded
		{path: "sub/keep.tmp", ignored: false},
		{path: "sub/other.tmp", ignored: true},
		// a negation inside an excluded directory cannot re-include a file
		{path: "private/public.md", ignored: true},
		// re-including the directory itself frees everything below it
		{path: "logs/a.txt", ignored: false},
		// the root itself is never ignored
		{path: ".", isDir: true, ignored: false},
	}
}

func sharedIgnoreFixture() ignoreFixture {
	return ignoreFixture{
		".gitignore": strings.Join([]string{
			"draft/",
			"images/temporary.png",
			"*.log",
			"!keep.log",
			"build/",
			"!build/keep.txt",
			"/anchored.md",
			"sub/**/deep.md",
			"temp*",
			`\!literal.md`,
			"notes2/*.secret",
			"*.tmp",
			"!sub/keep.tmp",
			"private/",
			"logs/",
			"!logs/",
		}, "\n"),
		"notes/.gitignore": strings.Join([]string{
			"private.md",
			"*.tmp",
			"!important.tmp",
		}, "\n"),
		"private/.gitignore": "!public.md\n",
	}
}

func TestGitIgnoreMatchesGitSemantics(t *testing.T) {
	root := buildIgnoreTree(t, sharedIgnoreFixture())
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range ignoreDecisions() {
		ignored, err := matcher.Ignored(decision.path, decision.isDir)
		if err != nil {
			t.Fatalf("Ignored(%q): %v", decision.path, err)
		}
		if ignored != decision.ignored {
			t.Errorf("Ignored(%q, isDir=%v) = %v, want %v", decision.path, decision.isDir, ignored, decision.ignored)
		}
	}
}

// TestGitIgnoreAgreesWithGitCheckIgnore re-runs the frozen scenario table
// through real `git check-ignore` and fails when the snapshot matcher would
// decide differently. It is the regression net for the parser dependency.
func TestGitIgnoreAgreesWithGitCheckIgnore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := buildIgnoreTree(t, sharedIgnoreFixture())
	decisions := ignoreDecisions()

	if output := gitCheckIgnore(t, root, decisions); output != nil {
		decided := make(map[string]bool, len(output))
		maps.Copy(decided, output)
		for _, decision := range decisions {
			if decision.path == "." || decision.skipGit {
				continue // see the ignoreDecision comments
			}
			gitIgnored, probed := decided[decision.path]
			if !probed {
				gitIgnored = false
			}
			if gitIgnored != decision.ignored {
				t.Errorf("git check-ignore %q = %v, frozen expectation = %v", decision.path, gitIgnored, decision.ignored)
			}
		}
	}
}

// gitCheckIgnore runs `git check-ignore -v --no-index --stdin` over the
// decision paths inside a fresh repository and reports each path's final
// verdict; nil output means git could not run in this environment.
func gitCheckIgnore(t *testing.T, root string, decisions []ignoreDecision) map[string]bool {
	t.Helper()
	if err := runGit(t, root, "init", "-q", "."); err != nil {
		t.Skipf("git init failed: %v", err)
	}

	queries := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		if decision.path == "." {
			continue
		}
		// check-ignore reads a trailing slash as "this is a directory",
		// which is exactly how the matcher receives isDir.
		if decision.isDir {
			queries = append(queries, decision.path+"/")
		} else {
			queries = append(queries, decision.path)
		}
	}

	command := exec.Command("git", "check-ignore", "-v", "--no-index", "--stdin")
	command.Dir = root
	command.Stdin = strings.NewReader(strings.Join(queries, "\n"))
	output, err := command.Output()
	// Exit code 1 means "no path is ignored"; anything else is a failure.
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Skipf("git check-ignore failed: %v", err)
		}
	}

	verdicts := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		path := parts[1]
		rule := strings.SplitN(parts[0], ":", 3)
		if len(rule) != 3 {
			continue
		}
		verdicts[strings.TrimSuffix(path, "/")] = !strings.HasPrefix(rule[2], "!")
	}
	return verdicts
}

func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	return command.Run()
}

func TestGitIgnoreToleratesMissingEmptyAndCommentedRules(t *testing.T) {
	root := buildIgnoreTree(t, ignoreFixture{
		".gitignore":        "\n# only a comment\n   \n",
		"nested/.gitignore": "",
	})
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"document.md", "nested/document.md", "draft"} {
		ignored, err := matcher.Ignored(path, false)
		if err != nil {
			t.Fatalf("Ignored(%q): %v", path, err)
		}
		if ignored {
			t.Errorf("Ignored(%q) = true, want false for empty rules", path)
		}
	}
}

func TestGitIgnoreNormalizesWindowsLineEndings(t *testing.T) {
	root := buildIgnoreTree(t, ignoreFixture{
		".gitignore":      "draft/\r\nnotes/private.md\r\n",
		"deep/.gitignore": "*.tmp\r\n",
	})
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range []ignoreDecision{
		{path: "draft/plan.md", ignored: true},
		{path: "notes/private.md", ignored: true},
		{path: "notes/public.md", ignored: false},
		{path: "deep/scratch.tmp", ignored: true},
	} {
		ignored, err := matcher.Ignored(decision.path, decision.isDir)
		if err != nil {
			t.Fatalf("Ignored(%q): %v", decision.path, err)
		}
		if ignored != decision.ignored {
			t.Errorf("Ignored(%q) = %v, want %v", decision.path, ignored, decision.ignored)
		}
	}
}

func TestGitIgnoreRejectsPathsOutsideTheRoot(t *testing.T) {
	root := buildIgnoreTree(t, ignoreFixture{".gitignore": "*.md\n"})
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside.md", "..", "a/../../b.md"} {
		if _, err := matcher.Ignored(path, false); err == nil {
			t.Errorf("Ignored(%q) returned no error, want an outside-root failure", path)
		}
	}
}

func TestGitIgnoreSurfacesUnreadableRuleFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read files regardless of permissions")
	}
	root := buildIgnoreTree(t, ignoreFixture{".gitignore": "*.md\n"})
	if err := os.Chmod(filepath.Join(root, ".gitignore"), 0o000); err != nil {
		t.Fatal(err)
	}
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := matcher.Ignored("document.md", false); err == nil {
		t.Fatal("Ignored() returned no error for an unreadable rule file")
	}
}

func TestNewGitIgnoreRequiresADirectoryRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "document.md")
	writeTestFile(t, file, "# Title")
	if _, err := NewGitIgnore(file); err == nil {
		t.Fatal("NewGitIgnore() accepted a file root")
	}
	if _, err := NewGitIgnore(filepath.Join(root, "missing")); err == nil {
		t.Fatal("NewGitIgnore() accepted a missing root")
	}
	if _, err := NewGitIgnore(root); err != nil {
		t.Fatalf("NewGitIgnore(%q): %v", root, err)
	}
}

// buildDiscoveryTree lays out the workspace the discovery tests share.
func buildDiscoveryTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, contents := range map[string]string{
		"README.md":            "# Root",
		"draft/note.md":        "# Draft",
		"draft/deep/inner.md":  "# Inner draft",
		"images/diagram.png":   "PNG",
		"images/temporary.png": "PNG",
		"app.log":              "log",
		"keep.log":             "log",
		"secret/hidden.md":     "# Hidden target",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), contents)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("draft/\nimages/temporary.png\n*.log\n!keep.log\nsecret/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret", "hidden.md"), filepath.Join(root, "alias.md")); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscoverAppliesIgnoreToMarkdownAndAssets(t *testing.T) {
	root := buildDiscoveryTree(t)
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	discovered, err := Discover(context.Background(), root, DiscoverOptions{
		Depth:      4,
		SkipHidden: true,
		Ignore:     matcher,
	})
	if err != nil {
		t.Fatal(err)
	}

	markdown := entryPaths(discovered.Markdown)
	if len(markdown) != 1 || markdown[0] != "README.md" {
		t.Errorf("Markdown = %v, want [README.md]", markdown)
	}
	assets := entryPaths(discovered.Assets)
	if !slices.Equal(assets, []string{"images/diagram.png", "keep.log"}) {
		t.Errorf("Assets = %v, want [images/diagram.png keep.log]", assets)
	}
}

func TestDiscoverWithoutIgnoreKeepsEverything(t *testing.T) {
	root := buildDiscoveryTree(t)
	discovered, err := Discover(context.Background(), root, DiscoverOptions{Depth: 4, SkipHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	// alias.md, draft/deep/inner.md, draft/note.md, README.md and
	// secret/hidden.md as Markdown, plus the four assets — everything but
	// dot paths.
	if got := len(discovered.Markdown) + len(discovered.Assets); got != 9 {
		t.Fatalf("Markdown+Assets = %d entries (%v / %v), want 9",
			got, entryPaths(discovered.Markdown), entryPaths(discovered.Assets))
	}
}

func TestDiscoverSkipsAliasPointingAtIgnoredTarget(t *testing.T) {
	root := buildDiscoveryTree(t)
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	discovered, err := Discover(context.Background(), root, DiscoverOptions{
		Depth:      4,
		SkipHidden: true,
		Ignore:     matcher,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range discovered.Markdown {
		if entry.RelativePath == "alias.md" {
			t.Error("alias.md resolves to the ignored secret/hidden.md and must not be discovered")
		}
	}
}

func TestFindDirectoryDocumentSkipsIgnoredDocuments(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		"docs/README.md":     "# Docs",
		"docs/guide.md":      "# Guide",
		"notes/README.md":    "# Notes",
		"archive/README.md":  "# Archive",
		"archive/history.md": "# History",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(name)), contents)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("docs/README.md\narchive/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	matcher, err := NewGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	options := DiscoverOptions{Depth: 4, SkipHidden: true, Ignore: matcher}

	// docs/README.md is ignored, so the next Markdown in the level wins.
	if document := FindDirectoryDocument(context.Background(), root, "docs", options); document != "docs/guide.md" {
		t.Errorf("FindDirectoryDocument(docs) = %q, want docs/guide.md", document)
	}
	// archive/ is ignored as a whole: no entry document, no BFS inside.
	if document := FindDirectoryDocument(context.Background(), root, "archive", options); document != "" {
		t.Errorf("FindDirectoryDocument(archive) = %q, want an empty result", document)
	}
	// notes/ carries no rule of its own and keeps picking its README.
	if document := FindDirectoryDocument(context.Background(), root, "notes", options); document != "notes/README.md" {
		t.Errorf("FindDirectoryDocument(notes) = %q, want notes/README.md", document)
	}
}
