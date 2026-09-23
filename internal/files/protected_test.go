package files

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestResolveRejectsProtectedInputs pins the root boundary: a protected
// path cannot become an input root, directly or through a symlink alias,
// whatever its kind — inside such a root the protected name disappears from
// every root-relative judgment, so refusing the root is the only guard.
func TestResolveRejectsProtectedInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, relative := range []string{
		".git/config",
		".ssh/id_ed25519",
		".env/hosts.md",
		".env.production",
		"docs/README.md",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(relative)), "x")
	}

	for _, protected := range []string{
		filepath.Join(root, ".git"),
		filepath.Join(root, ".ssh"),
		filepath.Join(root, ".env"),
		filepath.Join(root, ".env.production"),
		filepath.Join(root, ".ssh", "id_ed25519"),
	} {
		if _, err := Resolve(protected); err == nil || !strings.Contains(err.Error(), "protected path") {
			t.Errorf("Resolve(%q) error = %v, want protected-path refusal", protected, err)
		}
	}

	// Symlink aliases resolve to the protected target before the check.
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(filepath.Join(root, "alias")); err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Errorf("Resolve(alias) error = %v, want protected-path refusal", err)
	}
	if err := os.Symlink(filepath.Join(root, ".env.production"), filepath.Join(root, "config.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(filepath.Join(root, "config.md")); err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Errorf("Resolve(config.md) error = %v, want protected-path refusal", err)
	}

	// A visible root that merely contains protected entries keeps resolving.
	input, err := Resolve(filepath.Join(root, "docs"))
	if err != nil {
		t.Fatalf("Resolve(docs) error = %v", err)
	}
	if input.Kind != KindDirectory {
		t.Fatalf("docs kind = %d, want directory", input.Kind)
	}

	// The walking entry-document selection inherits the same refusal.
	if got := FindDirectoryDocument(context.Background(), filepath.Join(root, ".git"), ".", DiscoverOptions{Depth: 4, SkipHidden: false}); got != "" {
		t.Fatalf("FindDirectoryDocument on protected root = %q, want empty", got)
	}
}

// TestIsProtectedPathCoversSensitiveComponents pins the permanent protection
// policy: .git, .ssh, .env and derived .env.* spellings are protected at any
// depth, while look-alike names (.gitignore, .github, .environment) and
// everything visible stay publishable.
func TestIsProtectedPathCoversSensitiveComponents(t *testing.T) {
	t.Parallel()

	for _, protected := range []string{
		".git",
		".git/config",
		".git/hooks/pre-commit.sample",
		"worktree/.git/HEAD",
		".ssh",
		".ssh/id_ed25519",
		"home/.ssh/known_hosts",
		".env",
		".env.production",
		".env.local",
		"deploy/.env.staging",
	} {
		if !IsProtectedPath(protected) {
			t.Errorf("IsProtectedPath(%q) = false, want true", protected)
		}
	}
	for _, publishable := range []string{
		".",
		"",
		"README.md",
		".gitignore",
		".gitattributes",
		".github/workflows/ci.yml",
		".environment",
		".envelope.md",
		"notes.env",
		"env.production",
		"docs/notes.md",
	} {
		if IsProtectedPath(publishable) {
			t.Errorf("IsProtectedPath(%q) = true, want false", publishable)
		}
	}
}

// TestDiscoverAlwaysPrunesProtectedPaths pins the discovery contract: the
// protected subtrees are never listed and never entered, whatever SkipHidden
// says, while plain hidden paths follow the option.
func TestDiscoverAlwaysPrunesProtectedPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, relative := range []string{
		"README.md",
		"logo.png",
		".notes/guide.md",
		".draft.md",
		".git/config",
		".git/objects/ab/cdef",
		".ssh/id_ed25519",
		".env",
		".env.production",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(relative)), "x")
	}
	ctx := context.Background()

	hidden, err := Discover(ctx, root, DiscoverOptions{Depth: 4, SkipHidden: false})
	if err != nil {
		t.Fatalf("discover without SkipHidden: %v", err)
	}
	if got, want := entryPaths(hidden.Markdown), []string{".draft.md", ".notes/guide.md", "README.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("markdown = %v, want %v", got, want)
	}
	if got, want := entryPaths(hidden.Assets), []string{"logo.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("assets = %v, want %v (protected paths must never be listed)", got, want)
	}

	visible, err := Discover(ctx, root, DiscoverOptions{Depth: 4, SkipHidden: true})
	if err != nil {
		t.Fatalf("discover with SkipHidden: %v", err)
	}
	if got, want := entryPaths(visible.Markdown), []string{"README.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("markdown = %v, want %v", got, want)
	}
	if got, want := entryPaths(visible.Assets), []string{"logo.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("assets = %v, want %v", got, want)
	}
}

// TestDiscoverRefusesProtectedSymlinkTargets pins the canonical-target rule:
// a visibly named alias whose target sits inside a protected path publishes
// nothing, even with SkipHidden off.
func TestDiscoverRefusesProtectedSymlinkTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".env.production"), "TOKEN=1")
	writeTestFile(t, filepath.Join(root, ".git", "config"), "[core]")
	writeTestFile(t, filepath.Join(root, "README.md"), "# Readme")
	if err := os.Symlink(filepath.Join(root, ".env.production"), filepath.Join(root, "config.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".git", "config"), filepath.Join(root, "git-config.txt")); err != nil {
		t.Fatal(err)
	}

	discovered, err := Discover(context.Background(), root, DiscoverOptions{Depth: 4, SkipHidden: false})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got, want := entryPaths(discovered.Markdown), []string{"README.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("markdown = %v, want %v (the protected alias must be refused)", got, want)
	}
	if assets := entryPaths(discovered.Assets); len(assets) != 0 {
		t.Fatalf("assets = %v, want none (the protected alias must be refused)", assets)
	}
}

// TestDirectoryDocumentHonorsHiddenPolicy pins the entry-document policy:
// with SkipHidden off a hidden directory resolves like any other, protected
// directories never do, and hidden candidates only surface when allowed.
func TestDirectoryDocumentHonorsHiddenPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, directory := range []string{".notes", "topic"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if got := DirectoryDocument(root, ".notes", []string{".notes/guide.md", ".notes/.env.production"}, false); got != ".notes/guide.md" {
		t.Fatalf("hidden directory with SkipHidden off = %q, want .notes/guide.md", got)
	}
	if got := DirectoryDocument(root, ".notes", []string{".notes/guide.md"}, true); got != "" {
		t.Fatalf("hidden directory with SkipHidden on = %q, want empty", got)
	}
	if got := DirectoryDocument(root, ".git", []string{".git/README.md"}, false); got != "" {
		t.Fatalf("protected directory = %q, want empty", got)
	}
	if got := DirectoryDocument(root, "topic", []string{"topic/public.md", "topic/.env.local"}, false); got != "topic/public.md" {
		t.Fatalf("protected candidate not filtered = %q", got)
	}
}

// TestFindDirectoryDocumentHonorsHiddenPolicy pins the walking selection to
// the same policy: hidden directories open under SkipHidden off, protected
// ones stay closed, and a protected canonical target is never the entry.
func TestFindDirectoryDocumentHonorsHiddenPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".notes", "guide.md"), "# Guide")
	writeTestFile(t, filepath.Join(root, ".git", "README.md"), "# Git")
	writeTestFile(t, filepath.Join(root, "topic", ".secret.md"), "# Secret")
	if err := os.Symlink(filepath.Join(root, "topic", ".secret.md"), filepath.Join(root, "topic", "public.md")); err != nil {
		t.Fatal(err)
	}

	if got := FindDirectoryDocument(context.Background(), root, ".notes", DiscoverOptions{Depth: 4, SkipHidden: false}); got != ".notes/guide.md" {
		t.Fatalf("hidden directory with SkipHidden off = %q, want .notes/guide.md", got)
	}
	if got := FindDirectoryDocument(context.Background(), root, ".notes", DiscoverOptions{Depth: 4, SkipHidden: true}); got != "" {
		t.Fatalf("hidden directory with SkipHidden on = %q, want empty", got)
	}
	if got := FindDirectoryDocument(context.Background(), root, ".git", DiscoverOptions{Depth: 4, SkipHidden: false}); got != "" {
		t.Fatalf("protected directory = %q, want empty", got)
	}
	// The alias's visible name publishes nothing: its canonical target is
	// hidden, and SkipHidden is on here.
	if got := FindDirectoryDocument(context.Background(), root, "topic", DiscoverOptions{Depth: 4, SkipHidden: true}); got != "" {
		t.Fatalf("alias to hidden target = %q, want empty", got)
	}

	// A visible alias whose canonical target is protected stays excluded
	// even with SkipHidden off: the walk judges the resolved identity too.
	writeTestFile(t, filepath.Join(root, "leaks", ".env.production"), "TOKEN=1")
	writeTestFile(t, filepath.Join(root, "leaks", "public.md"), "# Public")
	if err := os.Symlink(filepath.Join(root, "leaks", ".env.production"), filepath.Join(root, "leaks", "alias.md")); err != nil {
		t.Fatal(err)
	}
	if got := FindDirectoryDocument(context.Background(), root, "leaks", DiscoverOptions{Depth: 4, SkipHidden: false}); got != "leaks/public.md" {
		t.Fatalf("entry with a protected canonical alias = %q, want leaks/public.md", got)
	}
}
