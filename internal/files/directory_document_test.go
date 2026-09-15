package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryDocumentSelection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "topic"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		visible []string
		want    string
	}{
		{"README first", []string{"topic/index.md", "topic/README.md", "topic/00-index.md"}, "topic/README.md"},
		{"index next", []string{"topic/a.md", "topic/INDEX.markdown", "topic/00-index.md"}, "topic/INDEX.markdown"},
		{"numbered index", []string{"topic/a.md", "topic/00-index.md"}, "topic/00-index.md"},
		{"sorted fallback", []string{"topic/z.md", "topic/b.md", "topic/a.md"}, "topic/a.md"},
		{"descendant fallback excludes siblings", []string{"topic/sub/a.md", "other/README.md"}, "topic/sub/a.md"},
		{"shallow descendant first", []string{"topic/a/deep/a.md", "topic/z/b.md", "topic/z/a.md"}, "topic/z/a.md"},
		{"hidden children excluded", []string{"topic/.hidden.md", "topic/.secret/a.md"}, ""},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DirectoryDocument(root, "topic", tt.visible); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
	if got := DirectoryDocument(root, ".", []string{"README.md"}); got != "README.md" {
		t.Fatal(got)
	}
	if got := DirectoryDocument(root, "absent", []string{"absent/README.md"}); got != "" {
		t.Fatal(got)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DirectoryDocument(root, "file", []string{"file/README.md"}); got != "" {
		t.Fatal(got)
	}
	if err := os.Symlink(filepath.Join(root, "topic"), filepath.Join(root, "alias")); err != nil {
		t.Skip(err)
	}
	if got := DirectoryDocument(root, "alias", []string{"alias/README.md"}); got != "" {
		t.Fatal("directory symlink accepted")
	}
}

// TestFindDirectoryDocumentSelection pins the filesystem-walking selection to
// the same outcomes the visible-list based DirectoryDocument has always
// produced, including the discovery boundaries it must respect.
func TestFindDirectoryDocumentSelection(t *testing.T) {
	t.Parallel()
	options := DiscoverOptions{Depth: 4, SkipHidden: true}
	tests := []struct {
		name      string
		directory string
		layout    []string
		want      string
		alias     bool
	}{
		{
			name:      "README outranks index and numbered index",
			directory: "topic",
			layout:    []string{"topic/index.md", "topic/README.md", "topic/00-index.md"},
			want:      "topic/README.md",
		},
		{
			name:      "index outranks numbered index",
			directory: "topic",
			layout:    []string{"topic/a.md", "topic/INDEX.markdown", "topic/00-index.md"},
			want:      "topic/INDEX.markdown",
		},
		{
			name:      "numbered index outranks plain documents",
			directory: "topic",
			layout:    []string{"topic/a.md", "topic/00-index.md"},
			want:      "topic/00-index.md",
		},
		{
			name:      "lexically first direct document without a conventional name",
			directory: "topic",
			layout:    []string{"topic/z.md", "topic/b.md", "topic/a.md"},
			want:      "topic/a.md",
		},
		{
			name:      "non-Markdown files never become the entry",
			directory: "topic",
			layout:    []string{"topic/LICENSE"},
			want:      "",
		},
		{
			name:      "shallowest descendant wins over a lexically smaller deeper one",
			directory: "topic",
			layout:    []string{"topic/a/deep/a.md", "topic/z/b.md", "topic/z/a.md"},
			want:      "topic/z/a.md",
		},
		{
			name:      "lexical order breaks same-level descendant ties",
			directory: "topic",
			layout:    []string{"topic/x/deep.md", "topic/a/deep.md"},
			want:      "topic/a/deep.md",
		},
		{
			name:      "hidden documents are excluded",
			directory: "topic",
			layout:    []string{"topic/.hidden.md", "topic/.secret/a.md"},
			want:      "",
		},
		{
			name:      "a visible alias to a hidden target stays excluded",
			directory: "topic",
			layout:    []string{"topic/.secret.md"},
			want:      "",
			alias:     true,
		},
		{
			name:      "the workspace root resolves like any directory",
			directory: ".",
			layout:    []string{"README.md", "a.md"},
			want:      "README.md",
		},
		{
			name:      "an absent directory resolves to nothing",
			directory: "absent",
			layout:    []string{"README.md"},
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, relative := range tt.layout {
				absolute := filepath.Join(root, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(absolute, []byte("# x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.alias {
				// A visible alias to the hidden original: the publishing
				// policy judges the canonical target, so the alias is not a
				// document despite its own visible name.
				if err := os.Symlink(
					filepath.Join(root, "topic", ".secret.md"),
					filepath.Join(root, "topic", "public.md"),
				); err != nil {
					t.Skip(err)
				}
			}
			got := FindDirectoryDocument(t.Context(), root, tt.directory, options)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestFindDirectoryDocumentRespectsScopeBoundaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, relative := range []string{
		"topic/deep/README.md",
		"topic/other.md",
		"outside/README.md",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A depth limit below the direct document's depth yields nothing.
	if got := FindDirectoryDocument(t.Context(), root, "topic", DiscoverOptions{Depth: 0, SkipHidden: true}); got != "" {
		t.Fatalf("depth limit ignored: %q", got)
	}
	// At the limit itself the direct document still wins.
	if got := FindDirectoryDocument(t.Context(), root, "topic", DiscoverOptions{Depth: 1, SkipHidden: true}); got != "topic/other.md" {
		t.Fatalf("direct pick at depth limit = %q", got)
	}

	// A glob restricts the candidates like it restricts Discover.
	globbed := DiscoverOptions{Depth: 4, SkipHidden: true, Pattern: "**/deep/*.md"}
	if got := FindDirectoryDocument(t.Context(), root, "topic", globbed); got != "topic/deep/README.md" {
		t.Fatalf("glob filter = %q", got)
	}

	// A directory symlink is never followed, even when it would offer an
	// entry document.
	if err := os.Symlink(filepath.Join(root, "outside"), filepath.Join(root, "topic", "alias")); err != nil {
		t.Skip(err)
	}
	if got := FindDirectoryDocument(t.Context(), root, "topic", DiscoverOptions{Depth: 4, SkipHidden: true}); got != "topic/other.md" {
		t.Fatalf("directory symlink followed: %q", got)
	}

	// A file symlink within the root is a publishable alias.
	if err := os.Symlink(filepath.Join(root, "topic", "other.md"), filepath.Join(root, "topic", "link.md")); err != nil {
		t.Skip(err)
	}
	if got := FindDirectoryDocument(t.Context(), root, "topic", DiscoverOptions{Depth: 4, SkipHidden: true}); got != "topic/link.md" {
		t.Fatalf("file symlink alias = %q", got)
	}

	// A cancelled context aborts the walk.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := FindDirectoryDocument(ctx, root, "topic", DiscoverOptions{Depth: 4, SkipHidden: true, Pattern: "**/deep/*.md"}); got != "" {
		t.Fatalf("cancelled walk returned %q", got)
	}
}
