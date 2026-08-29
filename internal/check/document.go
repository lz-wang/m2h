package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lz-wang/m2h/internal/files"
)

// document is one Markdown file inside the checked scope. relative is the
// normalized slash path the serve command would address the document by,
// absolute is its filesystem location, and display is the path shown in
// diagnostics: the input as the user wrote it, joined with the relative path,
// so path:line:column stays resolvable from the working directory.
type document struct {
	relative string
	absolute string
	display  string
}

// documentScope mirrors the serve command's rootScope: a single Markdown file
// becomes a scope rooted at its parent directory that admits only that one
// document (sibling assets stay reachable), while a directory scope admits
// every Markdown file the discovery query matches beneath the root. Keeping
// the scope rules identical means check and the WebUI can never disagree
// about which documents exist.
type documentScope struct {
	root      string
	single    bool
	file      string // single-file scope: the only admitted document
	discovery files.DiscoverOptions
	documents []document
}

// newSingleFileScope builds the scope for a resolved Markdown file input. The
// file's name is kept literally and never reinterpreted as a glob, so files
// named with glob metacharacters remain checkable.
func newSingleFileScope(input string, resolved string) (documentScope, error) {
	root := filepath.Dir(resolved)
	relative := files.NormalizeRelativePath(filepath.Base(resolved))
	info, err := os.Stat(resolved)
	if err != nil {
		return documentScope{}, fmt.Errorf("inspect %q: %w", resolved, err)
	}
	if !info.Mode().IsRegular() {
		return documentScope{}, fmt.Errorf("workspace document %q is not a regular file", relative)
	}
	return documentScope{
		root:      root,
		single:    true,
		file:      relative,
		documents: []document{newDocument(root, filepath.Dir(input), relative)},
	}, nil
}

// newDirectoryScope builds the scope for a resolved directory input by
// reusing the serve command's discovery walk, so depth, glob and symlink
// safety behave identically in both commands.
func newDirectoryScope(ctx context.Context, input string, resolved string, discovery files.DiscoverOptions) (documentScope, error) {
	found, err := files.Discover(ctx, resolved, discovery)
	if err != nil {
		return documentScope{}, err
	}
	scope := documentScope{
		root:      resolved,
		discovery: discovery,
		documents: make([]document, 0, len(found.Markdown)),
	}
	for _, entry := range found.Markdown {
		scope.documents = append(scope.documents, newDocument(resolved, input, entry.RelativePath))
	}
	return scope, nil
}

func newDocument(root string, inputDir string, relative string) document {
	return document{
		relative: relative,
		absolute: filepath.Join(root, filepath.FromSlash(relative)),
		display:  filepath.Join(inputDir, filepath.FromSlash(relative)),
	}
}
