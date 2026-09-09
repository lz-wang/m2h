package files

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// DirectoryDocument selects a visible Markdown entry inside a real
// directory. Callers supply their discovered scope, preserving depth/glob and
// root isolation. Directory symlinks are never followed.
func DirectoryDocument(root, directory string, visible []string) string {
	if IsHiddenPath(directory) && directory != "." {
		return ""
	}
	if directory != "." {
		if err := RequireExactPath(root, directory); err != nil {
			return ""
		}
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(directory)))
	if err != nil || !info.IsDir() {
		return ""
	}
	children := make([]string, 0)
	descendants := make([]string, 0)
	for _, candidate := range visible {
		if IsMarkdown(candidate) && !IsHiddenPath(candidate) && (directory == "." || strings.HasPrefix(candidate, directory+"/")) {
			descendants = append(descendants, candidate)
		}
		if path.Dir(candidate) == directory && IsMarkdown(candidate) && !IsHiddenPath(candidate) {
			children = append(children, candidate)
		}
	}
	sort.Strings(children)
	for _, stem := range []string{"readme", "index", "00-index"} {
		for _, candidate := range children {
			base := path.Base(candidate)
			if strings.EqualFold(strings.TrimSuffix(base, path.Ext(base)), stem) {
				return candidate
			}
		}
	}
	if len(children) > 0 {
		return children[0]
	}
	// With no direct document, choose the shallowest descendant, then lexical
	// path order. Discovery already enforced the original depth/glob limits.
	sort.Slice(descendants, func(i, j int) bool {
		left, right := strings.Count(descendants[i], "/"), strings.Count(descendants[j], "/")
		if left != right {
			return left < right
		}
		return descendants[i] < descendants[j]
	})
	if len(descendants) > 0 {
		return descendants[0]
	}
	return ""
}
