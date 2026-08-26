// Package assets embeds the static resources compiled into the m2h binary.
package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

// GitHubMarkdownCSSVersion is the vendored github-markdown-css release.
const GitHubMarkdownCSSVersion = "5.9.0"

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

// WebStylesheet returns a single, mode-independent stylesheet for the WebUI.
// Both GitHub Markdown palettes are embedded at once, each scoped to
// the resolved theme class the WebUI already toggles on <html>: light rules
// apply under html:not(.dark), dark rules under html.dark. Because the URL never
// changes, switching theme swaps colors via the CSSOM only (no new request, no
// full-body stylesheet reload) — exported HTML keeps using the mode-specific
// Stylesheet above.
func WebStylesheet() string {
	light := scopeMarkdownCSS(githubMarkdownLight, "html:not(.dark)")
	dark := scopeMarkdownCSS(githubMarkdownDark, "html.dark")
	return strings.Join([]string{light, dark, syntaxCSS, layoutCSS}, "\n")
}

// scopeMarkdownCSS rewrites every ".markdown-body" selector in a GitHub
// Markdown stylesheet to live under scope, so two palettes can coexist in one
// stylesheet and be selected by an ancestor class. The vendored CSS only ever
// uses ".markdown-body" as a selector prefix (never inside a value or url()),
// so a plain replacement is safe.
func scopeMarkdownCSS(css, scope string) string {
	return strings.ReplaceAll(css, ".markdown-body", scope+" .markdown-body")
}

// GitHubMarkdownCSSLicense returns the embedded upstream MIT license.
func GitHubMarkdownCSSLicense() string {
	return githubMarkdownCSSLicense
}

// RichFS returns the embedded KaTeX and Mermaid runtime rooted at the asset
// directory, served by the WebUI under /runtime/*.
func RichFS() fs.FS {
	sub, err := fs.Sub(richFS, "rich")
	if err != nil {
		// The "rich" directory is embedded at build time, so fs.Sub cannot fail.
		panic(fmt.Errorf("assets: rich subtree missing: %w", err))
	}
	return sub
}
