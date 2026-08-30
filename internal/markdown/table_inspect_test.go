package markdown

import (
	"slices"
	"testing"
)

func TestInspectTableDataRowMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []TableColumnMismatch
	}{
		{
			name:   "long row is truncated by the renderer",
			source: "| A | B |\n|---|---|\n| 1 | 2 | 3 |\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchRow, Expected: 2, Actual: 3, Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "short row is padded by the renderer",
			source: "| A | B |\n|---|---|\n| 1 |\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchRow, Expected: 2, Actual: 1, Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "matching rows stay clean",
			source: "| A | B |\n|---|---|\n| 1 | 2 |\n",
			want:   nil,
		},
		{
			name:   "escaped pipe does not split a cell",
			source: "| A | B |\n|---|---|\n| 1 \\| x | 2 |\n",
			want:   nil,
		},
		{
			name:   "bare pipe inside a code span still splits",
			source: "| A | B |\n|---|---|\n| `a|b` | 2 |\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchRow, Expected: 2, Actual: 3, Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "rows without border pipes",
			source: "A | B\n--- | ---\n1 | 2 | 3\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchRow, Expected: 2, Actual: 3, Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "leading and trailing pipes both optional",
			source: "| A | B |\n| --- | --- |\n| 1 | 2 |\n",
			want:   nil,
		},
		{
			name:   "bare pipe row declares no cells at all",
			source: "| A | B |\n|---|---|\n|\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchRow, Expected: 2, Actual: 0, Position: Position{Line: 3, Column: 1}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []TableColumnMismatch{}
			}
			if !slices.Equal(inspection.TableMismatches, want) {
				t.Fatalf("table mismatches = %+v, want %+v", inspection.TableMismatches, want)
			}
		})
	}
}

func TestInspectTableDelimiterMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []TableColumnMismatch
	}{
		{
			name:   "delimiter shorter than the header",
			source: "| A | B | C |\n|---|---|\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchDelimiter, Expected: 3, Actual: 2, Position: Position{Line: 2, Column: 1}}},
		},
		{
			name:   "delimiter longer than the header pads the header row",
			source: "| A |\n|---|---|\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchRow, Expected: 2, Actual: 1, Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "matching header and delimiter stay clean",
			source: "| A | B |\n|---|---|\n",
			want:   nil,
		},
		{
			name:   "colon-only delimiter counts as one column",
			source: "| A | B |\n:---\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchDelimiter, Expected: 2, Actual: 1, Position: Position{Line: 2, Column: 1}}},
		},
		{
			name:   "setext rule after text is a heading, not a table",
			source: "Some text\n---\n",
			want:   nil,
		},
		{
			name:   "thematic break alone is not a table",
			source: "Intro.\n\n***\n",
			want:   nil,
		},
		{
			name:   "valid table converts and leaves no paragraph lines",
			source: "Intro text.\n\n| A |\n|---|\n| 1 |\n",
			want:   nil,
		},
		{
			name:   "text above a broken pair is still reported",
			source: "Intro.\n| A | B | C |\n|---|---|\n",
			want:   []TableColumnMismatch{{Kind: TableMismatchDelimiter, Expected: 3, Actual: 2, Position: Position{Line: 3, Column: 1}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []TableColumnMismatch{}
			}
			if !slices.Equal(inspection.TableMismatches, want) {
				t.Fatalf("table mismatches = %+v, want %+v", inspection.TableMismatches, want)
			}
		})
	}
}

func TestInspectUnclosedComments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []UnclosedComment
	}{
		{
			name:   "unclosed comment swallows the rest",
			source: "# A\n\n<!-- forgotten\n\n## B\n\ncontent\n",
			want:   []UnclosedComment{{Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "closed inline comment",
			source: "# A\n\n<!-- note -->\n\nText.\n",
			want:   nil,
		},
		{
			name:   "closed multiline comment",
			source: "<!-- spans\nlines -->\n\nText.\n",
			want:   nil,
		},
		{
			name:   "abruptly closed empty comments",
			source: "<!-->\n\nText.\n\n<!--->\n",
			want:   nil,
		},
		{
			name:   "comment inside inline code is text",
			source: "Use `<!-- oops` inline.\n",
			want:   nil,
		},
		{
			name:   "comment inside fenced code is code",
			source: "```html\n<!-- forgotten\n```\n",
			want:   nil,
		},
		{
			name:   "mid-paragraph unclosed marker stays escaped text",
			source: "Text with <!-- oops inline.\n\nMore text.\n",
			want:   nil,
		},
		{
			name:   "later closer after fenced-looking content closes",
			source: "<!-- broken\n```\ncode\n```\n-->\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []UnclosedComment{}
			}
			if !slices.Equal(inspection.UnclosedComments, want) {
				t.Fatalf("unclosed comments = %+v, want %+v", inspection.UnclosedComments, want)
			}
		})
	}
}

func TestInspectReversedLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []ReversedLink
	}{
		{
			name:   "https destination",
			source: "See (OpenAI)[https://openai.com] now.\n",
			want:   []ReversedLink{{Text: "OpenAI", Destination: "https://openai.com", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "markdown file destination",
			source: "See (Guide)[guide.md].\n",
			want:   []ReversedLink{{Text: "Guide", Destination: "guide.md", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "anchor destination",
			source: "(Section)[#setup]\n",
			want:   []ReversedLink{{Text: "Section", Destination: "#setup", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "relative path destination",
			source: "(Docs)[../docs/x.md]\n",
			want:   []ReversedLink{{Text: "Docs", Destination: "../docs/x.md", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "function call indexing stays prose",
			source: "Call f(x)[0] for the first item.\n",
			want:   nil,
		},
		{
			name:   "array indexing stays prose",
			source: "Use array[index] here.\n",
			want:   nil,
		},
		{
			name:   "normal link syntax is never reversed",
			source: "See [Guide](guide.md) instead.\n",
			want:   nil,
		},
		{
			name:   "code protects the shape",
			source: "Use `(x)[y.md]` and:\n\n```md\n(x)[y.md]\n```\n",
			want:   nil,
		},
		{
			name:   "escaped parens stay prose",
			source: "Use \\(x\\)[y] here.\n",
			want:   nil,
		},
		{
			name:   "version-like destination stays prose",
			source: "Compare (version)[v1.2] and (section)[1.2.3].\n",
			want:   nil,
		},
		{
			name:   "html comment protects the shape",
			source: "<!--\nBad example: (Guide)[guide.md]\n-->\n",
			want:   nil,
		},
		{
			name:   "html block protects the shape",
			source: "<div>\n(Guide)[guide.md]\n</div>\n",
			want:   nil,
		},
		{
			name:   "literal code element protects the shape",
			source: "Use <code>(Guide)[guide.md]</code> verbatim.\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []ReversedLink{}
			}
			if !slices.Equal(inspection.ReversedLinks, want) {
				t.Fatalf("reversed links = %+v, want %+v", inspection.ReversedLinks, want)
			}
		})
	}
}

func TestCountTableColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want int
	}{
		{"| a | b |", 2},
		{"a | b", 2},
		{"|a|b|c|", 3},
		{"a", 1},
		{"|", 0},
		{"||", 0},
		{`| a \| b |`, 1},
		{"| `x|y` |", 2},
		{"", 0},
	}
	for _, test := range tests {
		if got := countTableColumns([]byte(test.line)); got != test.want {
			t.Errorf("countTableColumns(%q) = %d, want %d", test.line, got, test.want)
		}
	}
}

func TestDelimiterRowColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line    string
		columns int
		ok      bool
	}{
		{"|---|---|", 2, true},
		{"--- | ---", 2, true},
		{":---", 1, true},
		{"---:", 1, true},
		{":-:", 1, true},
		{"-", 0, false},
		{"---", 0, false},
		{"x---", 0, false},
		{"    ---", 0, false},
		{"|---|x|", 0, false},
	}
	for _, test := range tests {
		columns, ok := delimiterRowColumns([]byte(test.line))
		if ok != test.ok || columns != test.columns {
			t.Errorf("delimiterRowColumns(%q) = %d,%v want %d,%v", test.line, columns, ok, test.columns, test.ok)
		}
	}
}
