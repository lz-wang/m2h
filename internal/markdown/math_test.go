package markdown

import (
	"html"
	"strings"
	"testing"
)

func TestMathPreservesLiteralSource(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, source, literal string }{
		{"display DP", "$$\ndp[r][c]\n=\n\\min(dp[r-1][c],dp[r][c-1])\n$$\n", "$$\ndp[r][c]\n=\n\\min(dp[r-1][c],dp[r][c-1])\n$$\n"},
		{"inline", "$dp[i][j] = a_b * c_d + \\{x\\} + \\\\ + <x> & y$", "$dp[i][j] = a_b * c_d + \\{x\\} + \\\\ + <x> & y$"},
		{"same line display", "$$dp[i][j] = a_b * c_d$$", "$$dp[i][j] = a_b * c_d$$"},
		{"quoted block", "> $$\n> dp[i][j]\n> =\n> x\n> $$\n", "$$\ndp[i][j]\n=\nx\n$$\n"},
		{"list block", "- $$\n  dp[i][j]\n  =\n  x\n  $$\n", "$$\ndp[i][j]\n=\nx\n$$\n"},
		{"blank lines and HTML", "$$\nx < y & z\n\n<img src=\"missing.png\">\n[^a]: b\n|a|b|\n|-|-|\n$$\n", "$$\nx < y & z\n\n<img src=\"missing.png\">\n[^a]: b\n|a|b|\n|-|-|\n$$\n"},
		{"escaped dollar in math", "$x + \\$ + y$", "$x + \\$ + y$"},
		{"unicode", "$α + β$", "$α + β$"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := []byte("# Title\n\n## Formula\n\n" + tt.source + "\n\n### After\n\nText.\n")
			for _, mode := range []URLMode{URLPassthrough, URLWeb} {
				result, err := Render(source, RenderOptions{SourcePath: "math.md", URLMode: mode})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(result.Body, html.EscapeString(tt.literal)) {
					t.Fatalf("formula changed: %s", result.Body)
				}
				if len(result.Headings) != 3 || result.Headings[2].Text != "After" {
					t.Fatalf("headings polluted: %+v", result.Headings)
				}
			}
			inspection := Inspect(source)
			if inspection.H1Count != 1 || len(inspection.UndefinedReferences) != 0 || len(inspection.References) != 0 || len(inspection.Footnotes) != 0 || len(inspection.TableMismatches) != 0 {
				t.Fatalf("math inspected as Markdown: %+v", inspection)
			}
		})
	}
}

func TestMathKeepsMarkdownBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, source, want string }{
		{"currency", "$9 and $200", "<p>$9 and $200</p>"},
		{"spaced dollar", "$ **bold** $", "<strong>bold</strong>"},
		{"unmatched inline", "$x **bold**", "<strong>bold</strong>"},
		{"escaped opener", "\\$x **bold**$", "<strong>bold</strong>"},
		{"long dollar run", "$$$**bold**$$$", "<strong>bold</strong>"},
		{"inline code", "`$x * y$`", "<code>$x * y$</code>"},
		{"fenced code", "```text\n$$\nx\n=\n$$\n```", "<pre class=\"chroma\">"},
		{"indented code", "    $$\n    x\n    $$\n", "<pre><code>$$"},
		{"raw HTML", "<pre>\n$$\nx\n=\n$$\n</pre>\n", "<pre>\n$$\nx\n=\n$$\n</pre>"},
		{"real setext", "Real heading\n=\n", "<h1 id=\"real-heading\">Real heading</h1>"},
		{"math in heading", "# $a_b$\n", `<span class="m2h-math">$a_b$</span></h1>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render([]byte(tt.source), RenderOptions{SourcePath: "math.md"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.Body, tt.want) {
				t.Fatalf("want %q in %s", tt.want, result.Body)
			}
		})
	}
}

func TestMathSearchKeepsFormulaAndSection(t *testing.T) {
	t.Parallel()
	source := []byte("# $a_b$\n\n$$\ndp[i][j]\n=\nx\n$$\n\nInline $c_d$ text.\n")
	projection, err := ProjectForSearch(source, "math.md")
	if err != nil {
		t.Fatal(err)
	}
	if projection.Title != "$a_b$" || len(projection.Chunks) != 3 {
		t.Fatalf("math lost from search: %+v", projection)
	}
	for _, chunk := range projection.Chunks {
		if chunk.HeadingText != "$a_b$" {
			t.Fatalf("fake math section: %+v", chunk)
		}
	}
	if projection.Chunks[1].Kind != SearchChunkText || !strings.Contains(projection.Chunks[1].Text, "dp[i][j] = x") || projection.Chunks[2].Text != "Inline $c_d$ text." {
		t.Fatalf("formula lost: %+v", projection)
	}
}

func TestMathBlockBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, source, want string }{
		{"interrupt paragraph", "Intro\n$$\nx\n=\ny\n$$\n", "<p>Intro</p>\n<div class=\"m2h-math\">$$\nx\n=\ny\n$$\n</div>"},
		{"unclosed block stays literal", "$$\nx\n=\ny", "<div class=\"m2h-math\">$$\nx\n=\ny</div>"},
		{"unclosed quote ends at container", "> $$\n> x\n\n# Outside\n", "<h1 id=\"outside\">Outside</h1>"},
		{"empty block", "$$\n$$\n", "<div class=\"m2h-math\">$$\n$$\n</div>"},
		{"CRLF", "$$\r\nx\r\n=\r\ny\r\n$$\r\n", "$$\r\nx\r\n=\r\ny\r\n$$\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render([]byte(tt.source), RenderOptions{SourcePath: "math.md"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.Body, tt.want) {
				t.Fatalf("want %q in %s", tt.want, result.Body)
			}
		})
	}
}
