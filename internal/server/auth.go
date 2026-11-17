package server

import (
	"fmt"

	"github.com/lz-wang/m2h/internal/files"
)

// checkDataDirBoundary refuses a data directory that overlaps any served
// root's tree. The SQLite files (including write-ahead logs) must never sit
// inside a published document or attachment scope, nor may a root live
// inside the data directory where session and token rows would surface as
// reachable paths. Both directions are judged on canonical paths so
// symlinked roots cannot smuggle an overlap past the check.
func checkDataDirBoundary(dataDir string, workspace workspace) error {
	canonical, err := files.CanonicalPath(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data directory %q: %w", dataDir, err)
	}
	for _, root := range workspace.roots {
		boundary := root.scope.root
		if files.IsWithin(boundary, canonical) {
			return fmt.Errorf("data directory %q must not live inside served root %q", dataDir, root.input.Path)
		}
		if files.IsWithin(canonical, boundary) {
			return fmt.Errorf("served root %q must not live inside data directory %q", root.input.Path, dataDir)
		}
	}
	return nil
}
