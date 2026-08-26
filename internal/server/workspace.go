package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// workspaceRoot couples one resolved CLI input with its rootScope and the
// identity the workspace addresses it by. The id is positional (r0, r1, …) and
// stable for the lifetime of the server; the label is the display name shown
// in the sidebar. Labels may collide across machines and users; ids never do.
type workspaceRoot struct {
	id    string
	label string
	input files.Input
	scope rootScope
}

// workspace is the whole serving session: one or more independent
// roots, each keeping its own access boundary. Roots are never merged into a
// common filesystem parent — a request resolved inside one root can never
// reach another root's tree, so adding roots never widens the serving
// boundary beyond the directories the user named.
type workspace struct {
	roots []workspaceRoot
}

// newWorkspace builds the workspace from resolved inputs in the order
// the CLI received them; the first root is the primary root. Inputs that
// canonicalize to the same tree are rejected: two ids for one directory would
// double-serve identical documents and confuse both the sidebar and the
// default-document selection.
func newWorkspace(inputs []files.Input, discovery files.DiscoverOptions) (workspace, error) {
	roots := make([]workspaceRoot, 0, len(inputs))
	seen := make(map[string]string, len(inputs))
	labelCounts := make(map[string]int, len(inputs))
	for index, input := range inputs {
		scope := newRootScope(input, discovery)
		identity := scope.root
		if scope.isSingleFile() {
			identity = filepath.Join(scope.root, filepath.FromSlash(scope.file))
		}
		if previous, exists := seen[identity]; exists {
			return workspace{}, fmt.Errorf("duplicate workspace root %q: same file tree as %q", input.Path, previous)
		}
		seen[identity] = input.Path

		base := filepath.Base(input.Path)
		labelCounts[base]++
		label := base
		if labelCounts[base] > 1 {
			label = fmt.Sprintf("%s (%d)", base, labelCounts[base])
		}

		roots = append(roots, workspaceRoot{
			id:    fmt.Sprintf("r%d", index),
			label: label,
			input: input,
			scope: scope,
		})
	}
	return workspace{roots: roots}, nil
}

// rootCount reports how many roots the workspace serves.
func (workspace workspace) rootCount() int {
	return len(workspace.roots)
}

// root looks a root up by its id.
func (workspace workspace) root(id string) (workspaceRoot, bool) {
	for _, root := range workspace.roots {
		if root.id == id {
			return root, true
		}
	}
	return workspaceRoot{}, false
}

// kind reports the API kind. A lone single-file root stays "single" and a
// lone directory root stays "directory" — existing URLs and UI behavior are
// unchanged — while every multi-root workspace is "workspace".
func (workspace workspace) kind() workspaceKind {
	if workspace.rootCount() > 1 {
		return workspaceMultiRootKind
	}
	return workspace.primary().scope.kind()
}

// publicPath composes the addressable path for a root-relative document:
// prefixed with the root id only in a multi-root workspace, so single-root
// URLs (/doc/foo.md, /assets/foo.png) keep their long-standing shape.
func (workspace workspace) publicPath(rootID, relative string) string {
	if workspace.rootCount() <= 1 {
		return relative
	}
	return rootID + "/" + relative
}

// locate maps an addressable (virtual) path back onto its root and the
// root-relative remainder. With one root the whole path belongs to it; with
// several, the first segment must name a root — a path without a known root
// id never falls back into another root's tree.
func (workspace workspace) locate(virtual string) (workspaceRoot, string, error) {
	if workspace.rootCount() <= 1 {
		return workspace.primary(), virtual, nil
	}
	id, relative, found := strings.Cut(virtual, "/")
	if !found {
		return workspaceRoot{}, "", fmt.Errorf("path %q names no workspace root", virtual)
	}
	root, ok := workspace.root(id)
	if !ok {
		return workspaceRoot{}, "", fmt.Errorf("path %q names no workspace root", virtual)
	}
	return root, relative, nil
}

// singleRootWorkspace wraps one scope as the workspace's sole root (id r0).
// Production always goes through newWorkspace from resolved inputs;
// this adapter keeps single-scope call sites and tests unchanged in shape.
func singleRootWorkspace(scope rootScope) workspace {
	return workspace{roots: []workspaceRoot{{id: "r0", label: "", scope: scope}}}
}

// primary returns the first root. The workspace is built from a non-empty
// input list (normalizeOptions rejects empty input before construction), so
// there is always a primary.
func (workspace workspace) primary() workspaceRoot {
	return workspace.roots[0]
}

// anyDirectory reports whether at least one root serves a directory tree.
func (workspace workspace) anyDirectory() bool {
	for _, root := range workspace.roots {
		if root.input.Kind == files.KindDirectory {
			return true
		}
	}
	return false
}

// inputPaths lists every root's input path in root order.
func (workspace workspace) inputPaths() []string {
	paths := make([]string, 0, len(workspace.roots))
	for _, root := range workspace.roots {
		paths = append(paths, root.input.Path)
	}
	return paths
}
