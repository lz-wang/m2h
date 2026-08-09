// Package assets embeds the shared styles used by every HTML renderer.
package assets

import (
	"fmt"
	"strings"

	_ "embed"
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
