package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lz-wang/m2h/internal/files"
)

// rootScope expresses the access boundary of one served workspace. A single
// Markdown file becomes a scope rooted at its parent directory (so sibling
// assets stay reachable) that admits only that one file; a directory scope
// admits every Markdown file a DiscoverOptions query matches beneath the root.
type rootScope struct {
	root      string
	file      string // normalized relative path; empty means directory scope
	discovery files.DiscoverOptions
}

// workspaceKind tells the WebUI whether navigation is meaningful: a single-file
// scope has nothing to switch between, so the file sidebar is hidden.
type workspaceKind string

const (
	workspaceSingle        workspaceKind = "single"
	workspaceDirectory     workspaceKind = "directory"
	workspaceMultiRootKind workspaceKind = "workspace"
)

func (scope rootScope) kind() workspaceKind {
	if scope.isSingleFile() {
		return workspaceSingle
	}
	return workspaceDirectory
}

// newRootScope builds the scope for a resolved input. The single-file name
// is kept literally and never reinterpreted as a glob, so files named with
// glob metacharacters (foo[1].md, foo*.md) remain addressable.
func newRootScope(input files.Input, discovery files.DiscoverOptions) rootScope {
	if input.Kind == files.KindFile {
		return rootScope{
			root: filepath.Dir(input.Path),
			file: files.NormalizeRelativePath(filepath.Base(input.Path)),
		}
	}
	return rootScope{
		root:      input.Path,
		discovery: discovery,
	}
}

func (scope rootScope) isSingleFile() bool {
	return scope.file != ""
}

// discover returns the Markdown entries visible to the scope.
func (scope rootScope) discover(ctx context.Context) (files.Discovery, error) {
	if !scope.isSingleFile() {
		return files.Discover(ctx, scope.root, scope.discovery)
	}

	target := filepath.Join(scope.root, filepath.FromSlash(scope.file))
	info, err := os.Stat(target)
	if err != nil {
		return files.Discovery{}, err
	}
	if !info.Mode().IsRegular() {
		return files.Discovery{}, fmt.Errorf("workspace document %q is not a regular file", scope.file)
	}

	return files.Discovery{
		Markdown: []files.Entry{
			{
				AbsolutePath: target,
				RelativePath: scope.file,
				Mode:         info.Mode(),
			},
		},
	}, nil
}

// allowsDocument reports whether a normalized relative path is reachable
// through the scope. It is the single authority that guards /api/document.
func (scope rootScope) allowsDocument(relative string) bool {
	if scope.isSingleFile() {
		return relative == scope.file
	}
	return files.IsMarkdown(relative) && files.Matches(relative, scope.discovery)
}
