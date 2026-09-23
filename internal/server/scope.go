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
	// ignore records whether a directory scope consults the root's
	// .gitignore rules. The matcher itself is never stored: every check
	// builds a request-scoped snapshot (see ignoreSnapshot), so rule edits
	// take effect on the next request without a cache or a watcher.
	ignore bool
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
// glob metacharacters (foo[1].md, foo*.md) remain addressable. ignore only
// ever applies to directory scopes: naming a file on the command line is an
// explicit publishing act, so a single-file scope never consults rules.
func newRootScope(input files.Input, discovery files.DiscoverOptions, ignore bool) rootScope {
	if input.Kind == files.KindFile {
		return rootScope{
			root: filepath.Dir(input.Path),
			file: files.NormalizeRelativePath(filepath.Base(input.Path)),
		}
	}
	return rootScope{
		root:      input.Path,
		discovery: discovery,
		ignore:    ignore,
	}
}

func (scope rootScope) isSingleFile() bool {
	return scope.file != ""
}

// ignoreSnapshot builds the .gitignore matcher for one access decision or
// one discovery. Returning a nil matcher is the disabled case — a
// single-file scope or a root with ignore off — which Discover and the
// policy checks below treat as "no ignore rules".
func (scope rootScope) ignoreSnapshot() (files.IgnoreMatcher, error) {
	if scope.isSingleFile() || !scope.ignore {
		return nil, nil
	}
	matcher, err := files.NewGitIgnore(scope.root)
	if err != nil {
		return nil, err
	}
	return matcher, nil
}

// discover returns the Markdown entries visible to the scope.
func (scope rootScope) discover(ctx context.Context) (files.Discovery, error) {
	if !scope.isSingleFile() {
		matcher, err := scope.ignoreSnapshot()
		if err != nil {
			return files.Discovery{}, err
		}
		options := scope.discovery
		options.Ignore = matcher
		return files.Discover(ctx, scope.root, options)
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
// discovery's SkipHidden so the file tree and document admission never drift,
// while protected paths (.git, .ssh, .env) are refused whatever the option.
func (scope rootScope) allowsDocument(relative string) bool {
	if scope.isSingleFile() {
		return relative == scope.file
	}
	if files.IsProtectedPath(relative) {
		return false
	}
	if scope.discovery.SkipHidden && files.IsHiddenPath(relative) {
		return false
	}
	return files.IsMarkdown(relative) && files.Matches(relative, scope.discovery)
}

// allowsResolvedDocument re-checks only the security properties — a hidden
// or protected canonical target — after filesystem resolution. It
// deliberately skips the glob/depth rules: those belong to the alias path the
// reader addressed, so a shallow symlink to a deeper document keeps serving
// exactly as before. A single-file scope serves its explicitly named input
// whatever it resolves through, so there is nothing left to refuse.
func (scope rootScope) allowsResolvedDocument(relative string) bool {
	if scope.isSingleFile() {
		return true
	}
	if files.IsProtectedPath(relative) {
		return false
	}
	return !scope.discovery.SkipHidden || !files.IsHiddenPath(relative)
}

// allowsAsset reports whether a normalized relative path may be served
// through /assets. Markdown files belong to the document routes only, active
// web documents (HTML/JS/CSS) must not become same-origin content on the m2h
// origin, and protected paths are never publishable — every other regular
// file is an ordinary passive attachment. Hidden paths follow the same
// SkipHidden option as documents, with one guard: a single-file scope has no
// --hidden decision of its own, so its neighborhood keeps the default
// filtering instead of quietly widening the explicit input's reach.
func (scope rootScope) allowsAsset(relative string) bool {
	if files.IsMarkdown(relative) {
		return false
	}
	if files.IsProtectedPath(relative) {
		return false
	}
	if (scope.isSingleFile() || scope.discovery.SkipHidden) && files.IsHiddenPath(relative) {
		return false
	}
	return !files.IsActiveWebAsset(relative)
}

// The policy checks below layer the root's ignore rules on top of the
// boolean admission helpers above. They answer (allowed, error) instead of
// folding everything into a bool: a rule file that cannot be read or parsed
// is a filesystem failure the HTTP layer must report as 500, not a reason
// to quietly admit or refuse the request.

// admittedByPolicy extends allowsDocument with the root's ignore rules,
// judging the alias path the reader addressed. Single-file scopes and roots
// with ignore disabled pass through unchanged.
func (scope rootScope) admittedByPolicy(relative string) (bool, error) {
	if !scope.allowsDocument(relative) {
		return false, nil
	}
	if scope.isSingleFile() {
		return true, nil
	}
	matcher, err := scope.ignoreSnapshot()
	if err != nil {
		return false, err
	}
	if matcher == nil {
		return true, nil
	}
	ignored, err := matcher.Ignored(relative, false)
	if err != nil {
		return false, err
	}
	return !ignored, nil
}

// resolvedAdmittedByPolicy extends allowsResolvedDocument the same way: the
// canonical target of an accepted alias must itself stay publishable, so a
// visible symlink cannot hand out an ignored file. Only the ignore layer is
// re-judged here — glob/depth keep belonging to the alias path.
func (scope rootScope) resolvedAdmittedByPolicy(resolvedRelative string) (bool, error) {
	if !scope.allowsResolvedDocument(resolvedRelative) {
		return false, nil
	}
	if scope.isSingleFile() {
		return true, nil
	}
	matcher, err := scope.ignoreSnapshot()
	if err != nil {
		return false, err
	}
	if matcher == nil {
		return true, nil
	}
	ignored, err := matcher.Ignored(resolvedRelative, false)
	if err != nil {
		return false, err
	}
	return !ignored, nil
}

// assetAdmittedByPolicy extends allowsAsset with the root's ignore rules.
// Assets carry no glob/depth semantics, so the alias check and the resolved
// check call this method twice — once per identity — just like allowsAsset.
func (scope rootScope) assetAdmittedByPolicy(relative string) (bool, error) {
	if !scope.allowsAsset(relative) {
		return false, nil
	}
	if scope.isSingleFile() {
		return true, nil
	}
	matcher, err := scope.ignoreSnapshot()
	if err != nil {
		return false, err
	}
	if matcher == nil {
		return true, nil
	}
	ignored, err := matcher.Ignored(relative, false)
	if err != nil {
		return false, err
	}
	return !ignored, nil
}
