package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrExactPathMissing reports a relative path component that does not exist
// with exactly the requested name beneath its parent directory. On
// case-insensitive filesystems a differently-cased file exists, but the
// served workspace would refuse it, so callers treat this as absent.
var ErrExactPathMissing = errors.New("path component does not exist with exact case")

// ErrExactPathSymlink reports a relative path whose traversal crosses a
// symlink directory. Symlinked files are fine; directory symlinks are not
// followed so a workspace can never serve through them.
var ErrExactPathSymlink = errors.New("path traverses a symlink directory")

// RequireExactPath verifies that every slash-separated segment of relative
// exists directly beneath the previous one with exactly the requested name,
// and that no intermediate segment is a symlink directory. It is the shared
// filesystem boundary of the document server and the check command: what it
// rejects can never be reached in the WebUI, whatever the host filesystem
// would say on its own.
func RequireExactPath(root, relative string) error {
	current := root
	segments := strings.Split(relative, "/")
	for index, segment := range segments {
		entries, err := os.ReadDir(current)
		if err != nil {
			return fmt.Errorf("read directory %q: %w", current, err)
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == segment {
				if index < len(segments)-1 && entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("path %q: %w", relative, ErrExactPathSymlink)
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("path %q: %w", relative, ErrExactPathMissing)
		}
		current = filepath.Join(current, segment)
	}
	return nil
}
