package server

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// directoryDocumentResolver resolves directory links for one rendered
// response. The entry document of a directory is found by walking only that
// directory's own subtree (files.FindDirectoryDocument) with the root's
// discovery rules — it no longer unconditionally discovers the whole
// workspace, though a link into a very large directory can still scan a
// large subtree. Decisions are cached per response: the same Markdown may
// reference one directory repeatedly. It never broadens serving scope.
func (handler *documentHandler) directoryDocumentResolver(ctx context.Context, root workspaceRoot) func(string) (string, bool) {
	type decision struct {
		document  string
		directory bool
	}
	cache := make(map[string]decision)
	return func(encoded string) (string, bool) {
		if root.scope.isSingleFile() {
			return "", false
		}
		if result, ok := cache[encoded]; ok {
			return result.document, result.directory
		}
		cache[encoded] = decision{}
		prefix := handler.workspace.publicRoot(root.id)
		relative := encoded
		if prefix != "" {
			if encoded == prefix {
				relative = "."
			} else {
				var ok bool
				relative, ok = strings.CutPrefix(encoded, prefix+"/")
				if !ok {
					return "", false
				}
			}
		}
		directory := "."
		if relative != "." && relative != "" {
			var err error
			directory, err = files.DecodeRelativePath(relative)
			if err != nil {
				return "", false
			}
		}
		if directory != "." {
			if err := files.RequireExactPath(root.scope.root, directory); err != nil {
				return "", false
			}
		}
		info, err := os.Lstat(filepath.Join(root.scope.root, filepath.FromSlash(directory)))
		if err != nil || !info.IsDir() {
			return "", false
		}
		cache[encoded] = decision{directory: true}
		// The BFS must judge the same publishable set as the document
		// routes: the scope carries no matcher by design (rules are read
		// per request), so the walk without it would pick an ignored
		// README.md over a publishable index.md and the admission check
		// below would refuse the pick, killing the directory link entirely.
		// A rule file that cannot be read degrades the link to unresolved
		// here; the document routes still answer 500 for the same state.
		matcher, err := root.scope.ignoreSnapshot()
		if err != nil {
			return "", false
		}
		discovery := root.scope.discovery
		discovery.Ignore = matcher
		document := files.FindDirectoryDocument(ctx, root.scope.root, directory, discovery)
		if document == "" {
			return "", true
		}
		public := handler.workspace.publicPath(root.id, document)
		if _, _, _, err := handler.resolveVisibleDocument(public); err != nil {
			return "", true
		}
		parsed := url.URL{Path: public}
		cache[encoded] = decision{document: parsed.EscapedPath(), directory: true}
		return parsed.EscapedPath(), true
	}
}
