package markdown

import (
	"slices"
	"testing"
)

// TestInspectHTMLBlockMissingBlanks covers the conservative blank-line-after
// analysis: only Type 6/7 blocks, only text that follows the top-level
// element's completion on a later line, and skip-anything-ambiguous.
func TestInspectHTMLBlockMissingBlanks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []HTMLBlockMissingBlank
	}{
		{
			name:   "close tag then text warns at the text line",
			source: "<div class=\"notice\">\n  Notice\n</div>\n**Important**\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "close table then heading",
			source: "<table>\n<tr><td>a</td></tr>\n</table>\n## Heading\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "close details then bold",
			source: "<details>\n<summary>S</summary>\n</details>\n**bold**\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "empty element pair then text",
			source: "<div></div>\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 2, Column: 1}}},
		},
		{
			name:   "void hr then text",
			source: "<hr>\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 2, Column: 1}}},
		},
		{
			name:   "self closing image then text",
			source: "<img src=\"x.png\">\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 2, Column: 1}}},
		},
		{
			name:   "type 7 custom element then text",
			source: "<my-element attr=\"1\">\ncontent\n</my-element>\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "blank line after close stays clean",
			source: "<div>\nx\n</div>\n\ntext\n",
			want:   nil,
		},
		{
			name:   "consecutive p elements inside div stay clean",
			source: "<div>\n  <p>First</p>\n  <p>Second</p>\n</div>\n",
			want:   nil,
		},
		{
			name:   "two consecutive top level elements stay clean",
			source: "<div>First</div>\n<div>Second</div>\n",
			want:   nil,
		},
		{
			name:   "script block is raw and out of scope",
			source: "<script>\nvar x = 1;\n</script>\n**text**\n",
			want:   nil,
		},
		{
			name:   "comment block is out of scope",
			source: "<!-- note -->\ntext\n",
			want:   nil,
		},
		{
			name:   "html inside a code fence is code",
			source: "```html\n<div>\nx\n</div>\ntext\n```\n",
			want:   nil,
		},
		{
			name:   "block ending at end of file stays clean",
			source: "<div>\nx\n</div>\n",
			want:   nil,
		},
		{
			name:   "uppercase tags still match",
			source: "<DIV>\nx\n</DIV>\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "gt inside an attribute value does not close",
			source: "<div title=\"a>b\">\nx\n</div>\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "fake close tag inside a comment does not close",
			source: "<div>\n<!-- </div> -->\n</div>\ntext\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "blockquote html blocks are skipped",
			source: "> <div>\n> x\n> </div>\n> text\n",
			want:   nil,
		},
		{
			name:   "mismatched nesting is skipped",
			source: "<div><span></div>\ntext\n",
			want:   nil,
		},
		{
			name:   "unclosed element is skipped",
			source: "<div>\ntext\n",
			want:   nil,
		},
		{
			name:   "stray close tag alone is skipped",
			source: "</div>\ntext\n",
			want:   nil,
		},
		{
			name:   "crlf lines locate correctly",
			source: "<div>\r\nx\r\n</div>\r\ntext\r\n",
			want:   []HTMLBlockMissingBlank{{Position: Position{Line: 4, Column: 1}}},
		},
		{
			name:   "text after each of two elements reports twice",
			source: "<div>a</div>\ntext1\n<span>s</span>\ntext2\n",
			want: []HTMLBlockMissingBlank{
				{Position: Position{Line: 2, Column: 1}},
				{Position: Position{Line: 4, Column: 1}},
			},
		},
		{
			name:   "same line trailing text stays unreported",
			source: "<div>a</div> tail\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			if !slices.Equal(inspection.HTMLBlockMissingBlanks, test.want) {
				t.Fatalf("html blank findings = %+v, want %+v", inspection.HTMLBlockMissingBlanks, test.want)
			}
		})
	}
}
