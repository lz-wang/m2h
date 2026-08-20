package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lz-wang/m2h/internal/files"
)

// previewScope expresses the access boundary of one preview session. A single
// Markdown file becomes a scope rooted at its parent directory (so sibling
// assets stay reachable) that admits only that one file; a directory scope
// admits every Markdown file a DiscoverOptions query matches beneath the root.
type previewScope struct {
	root      string
	file      string // normalized relative path; empty means directory scope
	discovery files.DiscoverOptions
}

// previewKind tells the WebUI whether navigation is meaningful: a single-file
// scope has nothing to switch between, so the file sidebar is hidden.
type previewKind string

const (
	previewSingle        previewKind = "single"
	previewDirectory     previewKind = "directory"
	previewWorkspaceKind previewKind = "workspace"
)

func (scope previewScope) kind() previewKind {
	if scope.isSingleFile() {
		return previewSingle
	}
	return previewDirectory
}

// newPreviewScope builds the scope for a resolved input. The single-file name
// is kept literally and never reinterpreted as a glob, so files named with
// glob metacharacters (foo[1].md, foo*.md) remain addressable.
func newPreviewScope(input files.Input, discovery files.DiscoverOptions) previewScope {
	if input.Kind == files.KindFile {
		return previewScope{
			root: filepath.Dir(input.Path),
			file: files.NormalizeRelativePath(filepath.Base(input.Path)),
		}
	}
	return previewScope{
		root:      input.Path,
		discovery: discovery,
	}
}

func (scope previewScope) isSingleFile() bool {
	return scope.file != ""
}

// discover returns the Markdown entries visible to the scope.
func (scope previewScope) discover(ctx context.Context) (files.Discovery, error) {
	if !scope.isSingleFile() {
		return files.Discover(ctx, scope.root, scope.discovery)
	}

	target := filepath.Join(scope.root, filepath.FromSlash(scope.file))
	info, err := os.Stat(target)
	if err != nil {
		return files.Discovery{}, err
	}
	if !info.Mode().IsRegular() {
		return files.Discovery{}, fmt.Errorf("preview document %q is not a regular file", scope.file)
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
func (scope previewScope) allowsDocument(relative string) bool {
	if scope.isSingleFile() {
		return relative == scope.file
	}
	return files.IsMarkdown(relative) && files.Matches(relative, scope.discovery)
}
