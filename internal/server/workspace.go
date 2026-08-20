package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// previewRoot couples one resolved CLI input with its previewScope and the
// identity the preview addresses it by. The id is positional (r0, r1, …) and
// stable for the lifetime of the server; the label is the display name shown
// in the sidebar. Labels may collide across machines and users; ids never do.
type previewRoot struct {
	id    string
	label string
	input files.Input
	scope previewScope
}

// previewWorkspace is the whole preview session: one or more independent
// roots, each keeping its own access boundary. Roots are never merged into a
// common filesystem parent — a request resolved inside one root can never
// reach another root's tree, so adding roots never widens the serving
// boundary beyond the directories the user named.
type previewWorkspace struct {
	roots []previewRoot
}

// newPreviewWorkspace builds the workspace from resolved inputs in the order
// the CLI received them; the first root is the primary root. Inputs that
// canonicalize to the same tree are rejected: two ids for one directory would
// double-serve identical documents and confuse both the sidebar and the
// default-document selection.
func newPreviewWorkspace(inputs []files.Input, discovery files.DiscoverOptions) (previewWorkspace, error) {
	roots := make([]previewRoot, 0, len(inputs))
	seen := make(map[string]string, len(inputs))
	labelCounts := make(map[string]int, len(inputs))
	for index, input := range inputs {
		scope := newPreviewScope(input, discovery)
		identity := scope.root
		if scope.isSingleFile() {
			identity = filepath.Join(scope.root, filepath.FromSlash(scope.file))
		}
		if previous, exists := seen[identity]; exists {
			return previewWorkspace{}, fmt.Errorf("duplicate preview root %q: same file tree as %q", input.Path, previous)
		}
		seen[identity] = input.Path

		base := filepath.Base(input.Path)
		labelCounts[base]++
		label := base
		if labelCounts[base] > 1 {
			label = fmt.Sprintf("%s (%d)", base, labelCounts[base])
		}

		roots = append(roots, previewRoot{
			id:    fmt.Sprintf("r%d", index),
			label: label,
			input: input,
			scope: scope,
		})
	}
	return previewWorkspace{roots: roots}, nil
}

// rootCount reports how many roots the workspace serves.
func (workspace previewWorkspace) rootCount() int {
	return len(workspace.roots)
}

// root looks a root up by its id.
func (workspace previewWorkspace) root(id string) (previewRoot, bool) {
	for _, root := range workspace.roots {
		if root.id == id {
			return root, true
		}
	}
	return previewRoot{}, false
}

// kind reports the API kind. A lone single-file root stays "single" and a
// lone directory root stays "directory" — existing URLs and UI behavior are
// unchanged — while every multi-root workspace is "workspace".
func (workspace previewWorkspace) kind() previewKind {
	if workspace.rootCount() > 1 {
		return previewWorkspaceKind
	}
	return workspace.primary().scope.kind()
}

// publicPath composes the addressable path for a root-relative document:
// prefixed with the root id only in a multi-root workspace, so single-root
// URLs (/doc/foo.md, /assets/foo.png) keep their long-standing shape.
func (workspace previewWorkspace) publicPath(rootID, relative string) string {
	if workspace.rootCount() <= 1 {
		return relative
	}
	return rootID + "/" + relative
}

// locate maps an addressable (virtual) path back onto its root and the
// root-relative remainder. With one root the whole path belongs to it; with
// several, the first segment must name a root — a path without a known root
// id never falls back into another root's tree.
func (workspace previewWorkspace) locate(virtual string) (previewRoot, string, error) {
	if workspace.rootCount() <= 1 {
		return workspace.primary(), virtual, nil
	}
	id, relative, found := strings.Cut(virtual, "/")
	if !found {
		return previewRoot{}, "", fmt.Errorf("path %q names no preview root", virtual)
	}
	root, ok := workspace.root(id)
	if !ok {
		return previewRoot{}, "", fmt.Errorf("path %q names no preview root", virtual)
	}
	return root, relative, nil
}

// singleRootWorkspace wraps one scope as the workspace's sole root (id r0).
// Production always goes through newPreviewWorkspace from resolved inputs;
// this adapter keeps single-scope call sites and tests unchanged in shape.
func singleRootWorkspace(scope previewScope) previewWorkspace {
	return previewWorkspace{roots: []previewRoot{{id: "r0", label: "", scope: scope}}}
}

// primary returns the first root. The workspace is built from a non-empty
// input list (normalizeOptions rejects empty input before construction), so
// there is always a primary.
func (workspace previewWorkspace) primary() previewRoot {
	return workspace.roots[0]
}

// anyDirectory reports whether at least one root previews a directory tree.
func (workspace previewWorkspace) anyDirectory() bool {
	for _, root := range workspace.roots {
		if root.input.Kind == files.KindDirectory {
			return true
		}
	}
	return false
}

// singleFilePaths lists the absolute paths of the single-file roots — the
// files the change watcher should observe. Directory roots are not watched.
func (workspace previewWorkspace) singleFilePaths() []string {
	paths := make([]string, 0, len(workspace.roots))
	for _, root := range workspace.roots {
		if root.input.Kind == files.KindFile {
			paths = append(paths, root.input.Path)
		}
	}
	return paths
}

// inputPaths lists every root's input path in root order.
func (workspace previewWorkspace) inputPaths() []string {
	paths := make([]string, 0, len(workspace.roots))
	for _, root := range workspace.roots {
		paths = append(paths, root.input.Path)
	}
	return paths
}
