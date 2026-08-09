package assets

import (
	"strings"
	"testing"
)

func TestStylesheetContractSnapshot(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"light", "dark", "auto"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			stylesheet, err := Stylesheet(mode)
			if err != nil {
				t.Fatalf("Stylesheet(%q) returned error: %v", mode, err)
			}
			for _, want := range []string{
				".markdown-body",
				"max-width: 980px",
				"padding: 45px",
				"@media (max-width: 767px)",
				"padding: 15px",
				".chroma",
				"--color-prettylights-syntax-keyword",
			} {
				if !strings.Contains(stylesheet, want) {
					t.Errorf("Stylesheet(%q) does not contain %q", mode, want)
				}
			}
		})
	}

	auto, err := Stylesheet("auto")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(auto, "@media (prefers-color-scheme: dark)") ||
		!strings.Contains(auto, "@media (prefers-color-scheme: light)") {
		t.Fatal("auto stylesheet does not include light and dark media rules")
	}
}

func TestStylesheetRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	if _, err := Stylesheet("sepia"); err == nil {
		t.Fatal("Stylesheet() accepted an unknown mode")
	}
}

func TestVendoredMetadata(t *testing.T) {
	t.Parallel()

	if GitHubMarkdownCSSVersion != "5.9.0" {
		t.Fatalf("GitHubMarkdownCSSVersion = %q", GitHubMarkdownCSSVersion)
	}
	license := GitHubMarkdownCSSLicense()
	if !strings.Contains(license, "MIT License") || !strings.Contains(license, "Sindre Sorhus") {
		t.Fatal("vendored github-markdown-css license is incomplete")
	}
}
