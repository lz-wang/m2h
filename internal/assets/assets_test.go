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
