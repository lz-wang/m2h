package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
		if errors.Is(err, fs.ErrNotExist) {
			return files.Discovery{}, nil
		}
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
// A single-file scope serves its explicit input whatever the file is named —
// naming a hidden file on the command line is an explicit publishing act, not
// an accidental exposure by directory discovery. A directory scope honors the
// discovery's SkipHidden so the file tree and document admission never drift.
func (scope rootScope) allowsDocument(relative string) bool {
	if scope.isSingleFile() {
		return relative == scope.file
	}
	if scope.discovery.SkipHidden && files.IsHiddenPath(relative) {
		return false
	}
	return files.IsMarkdown(relative) && files.Matches(relative, scope.discovery)
}

// allowsResolvedDocument re-checks only the security property — a hidden
// canonical target — after filesystem resolution. It deliberately skips the
// glob/depth rules: those belong to the alias path the reader addressed, so
// a shallow symlink to a deeper document keeps serving exactly as before.
// A single-file scope serves its explicitly named input whatever it resolves
// through, so there is nothing left to refuse.
func (scope rootScope) allowsResolvedDocument(relative string) bool {
	if scope.isSingleFile() {
		return true
	}
	return !scope.discovery.SkipHidden || !files.IsHiddenPath(relative)
}

// allowsAsset reports whether a normalized relative path may be served
// through /assets. Markdown files belong to the document routes only, hidden
// paths are never publishable, and active web documents (HTML/JS/CSS) must
// not become same-origin content on the m2h origin — every other regular
// file is an ordinary passive attachment.
func (scope rootScope) allowsAsset(relative string) bool {
	if files.IsMarkdown(relative) {
		return false
	}
	if files.IsHiddenPath(relative) {
		return false
	}
	return !isActiveWebAsset(relative)
}
