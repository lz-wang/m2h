// Package assets embeds the shared styles used by every HTML renderer.
package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GitHubMarkdownCSSVersion is the vendored github-markdown-css release.
const GitHubMarkdownCSSVersion = "5.9.0"

// RichAssetDir is the on-disk directory name convert writes rich-content
// runtime assets into, next to the generated HTML.
const RichAssetDir = ".m2h"

var (
	//go:embed github-markdown.css
	githubMarkdownAuto string

	//go:embed github-markdown-light.css
	githubMarkdownLight string

	//go:embed github-markdown-dark.css
	githubMarkdownDark string

	//go:embed syntax.css
	syntaxCSS string

	//go:embed layout.css
	layoutCSS string

	//go:embed LICENSE.github-markdown-css
	githubMarkdownCSSLicense string

	//go:embed all:rich
	richFS embed.FS
)

// Stylesheet returns the fixed GitHub Markdown, syntax, and layout styles.
func Stylesheet(mode string) (string, error) {
	var markdownCSS string
	switch mode {
	case "light":
		markdownCSS = githubMarkdownLight
	case "dark":
		markdownCSS = githubMarkdownDark
	case "auto":
		markdownCSS = githubMarkdownAuto
	default:
		return "", fmt.Errorf("unsupported stylesheet mode %q", mode)
	}

	return strings.Join([]string{markdownCSS, syntaxCSS, layoutCSS}, "\n"), nil
}

// GitHubMarkdownCSSLicense returns the embedded upstream MIT license.
func GitHubMarkdownCSSLicense() string {
	return githubMarkdownCSSLicense
}

// RichFS returns the embedded KaTeX and Mermaid runtime rooted at the asset
// directory, suitable for http.FileServer and on-disk writes.
func RichFS() fs.FS {
	sub, err := fs.Sub(richFS, "rich")
	if err != nil {
		// The "rich" directory is embedded at build time, so fs.Sub cannot fail.
		panic(fmt.Errorf("assets: rich subtree missing: %w", err))
	}
	return sub
}

// WriteRichAssets writes every embedded rich-content asset into target so that
// katex.min.css, its fonts/, mermaid.min.js and rich-content.js appear directly
// under target. It is safe to call repeatedly to refresh a stale directory.
func WriteRichAssets(target string) error {
	return fs.WalkDir(richFS, "rich", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(path, "rich/")
		destination := filepath.Join(target, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("create asset directory %q: %w", filepath.Dir(destination), err)
		}
		contents, err := richFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded asset %q: %w", path, err)
		}
		if err := os.WriteFile(destination, contents, 0o644); err != nil {
			return fmt.Errorf("write asset %q: %w", destination, err)
		}
		return nil
	})
}
