package server

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lz-wang/m2h/internal/files"
)

// directoryDocumentResolver lazily discovers only the referencing root and
// caches directory decisions for one response. It never broadens serving scope.
func (handler *documentHandler) directoryDocumentResolver(ctx context.Context, root workspaceRoot) func(string) (string, bool) {
	var visible []string
	loaded := false
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
		if !loaded {
			loaded = true
			found, err := root.scope.discover(ctx)
			if err != nil {
				return "", true
			}
			for _, entry := range found.Markdown {
				visible = append(visible, entry.RelativePath)
			}
		}
		document := files.DirectoryDocument(root.scope.root, directory, visible)
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
