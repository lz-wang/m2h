package files

import (
	"context"
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
	return pickEntryDocument(children, descendants)
}

// pickEntryDocument applies the shared selection order: among the direct
// children, README wins over index over 00-index (case-insensitive stems),
// then the lexically first child; with no direct document the shallowest
// descendant wins and lexical path order breaks ties.
func pickEntryDocument(children, descendants []string) string {
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

// reachableMarkdown reports whether one directory entry is a publishable
// Markdown file under options: a regular file — resolving file symlinks
// within the root, refusing anything else — whose canonical target is not
// hidden when SkipHidden applies, and that passes depth and glob rules on
// its requested path. It mirrors Discover's per-file decisions so a document
// FindDirectoryDocument picks is one Discover would have listed.
func reachableMarkdown(root, current string, entry os.DirEntry, relative string, options DiscoverOptions) (bool, error) {
	target := current
	if entry.Type()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return false, nil
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return false, err
		}
		if !IsWithin(root, resolved) {
			return false, nil
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return false, nil
		}
		target = resolved
	} else {
		info, err := entry.Info()
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, nil
		}
	}
	if options.SkipHidden {
		resolvedRelative, err := filepath.Rel(root, target)
		if err != nil {
			return false, err
		}
		if IsHiddenPath(NormalizeRelativePath(resolvedRelative)) {
			return false, nil
		}
	}
	return IsMarkdown(relative) && Matches(relative, options), nil
}

// FindDirectoryDocument selects the entry document of one directory without
// scanning the whole workspace: a breadth-first walk covers only the target
// directory's own subtree, applying the same rules as Discover — depth,
// glob, hidden pruning, excludes, file-symlink resolution within the root,
// and directory symlinks never followed — so a document it picks is one the
// full discovery would have listed. The selection order matches
// DirectoryDocument exactly, with the BFS levels realizing the
// shallowest-descendant rule: the first level that yields any publishable
// Markdown contributes the answer, lexical path order breaking ties.
// A cancelled context aborts the walk.
func FindDirectoryDocument(ctx context.Context, root, directory string, options DiscoverOptions) string {
	if err := ValidateDiscoverOptions(options); err != nil {
		return ""
	}
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
	// Canonicalize the root like Discover does, so symlink resolution and the
	// within-root checks below compare against the same spelling of the root
	// (macOS /var vs /private/var, and any other existing symlink components).
	input, err := Resolve(root)
	if err != nil {
		return ""
	}
	root = input.Path
	if err := ctx.Err(); err != nil {
		return ""
	}
	excludeRoot, err := normalizeExcludeRoot(options.ExcludeRoot)
	if err != nil {
		return ""
	}
	excludes, err := normalizeExcludes(options.Excludes)
	if err != nil {
		return ""
	}

	start := directory
	if start == "." {
		start = ""
	}

	// documentsOf reads one directory level: the publishable Markdown files
	// directly inside it (root-relative paths) and the subdirectories a
	// deeper level may still find documents in.
	documentsOf := func(level string) (documents []string, directories []string, err error) {
		entries, readErr := os.ReadDir(filepath.Join(root, filepath.FromSlash(level)))
		if readErr != nil {
			return nil, nil, readErr
		}
		for _, entry := range entries {
			relative := entry.Name()
			if level != "" {
				relative = level + "/" + relative
			}
			current := filepath.Join(root, filepath.FromSlash(relative))
			if options.SkipHidden && IsHiddenPath(relative) {
				continue
			}
			if isExcluded(excludeRoot, excludes, current) {
				continue
			}
			if entry.IsDir() {
				// A directory at relative depth d can only hold files at
				// depth d+1 and beyond, so this prunes exactly like
				// Discover's directory depth rule.
				if FileDepth(relative) >= options.Depth {
					continue
				}
				directories = append(directories, relative)
				continue
			}
			publishable, err := reachableMarkdown(root, current, entry, relative, options)
			if err != nil {
				return nil, nil, err
			}
			if publishable {
				documents = append(documents, relative)
			}
		}
		return documents, directories, nil
	}

	documents, directories, err := documentsOf(start)
	if err != nil {
		return ""
	}
	if document := pickEntryDocument(documents, nil); document != "" {
		return document
	}
	// Breadth-first over subdirectories: the first level with any publishable
	// Markdown yields the shallowest descendant, lexical order breaks ties.
	for len(directories) > 0 {
		if err := ctx.Err(); err != nil {
			return ""
		}
		sort.Strings(directories)
		next := make([]string, 0)
		found := make([]string, 0)
		for _, level := range directories {
			documents, subdirectories, err := documentsOf(level)
			if err != nil {
				return ""
			}
			found = append(found, documents...)
			next = append(next, subdirectories...)
		}
		if len(found) > 0 {
			sort.Strings(found)
			return found[0]
		}
		directories = next
	}
	return ""
}
