package markdown

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// githubIDs implements parser.IDs with GitHub-compatible anchor rules that keep
// Unicode (CJK) characters, unlike Goldmark's default generator which drops
// every multibyte rune. Each instance belongs to a single document so duplicate
// headings get -1, -2, ... suffixes.
type githubIDs struct {
	used map[string]int
}

func newGitHubIDs() parser.IDs {
	return &githubIDs{used: make(map[string]int)}
}

// Generate returns a GitHub-compatible anchor id for value and records it so the
// next duplicate slug receives an incrementing suffix.
func (g *githubIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := githubSlug(string(value))
	if base == "" {
		base = "section"
	}
	if count, exists := g.used[base]; exists {
		count++
		g.used[base] = count
		return []byte(fmt.Sprintf("%s-%d", base, count))
	}
	g.used[base] = 0
	return []byte(base)
}

// Put registers an id as already in use, e.g. a hand-authored heading id, so
// later auto-generated ids avoid colliding with it.
func (g *githubIDs) Put(value []byte) {
	key := string(value)
	if _, exists := g.used[key]; !exists {
		g.used[key] = 0
	}
}

// githubSlug lowercases value, drops punctuation and symbols while keeping
// letters (including CJK), digits, underscores and hyphens, then joins the
// remaining words on hyphens. Examples:
//
//	"7. 代码"   -> "7-代码"
//	"C++ API"  -> "c-api"
//	"Foo *Bar*" -> "foo-bar"
func githubSlug(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsLetter(r), unicode.IsNumber(r):
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			// drop punctuation, symbols and other separators
		}
	}
	return strings.Join(strings.Fields(b.String()), "-")
}
