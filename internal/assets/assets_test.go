package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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

func TestPreviewStylesheetScopesBothPalettesToOneResource(t *testing.T) {
	t.Parallel()

	stylesheet := PreviewStylesheet()

	// Both palettes are present, scoped to the class the client toggles on <html>
	// so a theme switch never needs a new stylesheet request.
	for _, want := range []string{
		"html:not(.dark) .markdown-body",
		"html.dark .markdown-body",
		"color-scheme: light",
		"color-scheme: dark",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("preview stylesheet missing %q", want)
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
			t.Errorf("preview stylesheet missing shared layer %q", want)
		}
	}

	// The same stable bytes come back every call: there is no mode input.
	if PreviewStylesheet() != stylesheet {
		t.Error("PreviewStylesheet is not deterministic")
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
// them from convert output and Web preview at once.
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
		"rich-content.js",
		"fonts/KaTeX_AMS-Regular.woff2",
		"LICENSE.katex",
		"LICENSE.mermaid",
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

	stylesheet, err := RichAssetText("katex.min.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{`format("woff")`, `format("truetype")`} {
		if strings.Contains(stylesheet, unwanted) {
			t.Errorf("katex.min.css still references %q", unwanted)
		}
	}
}

func TestRichContentRuntimeAddsCodeCopyButtons(t *testing.T) {
	t.Parallel()

	runtime, err := fs.ReadFile(RichFS(), "rich-content.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"m2h-code-copy",
		"window.isSecureContext",
		`document.execCommand("copy")`,
	} {
		if !strings.Contains(string(runtime), want) {
			t.Errorf("rich-content runtime does not contain %q", want)
		}
	}
}

func TestWriteRichAssets(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	if err := WriteRichAssets(target); err != nil {
		t.Fatalf("WriteRichAssets returned error: %v", err)
	}

	for _, rel := range []string{
		"katex.min.css",
		"katex.min.js",
		"auto-render.min.js",
		"mermaid.min.js",
		"rich-content.js",
		"fonts/KaTeX_AMS-Regular.woff2",
	} {
		info, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("WriteRichAssets did not write %q: %v", rel, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("WriteRichAssets wrote empty %q", rel)
		}
	}

	// Re-running must refresh the directory without error (idempotent overwrite).
	if err := WriteRichAssets(target); err != nil {
		t.Fatalf("WriteRichAssets second run returned error: %v", err)
	}
}

func TestWriteRichAssetsReportsUnwritableTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if err := WriteRichAssets(filepath.Join(locked, RichAssetDir)); err == nil {
		t.Fatal("WriteRichAssets succeeded against an unwritable target")
	}
}

func TestInlineKatexCSSEmbedsFonts(t *testing.T) {
	t.Parallel()

	stylesheet, err := InlineKatexCSS()
	if err != nil {
		t.Fatalf("InlineKatexCSS() returned error: %v", err)
	}
	if !strings.Contains(stylesheet, "data:font/woff2;base64,") {
		t.Fatal("inline stylesheet does not embed any WOFF2 data URI")
	}
	for _, unwanted := range []string{
		"url(fonts/",
		`format("woff")`,
		`format("truetype")`,
	} {
		if strings.Contains(stylesheet, unwanted) {
			t.Errorf("inline stylesheet still references external fallback %q", unwanted)
		}
	}

	// Every vendored woff2 font must be reachable; a missing file is an error
	// rather than a silently dropped font.
	raw, err := fs.ReadFile(RichFS(), "katex.min.css")
	if err != nil {
		t.Fatal(err)
	}
	references := strings.Count(string(raw), ".woff2")
	if references == 0 {
		t.Fatal("vendored katex.min.css references no woff2 fonts")
	}
	if got := strings.Count(stylesheet, "data:font/woff2;base64,"); got != references {
		t.Errorf("embedded %d fonts, want %d", got, references)
	}

	cached, err := InlineKatexCSS()
	if err != nil {
		t.Fatal(err)
	}
	if cached != stylesheet {
		t.Error("InlineKatexCSS() returned a different stylesheet on the second call")
	}
}

func TestRichAssetText(t *testing.T) {
	t.Parallel()

	contents, err := RichAssetText("rich-content.js")
	if err != nil {
		t.Fatalf("RichAssetText() returned error: %v", err)
	}
	if !strings.Contains(contents, "m2h-code-copy") {
		t.Error("RichAssetText returned the wrong file contents")
	}
	if _, err := RichAssetText("missing.js"); err == nil {
		t.Error("RichAssetText() accepted a missing asset name")
	}
}
