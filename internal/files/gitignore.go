package files

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	gitignore "github.com/denormal/go-gitignore"
)

// IgnoreMatcher answers whether one root-relative path is ignored by the
// publishing rules of its input root. relative always uses "/" separators,
// so matching behaves identically across platforms; isDir distinguishes a
// directory from a same-named file, because a "build/" rule ignores the
// directory while a file named build stays publishable.
//
// The interface is deliberately free of Git vocabulary so future rule
// sources (a dedicated .m2hignore, for example) can share one contract.
// Implementations observe one snapshot of the rule files: they are built
// per discovery or per request, and never watch the filesystem, keeping
// m2h's read-on-demand design intact.
type IgnoreMatcher interface {
	// Ignored reports whether the normalized root-relative path is ignored.
	Ignored(relative string, isDir bool) (bool, error)
}

// GitIgnore is a snapshot of the .gitignore rules inside one directory root.
// It lazily reads every .gitignore between the root and the queried path —
// root/.gitignore, notes/.gitignore for the query notes/private.md — so a
// decision touches only the files on that path, never the whole tree.
//
// Matching follows Git semantics: within one file the last matching rule
// wins, a deeper .gitignore overrides its ancestors, a rule without a
// leading slash matches at every level, and an excluded directory excludes
// everything inside it — no negation inside an excluded directory can
// re-include a file until the directory itself is re-included.
type GitIgnore struct {
	root string
	// levels caches parsed .gitignore files by their slash-relative
	// directory; a nil entry records "the file does not exist" so absent
	// rules cost one stat per directory per snapshot.
	levels map[string]gitignore.GitIgnore
}

// NewGitIgnore builds the rule snapshot for one directory root. A root
// without any .gitignore yields a valid matcher that ignores nothing, so
// callers never special-case repositories without rules.
func NewGitIgnore(root string) (*GitIgnore, error) {
	absolute, err := CanonicalPath(root)
	if err != nil {
		return nil, fmt.Errorf("load .gitignore rules for %q: %w", root, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("load .gitignore rules for %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("load .gitignore rules for %q: expected a directory", absolute)
	}
	return &GitIgnore{root: absolute, levels: make(map[string]gitignore.GitIgnore)}, nil
}

// Ignored implements IgnoreMatcher. The decision walks every path component:
// a directory excluded by an ancestor rule decides immediately, so a
// negation deeper inside can never re-include what the walk never reaches —
// exactly Git's "it is not possible to re-include a file if a parent
// directory of that file is excluded" rule.
func (g *GitIgnore) Ignored(relative string, isDir bool) (bool, error) {
	relative = NormalizeRelativePath(relative)
	if relative == "" || relative == "." {
		return false, nil
	}
	// The rule boundary is the input root: rules outside it are never read
	// (not even an ancestor's .gitignore), so a query that escapes the root
	// is a caller contract violation worth reporting, not silently passing.
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return false, fmt.Errorf("gitignore: path %q is not inside the root", relative)
	}

	parts := strings.Split(relative, "/")
	for end := 1; end <= len(parts); end++ {
		prefix := strings.Join(parts[:end], "/")
		final := end == len(parts)
		// Every ancestor component is a directory; only the final component
		// answers with the caller's isDir.
		decided, err := g.level(prefix, !final || isDir)
		if err != nil {
			return false, err
		}
		if final {
			return decided, nil
		}
		if decided {
			return true, nil
		}
	}
	return false, nil
}

// level decides one path prefix against the .gitignore files that may
// govern it — those in its own directory and every ancestor up to the root.
// Deeper files are consulted first: their rules override the ancestors',
// and within one file the matcher already applies last-rule-wins.
func (g *GitIgnore) level(prefix string, isDir bool) (bool, error) {
	base := path.Dir(prefix)
	for {
		matcher, err := g.fileAt(base)
		if err != nil {
			return false, err
		}
		if matcher != nil {
			relative := prefix
			if base != "." {
				relative = strings.TrimPrefix(prefix, base+"/")
			}
			if match := matcher.Relative(relative, isDir); match != nil {
				return match.Ignore(), nil
			}
		}
		if base == "." {
			return false, nil
		}
		base = path.Dir(base)
	}
}

// fileAt parses root/<dir>/.gitignore, caching the result for the snapshot's
// lifetime. A missing file is normal (nil matcher); unreadable or unparsable
// rules surface as errors so a discovery failure is never mistaken for an
// empty rule set.
func (g *GitIgnore) fileAt(dir string) (gitignore.GitIgnore, error) {
	if cached, ok := g.levels[dir]; ok {
		return cached, nil
	}
	name := ".gitignore"
	if dir != "." {
		name = path.Join(dir, ".gitignore")
	}
	data, err := os.ReadFile(filepath.Join(g.root, filepath.FromSlash(name)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			g.levels[dir] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	var failure error
	matcher := gitignore.New(bytes.NewReader(normalizeRuleLines(data)), g.root, func(item gitignore.Error) bool {
		if failure == nil {
			failure = item
		}
		return true
	})
	if failure != nil {
		return nil, fmt.Errorf("parse %s: %w", name, failure)
	}
	g.levels[dir] = matcher
	return matcher, nil
}

// normalizeRuleLines converts CRLF and CR line endings to plain LF. Git
// tolerates Windows-authored rule files; the parser underneath does not
// strip a trailing carriage return itself, and a "draft/\r" rule would
// silently match nothing.
func normalizeRuleLines(data []byte) []byte {
	if !bytes.ContainsRune(data, '\r') {
		return data
	}
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
}
