package files

import (
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
