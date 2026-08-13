package markdown

import (
	"errors"
	"strings"
	"testing"
)

// assetSnippet returns the opening bytes of one embedded runtime script so
// tests can fingerprint exactly which bundles a rendered page carries.
func assetSnippet(t *testing.T, name string, length int) string {
	t.Helper()
	contents, err := inlineScript(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) < length {
		length = len(contents)
	}
	return contents[:length]
}

func TestRenderInlineAssetsEmbedOnlyWhatTheDocumentUses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		source          string
		wantScripts     []string
		wantStyles      []string
		unwantedScripts []string
		unwantedStrings []string
	}{
		{
			name:            "math document embeds KaTeX only",
			source:          "# Title\n\n$E=mc^2$ and \\(a+b\\)\n",
			wantScripts:     []string{"katex.min.js", "auto-render.min.js", "rich-content.js"},
			wantStyles:      []string{"data:font/woff2;base64,"},
			unwantedScripts: []string{"mermaid.min.js"},
		},
		{
			name:            "diagram document embeds Mermaid only",
			source:          "# Title\n\n```mermaid\nflowchart LR\n    A-->B\n```\n",
			wantScripts:     []string{"mermaid.min.js", "rich-content.js"},
			wantStyles:      nil,
			unwantedScripts: []string{"katex.min.js"},
			unwantedStrings: []string{"data:font/woff2"},
		},
		{
			name:            "plain document embeds only the enhancer",
			source:          "# Title\n\n```go\nfmt.Println(1)\n```\n",
			wantScripts:     []string{"rich-content.js"},
			wantStyles:      nil,
			unwantedScripts: []string{"katex.min.js", "mermaid.min.js"},
			unwantedStrings: []string{"data:font/woff2"},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := Render([]byte(test.source), RenderOptions{
				Mode:       ModeAuto,
				Target:     TargetConvert,
				SourcePath: "guide.md",
				Assets:     AssetInline,
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range test.wantScripts {
				if !strings.Contains(result.HTML, assetSnippet(t, name, 60)) {
					t.Errorf("inline HTML missing runtime %q", name)
				}
			}
			for _, want := range test.wantStyles {
				if !strings.Contains(result.HTML, want) {
					t.Errorf("inline HTML missing %q", want)
				}
			}
			for _, name := range test.unwantedScripts {
				if strings.Contains(result.HTML, assetSnippet(t, name, 60)) {
					t.Errorf("inline HTML unexpectedly embeds runtime %q", name)
				}
			}
			unwantedStrings := append([]string{"<script src=", "url(fonts/", ".m2h/"}, test.unwantedStrings...)
			for _, unwanted := range unwantedStrings {
				if strings.Contains(result.HTML, unwanted) {
					t.Errorf("inline HTML unexpectedly contains %q", unwanted)
				}
			}
		})
	}
}

func TestRenderInlineAssetsRejectExternalAssetBase(t *testing.T) {
	t.Parallel()

	_, err := Render([]byte("# Title"), RenderOptions{
		Mode:       ModeAuto,
		Target:     TargetConvert,
		SourcePath: "guide.md",
		Assets:     AssetInline,
		AssetBase:  ".m2h/",
	})
	var optionErr *OptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("Render() error = %v, want OptionError", err)
	}
	if optionErr.Name != "asset base" {
		t.Errorf("OptionError name = %q, want %q", optionErr.Name, "asset base")
	}
}

func TestRenderRejectsUnknownAssetMode(t *testing.T) {
	t.Parallel()

	_, err := Render([]byte("# Title"), RenderOptions{
		Mode:       ModeAuto,
		Target:     TargetConvert,
		SourcePath: "guide.md",
		Assets:     AssetMode(9),
	})
	var optionErr *OptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("Render() error = %v, want OptionError", err)
	}
	if optionErr.Name != "assets" {
		t.Errorf("OptionError name = %q, want %q", optionErr.Name, "assets")
	}
}

func TestRenderInlineImageResolverConsultedForLocalImages(t *testing.T) {
	t.Parallel()

	resolve := func(relative string) (string, bool) {
		if relative == "img/ok.png" {
			return "data:image/png;base64,Zm9rZW4=", true
		}
		return "", false
	}

	result, err := Render([]byte("# Title\n\n![Ok](img/ok.png)\n\n![Keep](img/missing.png)\n"), RenderOptions{
		Mode:        ModeAuto,
		Target:      TargetConvert,
		SourcePath:  "guide.md",
		Assets:      AssetInline,
		InlineImage: resolve,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Body, `src="data:image/png;base64,Zm9rZW4="`) {
		t.Errorf("inlinable image not replaced with data URI: %s", result.Body)
	}
	if !strings.Contains(result.Body, `src="img/missing.png"`) {
		t.Errorf("uninlinable image did not keep its relative path: %s", result.Body)
	}
}

func TestRenderInlineImageSkipsEscapingAndRemotePaths(t *testing.T) {
	t.Parallel()

	var seen []string
	resolve := func(relative string) (string, bool) {
		seen = append(seen, relative)
		return "", false
	}

	result, err := Render([]byte("# T\n\n![A](../outside.png)\n\n![B](https://example.com/b.png)\n\n![C](/absolute.png)\n"), RenderOptions{
		Mode:        ModeAuto,
		Target:      TargetConvert,
		SourcePath:  "guide.md",
		Assets:      AssetInline,
		InlineImage: resolve,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Errorf("resolver consulted for non-root-relative images: %v", seen)
	}
	for _, keep := range []string{"../outside.png", "https://example.com/b.png", "/absolute.png"} {
		if !strings.Contains(result.Body, keep) {
			t.Errorf("output missing preserved reference %q: %s", keep, result.Body)
		}
	}
}

func TestRenderInlineAssetsEscapeClosingTags(t *testing.T) {
	t.Parallel()

	// The real vendored bundles contain no closing tags; verify the escaping
	// path directly so a future upstream update cannot break the page.
	if escaped := escapeClosingTag(`x</style>y`, "style"); escaped != `x<\/style>y` {
		t.Errorf("escapeClosingTag = %q", escaped)
	}
	if escaped := escapeScriptTag.ReplaceAllString(`a</script>b`, `<\/script`); escaped != `a<\/script>b` {
		t.Errorf("script escaping = %q", escaped)
	}
	if escaped := escapeClosingTag(`a</STYLE>b`, "style"); escaped != `a<\/style>b` {
		t.Errorf("case-insensitive escaping = %q", escaped)
	}
}

func TestRuntimeFragmentsRejectsUnknownAssetMode(t *testing.T) {
	t.Parallel()

	_, _, err := runtimeFragments(RenderOptions{Assets: AssetMode(9)}, "body")
	var optionErr *OptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("runtimeFragments() error = %v, want OptionError", err)
	}
}

func TestInlineScriptCachesAndReportsMissingAssets(t *testing.T) {
	t.Parallel()

	if _, err := inlineScript("definitely-missing.js"); err == nil {
		t.Fatal("inlineScript() accepted a missing runtime script")
	}
	first, err := inlineScript("rich-content.js")
	if err != nil {
		t.Fatal(err)
	}
	second, err := inlineScript("rich-content.js")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("inlineScript() returned different contents for a cached script")
	}
}

func TestWriteInlineScriptReportsMissingAsset(t *testing.T) {
	t.Parallel()

	var scripts strings.Builder
	if err := writeInlineScript(&scripts, "definitely-missing.js"); err == nil {
		t.Fatal("writeInlineScript() accepted a missing runtime script")
	}
}
