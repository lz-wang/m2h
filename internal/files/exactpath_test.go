package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRequireExactPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "file.md"), []byte("# F"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		relative string
		want     error
	}{
		{name: "existing file", relative: "sub/file.md", want: nil},
		{name: "existing directory", relative: "sub", want: nil},
		{name: "missing component", relative: "sub/missing.md", want: ErrExactPathMissing},
		{name: "missing root component", relative: "missing.md", want: ErrExactPathMissing},
		{name: "symlink file is allowed", relative: "linked", want: nil},
		{name: "symlink directory is refused", relative: "linked/file.md", want: ErrExactPathSymlink},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := RequireExactPath(root, test.relative)
			if !errors.Is(err, test.want) {
				t.Fatalf("RequireExactPath(%q) error = %v, want %v", test.relative, err, test.want)
			}
		})
	}
}

func TestRequireExactPathMissingDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	err := RequireExactPath(filepath.Join(root, "missing"), "file.md")
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want a not-exist failure reading the directory", err)
	}
}
