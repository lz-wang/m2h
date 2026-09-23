// Package files resolves input roots and discovers safe files beneath them.
package files

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Kind identifies whether a resolved input is one file or one directory.
type Kind uint8

const (
	KindFile Kind = iota + 1
	KindDirectory
)

// Input is an absolute, symlink-resolved input path.
type Input struct {
	Path string
	Kind Kind
}

// Entry is a safe regular file discovered within an input root.
type Entry struct {
	AbsolutePath string
	RelativePath string
	Mode         fs.FileMode
}

// DiscoverOptions configures deterministic depth and glob filtering.
type DiscoverOptions struct {
	Depth       int
	Pattern     string
	ExcludeRoot string
	Excludes    []string
	// SkipHidden keeps every dot-prefixed path component out of the results.
	// Both published scopes (serve, check) turn it on by default so
	// implicitly published directory content never exposes dotfiles; the
	// caller's Hidden option decides the value. Protected paths are pruned
	// regardless of this flag.
	SkipHidden bool
	Log        io.Writer
	// Ignore applies the root's publishing rules (.gitignore today) on top
	// of the structural filters. A nil matcher disables the check entirely,
	// so single-file inputs and opt-out callers keep their exact behavior.
	// Like every rule here it filters discovery only — it never widens it.
	Ignore IgnoreMatcher
}

// Discovery separates Markdown inputs from other assets.
type Discovery struct {
	Markdown []Entry
	Assets   []Entry
}

// Resolve normalizes an input and permits a root file or directory symlink.
// The resolved path must not itself cross a protected component: when a
// protected directory becomes the input root, its contents are judged by
// root-relative paths that no longer carry the protected name, so refusing
// the root here is the only thing that keeps the permanent protection true
// for explicitly named inputs too — whatever their kind.
func Resolve(input string) (Input, error) {
	if strings.TrimSpace(input) == "" {
		return Input{}, fmt.Errorf("resolve input: path is required")
	}
	resolved, err := CanonicalPath(input)
	if err != nil {
		return Input{}, fmt.Errorf("resolve input %q: %w", input, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Input{}, fmt.Errorf("inspect input %q: %w", resolved, err)
	}
	if IsProtectedPath(resolved) {
		return Input{}, fmt.Errorf("resolve input %q: %q is a protected path and cannot be published", input, resolved)
	}
	if info.IsDir() {
		return Input{Path: resolved, Kind: KindDirectory}, nil
	}
	if info.Mode().IsRegular() {
		return Input{Path: resolved, Kind: KindFile}, nil
	}
	return Input{}, fmt.Errorf("inspect input %q: expected a regular file or directory", resolved)
}

// CanonicalPath resolves every existing symlink component while preserving a
// possibly nonexistent final suffix. This also normalizes macOS /var aliases.
func CanonicalPath(value string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", err
	}

	candidate := absolute
	suffix := []string{}
	for {
		resolved, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr == nil {
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(resolveErr) {
			return "", resolveErr
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", resolveErr
		}
		suffix = append(suffix, filepath.Base(candidate))
		candidate = parent
	}
}

// Discover walks one directory without following internal symlink directories.
func Discover(ctx context.Context, root string, options DiscoverOptions) (Discovery, error) {
	if err := ValidateDiscoverOptions(options); err != nil {
		return Discovery{}, err
	}
	if err := ctx.Err(); err != nil {
		return Discovery{}, fmt.Errorf("discover files: %w", err)
	}

	input, err := Resolve(root)
	if err != nil {
		return Discovery{}, err
	}
	if input.Kind != KindDirectory {
		return Discovery{}, fmt.Errorf("discover files in %q: expected a directory", input.Path)
	}

	excludeRoot, err := normalizeExcludeRoot(options.ExcludeRoot)
	if err != nil {
		return Discovery{}, err
	}
	excludes, err := normalizeExcludes(options.Excludes)
	if err != nil {
		return Discovery{}, err
	}
	logger := options.Log
	if logger == nil {
		logger = io.Discard
	}

	result := Discovery{}
	err = filepath.WalkDir(input.Path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", current, walkErr)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("walk %q: %w", current, err)
		}
		if current == input.Path {
			return nil
		}
		if isExcluded(excludeRoot, excludes, current) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relative, err := filepath.Rel(input.Path, current)
		if err != nil {
			return fmt.Errorf("make %q relative to %q: %w", current, input.Path, err)
		}
		relative = NormalizeRelativePath(relative)
		// Protected paths are pruned before every other rule: .git/, .ssh/
		// and .env files stay unpublished whatever SkipHidden or the ignore
		// rules say, and SkipDir keeps the walk out of their subtrees
		// entirely.
		if IsProtectedPath(relative) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Hidden components are pruned during the walk, not filtered after it:
		// SkipDir keeps the walker out of the subtree entirely, so a large
		// hidden directory costs nothing.
		if options.SkipHidden && IsHiddenPath(relative) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// The ignore rules run after the structural exclusions and before
		// depth, glob and the safe-file checks: an ignored directory prunes
		// its subtree exactly like a hidden one, and every failure inside
		// the rule files surfaces as a discovery error instead of a silent
		// widening of the visible set.
		if options.Ignore != nil {
			ignored, err := options.Ignore.Ignored(relative, entry.IsDir())
			if err != nil {
				return fmt.Errorf("walk %q: %w", current, err)
			}
			if ignored {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if entry.IsDir() {
			if directoryDepth(relative) > options.Depth {
				return filepath.SkipDir
			}
			return nil
		}
		if FileDepth(relative) > options.Depth {
			return nil
		}

		safe, reachable, err := safeFileInfo(input.Path, current, entry, logger, relative)
		if err != nil {
			return err
		}
		if !reachable {
			return nil
		}
		// The publishing rules run on the canonical target as well: a visible
		// alias (public.md → .secret.md) would otherwise publish — and even
		// title-leak — a file the walk never surfaced. Only the security
		// properties are re-checked; glob/depth keep judging the alias path.
		resolvedRelative, relErr := filepath.Rel(input.Path, safe.target)
		if relErr != nil {
			return fmt.Errorf("make %q relative to %q: %w", safe.target, input.Path, relErr)
		}
		resolvedRelative = NormalizeRelativePath(resolvedRelative)
		if options.SkipHidden && IsHiddenPath(resolvedRelative) {
			return nil
		}
		if IsProtectedPath(resolvedRelative) {
			return nil
		}
		// The same alias argument applies to the ignore rules: an entry whose
		// canonical target is ignored stays unpublished even when it is
		// reached through a visible symlink.
		if options.Ignore != nil {
			ignored, err := options.Ignore.Ignored(resolvedRelative, false)
			if err != nil {
				return fmt.Errorf("walk %q: %w", current, err)
			}
			if ignored {
				return nil
			}
		}

		discovered := Entry{AbsolutePath: current, RelativePath: relative, Mode: safe.info.Mode()}
		if IsMarkdown(relative) {
			if Matches(relative, options) {
				result.Markdown = append(result.Markdown, discovered)
			}
		} else {
			result.Assets = append(result.Assets, discovered)
		}
		return nil
	})
	if err != nil {
		return Discovery{}, fmt.Errorf("discover files in %q: %w", input.Path, err)
	}

	sortEntries(result.Markdown)
	sortEntries(result.Assets)
	return result, nil
}

// Matches reports whether a normalized file path passes depth and glob rules.
// Call ValidateDiscoverOptions before using this helper with external input.
func Matches(relative string, options DiscoverOptions) bool {
	relative = NormalizeRelativePath(relative)
	if relative == "." || FileDepth(relative) > options.Depth {
		return false
	}
	if options.SkipHidden && IsHiddenPath(relative) {
		return false
	}
	return options.Pattern == "" || doublestar.MatchUnvalidated(options.Pattern, relative)
}

// ValidateDiscoverOptions validates enumeration flags before filesystem access.
func ValidateDiscoverOptions(options DiscoverOptions) error {
	if options.Depth < 0 {
		return fmt.Errorf("invalid depth %d: must be zero or greater", options.Depth)
	}
	if options.Pattern != "" && !doublestar.ValidatePattern(options.Pattern) {
		return fmt.Errorf("invalid glob %q", options.Pattern)
	}
	return nil
}

// NormalizeRelativePath converts both common separators into a clean slash path.
func NormalizeRelativePath(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Clean(value)
	return strings.TrimPrefix(value, "./")
}

// IsMarkdown reports whether a path has a supported Markdown extension.
func IsMarkdown(value string) bool {
	extension := filepath.Ext(value)
	return strings.EqualFold(extension, ".md") || strings.EqualFold(extension, ".markdown")
}

// IsHiddenPath reports whether a normalized root-relative path crosses a
// dot-prefixed path component (.git/config, .env, foo/.private/file.pdf).
// It is a structural rule about publishing boundaries, not a filename
// blacklist: only the path's own components decide.
func IsHiddenPath(relative string) bool {
	relative = NormalizeRelativePath(relative)
	for segment := range strings.SplitSeq(relative, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

// IsProtectedPath reports whether a normalized root-relative path crosses a
// component the publishing policy never serves: .git, .ssh, .env and its
// derived spellings (.env.local, .env.production, ...). Hidden admission is
// configurable (--hidden), this protection is not — publishing an option
// must not turn version-control internals, credential stores or environment
// files into web content, so every caller applies the rule before and after
// symlink resolution. Like IsHiddenPath it is a structural rule judged on
// the path's own components.
func IsProtectedPath(relative string) bool {
	relative = NormalizeRelativePath(relative)
	for segment := range strings.SplitSeq(relative, "/") {
		switch {
		case segment == ".git" || segment == ".ssh":
			return true
		case segment == ".env" || strings.HasPrefix(segment, ".env."):
			return true
		}
	}
	return false
}

// IsWithin reports whether candidate is root itself or lies beneath root.
func IsWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)))
}

func normalizeExcludeRoot(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	absolute, err := CanonicalPath(value)
	if err != nil {
		return "", fmt.Errorf("resolve excluded output %q: %w", value, err)
	}
	return absolute, nil
}

func normalizeExcludes(values []string) ([]string, error) {
	excludes := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		absolute, err := CanonicalPath(value)
		if err != nil {
			return nil, fmt.Errorf("resolve excluded path %q: %w", value, err)
		}
		excludes = append(excludes, absolute)
	}
	return excludes, nil
}

func isExcluded(excludeRoot string, excludes []string, current string) bool {
	if excludeRoot != "" && IsWithin(excludeRoot, current) {
		return true
	}
	for _, exclude := range excludes {
		if IsWithin(exclude, current) {
			return true
		}
	}
	return false
}

func directoryDepth(relative string) int {
	if relative == "." || relative == "" {
		return 0
	}
	return strings.Count(relative, "/") + 1
}

// FileDepth reports how many directories a normalized relative file path is
// nested beneath its root (a root-level file has depth 0). It backs the
// depth filter that both discovery and scope admission apply.
func FileDepth(relative string) int {
	return strings.Count(relative, "/")
}

// safeFile is one safely reachable regular file together with the absolute
// canonical path it resolves to. For a plain file the target is the path
// itself; for a file symlink it is the fully resolved destination.
type safeFile struct {
	info   fs.FileInfo
	target string
}

func safeFileInfo(root, current string, entry fs.DirEntry, logger io.Writer, relative string) (safeFile, bool, error) {
	if entry.Type()&os.ModeSymlink == 0 {
		info, err := entry.Info()
		if err != nil {
			return safeFile{}, false, fmt.Errorf("inspect %q: %w", current, err)
		}
		if !info.Mode().IsRegular() {
			return safeFile{}, false, nil
		}
		return safeFile{info: info, target: current}, true, nil
	}

	target, err := filepath.EvalSymlinks(current)
	if err != nil {
		_, _ = fmt.Fprintf(logger, "m2h: skip symlink %s: %v\n", relative, err)
		return safeFile{}, false, nil
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return safeFile{}, false, fmt.Errorf("resolve symlink target %q: %w", current, err)
	}
	if !IsWithin(root, target) {
		_, _ = fmt.Fprintf(logger, "m2h: skip symlink %s: target escapes root\n", relative)
		return safeFile{}, false, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		_, _ = fmt.Fprintf(logger, "m2h: skip symlink %s: %v\n", relative, err)
		return safeFile{}, false, nil
	}
	if info.IsDir() {
		return safeFile{}, false, nil
	}
	if !info.Mode().IsRegular() {
		return safeFile{}, false, nil
	}
	return safeFile{info: info, target: target}, true, nil
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].RelativePath < entries[right].RelativePath
	})
}
