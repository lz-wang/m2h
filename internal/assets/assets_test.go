package assets

import (
	"io/fs"
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
				".m2h-code-copy",
				".m2h-code-frame",
				"padding-right: 3.25rem",
				"place-items: center",
				"@media (max-width: 767px)",
				"padding: 15px",
				".chroma",
				"--m2h-syntax-keyword",
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

	dark, err := Stylesheet("dark")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		".m2h-mode-dark .chroma",
		"--m2h-syntax-keyword: #ff7b72",
		"--m2h-syntax-string: #a5d6ff",
		"--m2h-syntax-comment: #9198a1",
		"--m2h-syntax-background: #151b23",
	} {
		if !strings.Contains(dark, want) {
			t.Errorf("dark stylesheet does not contain GitHub dark syntax rule %q", want)
		}
	}
}

func TestWebStylesheetScopesBothPalettesToOneResource(t *testing.T) {
	t.Parallel()

	stylesheet := WebStylesheet()

	// Both palettes are present, scoped to the class the client toggles on <html>
	// so a theme switch never needs a new stylesheet request.
	for _, want := range []string{
		"html:not(.dark) .markdown-body",
		"html.dark .markdown-body",
		"color-scheme: light",
		"color-scheme: dark",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("web stylesheet missing %q", want)
		}
	}

	// Shared, mode-independent layers are still included and remain class-driven
	// (layout.css keeps its own unscoped .markdown-body structural rules).
	for _, want := range []string{
		".m2h-mode-dark .chroma",
		"--m2h-syntax-keyword",
		"html.m2h-mode-dark",
		"max-width: 980px",
		".m2h-code-copy",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("web stylesheet missing shared layer %q", want)
		}
	}

	// The same stable bytes come back every call: there is no mode input.
	if WebStylesheet() != stylesheet {
		t.Error("WebStylesheet is not deterministic")
	}
}

func TestStylesheetRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	if _, err := Stylesheet("sepia"); err == nil {
		t.Fatal("Stylesheet() accepted an unknown mode")
	}
}

func TestStylesheetIncludesExtendedMarkup(t *testing.T) {
	t.Parallel()

	stylesheet, err := Stylesheet("light")
	if err != nil {
		t.Fatalf("Stylesheet(\"light\") returned error: %v", err)
	}
	for _, want := range []string{
		".markdown-body ins",
		".markdown-body .keys",
		".markdown-body mark",
		".markdown-body .m2h-critic-comment",
		".markdown-body .m2h-critic-delete",
		".markdown-body .m2h-critic-insert",
		".markdown-body .m2h-critic-block",
		".markdown-body .m2h-critic-block-highlight",
		".markdown-body .m2h-critic-block-insert",
		".markdown-body .m2h-critic-block-delete",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("stylesheet does not contain extended-markup rule %q", want)
		}
	}
}

// TestStylesheetIncludesKeycapStyles guards the Material-style keycap: the
// theme variables, the keycap plate and the canonical-class glyph rules must
// all stay in the shared stylesheet so a CSS cleanup cannot silently strip
// them from export output and the WebUI at once.
func TestStylesheetIncludesKeycapStyles(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"light", "dark", "auto"} {
		stylesheet, err := Stylesheet(mode)
		if err != nil {
			t.Fatalf("Stylesheet(%q) returned error: %v", mode, err)
		}
		for _, want := range []string{
			".markdown-body .keys > kbd",

			"--m2h-kbd-fg:",
			"--m2h-kbd-bg:",
			"--m2h-kbd-border:",
			"--m2h-kbd-highlight:",
			"--m2h-kbd-separator:",

			".key-control::before",
			".key-command::before",
			".key-shift::before",
			".key-alt::before",
			".key-option::before",
			".key-arrow-left::before",
			".key-page-up::before",
			".key-enter::after",
			".key-tab::after",
			".key-num-enter::after",
		} {
			if !strings.Contains(stylesheet, want) {
				t.Errorf("Stylesheet(%q) does not contain keycap rule %q", mode, want)
			}
		}
	}
}

func TestStylesheetExtendedMarkupIsThemeAware(t *testing.T) {
	t.Parallel()

	// layout.css is shared by every mode, so the Critic colors and keycap
	// colors must be CSS variables with explicit dark overrides; otherwise
	// red/green/amber washes are unreadable on the #0d1117 dark background
	// and the raised keycaps keep light-mode plates.
	stylesheet, err := Stylesheet("light")
	if err != nil {
		t.Fatalf("Stylesheet(\"light\") returned error: %v", err)
	}
	for _, want := range []string{
		"--m2h-critic-mark-bg:",
		"--m2h-critic-delete-bg:",
		"--m2h-critic-insert-bg:",
		"--m2h-kbd-fg:",
		"--m2h-kbd-bg:",
		"--m2h-kbd-border:",
		"--m2h-kbd-highlight:",
		"--m2h-kbd-separator:",
		"html.m2h-mode-dark",
		"rgb(187 128 9 / 40%)",
		"rgb(248 81 73 / 30%)",
		"rgb(63 185 80 / 30%)",
		"rgb(255 255 255 / 10%)",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("stylesheet does not contain dark-mode critic token %q", want)
		}
	}
}

// TestStylesheetSortableHeadersAreThemeAware guards the sortable-header
// palette: the arrow, hover wash, and focus ring are --m2h-sort-* variables
// with explicit dark overrides. github-markdown-css defines no
// --bgColor-muted/--fgColor-muted/--focus-outlineColor variables, so a bare
// var() with a light fallback would render light-only in dark mode.
func TestStylesheetSortableHeadersAreThemeAware(t *testing.T) {
	t.Parallel()

	dark, err := Stylesheet("dark")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"--m2h-sort-fg:",
		"--m2h-sort-hover-bg:",
		"--m2h-sort-focus:",
		"html.m2h-mode-dark",
		"var(--m2h-sort-fg)",
		"var(--m2h-sort-hover-bg)",
		"var(--m2h-sort-focus)",
		// The dark palette must be present, not just the light defaults.
		"--m2h-sort-hover-bg: #21262d",
		"--m2h-sort-fg: #8c959f",
		"--m2h-sort-focus: #4493f8",
	} {
		if !strings.Contains(dark, want) {
			t.Errorf("stylesheet does not contain sortable-header token %q", want)
		}
	}

	// auto must resolve the dark palette through the media query, not by
	// switching the base values.
	auto, err := Stylesheet("auto")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(auto, "html.m2h-mode-auto") ||
		!strings.Contains(auto, "--m2h-sort-hover-bg: #21262d") {
		t.Error("auto stylesheet missing the dark media-query overrides for sortable headers")
	}
}

// TestStylesheetSortableHeaderGeometryIsStatic guards the layout-stability
// contract: the sorting-indicator space (position and padding-right) must be
// reserved by a static rule on every plain GFM header cell, present from the
// first layout pass. The runtime rule keyed on Tablesort's role attribute may
// only carry interaction properties — if geometry ever moved back behind the
// async enhancement, table columns would reflow after load and shift a
// restored reading position.
func TestStylesheetSortableHeaderGeometryIsStatic(t *testing.T) {
	t.Parallel()

	stylesheet, err := Stylesheet("light")
	if err != nil {
		t.Fatalf("Stylesheet(\"light\") returned error: %v", err)
	}
	for _, want := range []string{
		".markdown-body table:not([class]) thead th {\n  position: relative;\n  padding-right: 2rem;\n}",
		".markdown-body th[role=\"columnheader\"]:not([data-sort-method=\"none\"]) {\n  cursor: pointer;\n  user-select: none;\n}",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("stylesheet does not contain sortable-header rule %q", want)
		}
	}
	// The runtime rule must not re-introduce geometry: any layout property on
	// the role-keyed block is the regression this test exists to catch.
	dynamicRule := strings.Index(stylesheet, "th[role=\"columnheader\"]:not([data-sort-method=\"none\"]) {")
	if dynamicRule == -1 {
		t.Fatal("stylesheet lost the role-keyed sortable-header rule")
	}
	block := stylesheet[dynamicRule : strings.Index(stylesheet[dynamicRule:], "}")+dynamicRule]
	for _, unwanted := range []string{"padding", "width", "position"} {
		if strings.Contains(block, unwanted) {
			t.Errorf("role-keyed sortable-header rule still carries geometry property %q:\n%s", unwanted, block)
		}
	}
}

// TestStylesheetCodeCopyButtonIsThemeAware guards the copy-control palette:
// github-markdown-css defines no --bgColor-default/--fgColor-muted family of
// variables, so the button must carry its own --m2h-copy-* tokens with an
// explicit dark palette instead of light fallbacks.
func TestStylesheetCodeCopyButtonIsThemeAware(t *testing.T) {
	t.Parallel()

	dark, err := Stylesheet("dark")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"var(--m2h-copy-fg)",
		"var(--m2h-copy-bg)",
		"var(--m2h-copy-border)",
		"var(--m2h-copy-accent)",
		"var(--m2h-copy-hover-bg)",
		"var(--m2h-copy-focus-glow)",
		// The dark palette must be present, not just the light defaults.
		"--m2h-copy-bg: #21262d",
		"--m2h-copy-border: #3d444d",
		"--m2h-copy-accent: #4493f8",
		"--m2h-copy-hover-bg: #30363d",
	} {
		if !strings.Contains(dark, want) {
			t.Errorf("stylesheet does not contain copy-control token %q", want)
		}
	}
	// No bare light-valued fallbacks may remain on the button rules.
	if strings.Contains(dark, "var(--bgColor-default") ||
		strings.Contains(dark, "var(--borderColor-default") {
		t.Error("copy control still references bare github-markdown-css variables")
	}

	auto, err := Stylesheet("auto")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(auto, "html.m2h-mode-auto") ||
		!strings.Contains(auto, "--m2h-copy-bg: #21262d") {
		t.Error("auto stylesheet missing the dark media-query overrides for the copy control")
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

func TestRichAssetsEmbedded(t *testing.T) {
	t.Parallel()

	rich := RichFS()
	for _, name := range []string{
		"katex.min.css",
		"katex.min.js",
		"auto-render.min.js",
		"mermaid.min.js",
		"mermaid-zenuml/mermaid-zenuml.esm.min.mjs",
		"mermaid-zenuml/chunks/mermaid-zenuml.esm.min/zenuml-definition-EPHX7WPJ.mjs",
		"mermaid-zenuml/chunks/mermaid-zenuml.esm.min/chunk-PPGA74DV.mjs",
		"tablesort.min.js",
		"tablesort.number.js",
		"tablesort.date.js",
		"tablesort.filesize.js",
		"tablesort.dotsep.js",
		"tablesort.monthname.js",
		"fonts/KaTeX_AMS-Regular.woff2",
		"LICENSE.katex",
		"LICENSE.mermaid",
		"LICENSE.mermaid-zenuml",
		"LICENSE.zenuml-core",
		"LICENSE.tablesort",
	} {
		info, err := fs.Stat(rich, name)
		if err != nil {
			t.Errorf("RichFS missing %q: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("embedded rich asset %q is empty", name)
		}
	}
}

// TestRichFontsAreWoff2Only guards the size optimization: m2h targets modern
// browsers, so the woff and truetype fallback formats must not creep back in
// with a future KaTeX upgrade.
func TestRichFontsAreWoff2Only(t *testing.T) {
	t.Parallel()

	offenders := []string{}
	err := fs.WalkDir(RichFS(), "fonts", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".woff2") {
			offenders = append(offenders, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("non-woff2 font files embedded: %v", offenders)
	}

	stylesheet, err := fs.ReadFile(RichFS(), "katex.min.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{`format("woff")`, `format("truetype")`} {
		if strings.Contains(string(stylesheet), unwanted) {
			t.Errorf("katex.min.css still references %q", unwanted)
		}
	}
}
