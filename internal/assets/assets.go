// Package assets embeds the shared styles used by every HTML renderer.
package assets

import (
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

// RichAssetText returns the contents of one file directly under the embedded
// rich-content runtime root, such as "katex.min.js".
func RichAssetText(name string) (string, error) {
	contents, err := richFS.ReadFile("rich/" + name)
	if err != nil {
		return "", fmt.Errorf("read embedded asset %q: %w", name, err)
	}
	return string(contents), nil
}

var (
	// katexFontFallback matches the woff and truetype src entries every
	// @font-face declares after its woff2 url; inline documents keep only the
	// woff2 reference so fonts are not embedded three times over.
	katexFontFallback = regexp.MustCompile(
		`,url\(fonts/[^)]+\.woff\) format\("woff"\),url\(fonts/[^)]+\.ttf\) format\("truetype"\)`,
	)
	katexWoff2Reference = regexp.MustCompile(`url\(fonts/([^)]+\.woff2)\)`)
)

// inlineKatexCSS caches the stylesheet with every font embedded as a data URI;
// building it base64-encodes the full font set, which is far too expensive to
// repeat per rendered document.
var inlineKatexCSS = sync.OnceValues(func() (string, error) {
	raw, err := richFS.ReadFile("rich/katex.min.css")
	if err != nil {
		return "", fmt.Errorf("read embedded katex.min.css: %w", err)
	}
	stylesheet := katexFontFallback.ReplaceAllString(string(raw), "")
	for _, match := range katexWoff2Reference.FindAllStringSubmatch(stylesheet, -1) {
		fontName := match[1]
		font, err := richFS.ReadFile("rich/fonts/" + fontName)
		if err != nil {
			return "", fmt.Errorf("read embedded font %q: %w", fontName, err)
		}
		dataURI := "data:font/woff2;base64," + base64.StdEncoding.EncodeToString(font)
		stylesheet = strings.ReplaceAll(stylesheet, match[0], "url("+dataURI+")")
	}
	return stylesheet, nil
})

// InlineKatexCSS returns the vendored KaTeX stylesheet with its WOFF2 fonts
// inlined as data URIs, so a self-contained HTML document needs no external
// font files. The result is cached after the first call.
func InlineKatexCSS() (string, error) {
	return inlineKatexCSS()
}
