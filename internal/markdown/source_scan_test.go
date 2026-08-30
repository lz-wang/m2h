package markdown

import (
	"slices"
	"testing"
)

func TestInspectCollectsReferenceDefinitions(t *testing.T) {
	t.Parallel()

	source := []byte("Intro.\n\n[guide]: docs/guide.md \"Guide\"\n[logo]: images/logo.png\n")
	inspection := Inspect(source)

	want := []ReferenceDefinition{
		{Label: "guide", Destination: "docs/guide.md", Position: Position{Line: 3, Column: 1}},
		{Label: "logo", Destination: "images/logo.png", Position: Position{Line: 4, Column: 1}},
	}
	if !slices.Equal(inspection.ReferenceDefinitions, want) {
		t.Fatalf("definitions = %+v, want %+v", inspection.ReferenceDefinitions, want)
	}
}

func TestInspectDeduplicatesReferenceDefinitions(t *testing.T) {
	t.Parallel()

	// Goldmark resolves uses against the first definition of a label and
	// ignores later duplicates, so facts keep the first position only.
	inspection := Inspect([]byte("[foo]: a.md\n[foo]: b.md\n"))
	want := []ReferenceDefinition{
		{Label: "foo", Destination: "a.md", Position: Position{Line: 1, Column: 1}},
	}
	if !slices.Equal(inspection.ReferenceDefinitions, want) {
		t.Fatalf("definitions = %+v, want %+v", inspection.ReferenceDefinitions, want)
	}
}

func TestInspectReportsUndefinedReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []ReferenceUse
	}{
		{
			name:   "full reference without definition",
			source: "See [Guide][guide].\n",
			want:   []ReferenceUse{{Label: "guide", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "collapsed reference without definition",
			source: "See [missing][].\n",
			want:   []ReferenceUse{{Label: "missing", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "image reference without definition",
			source: "![alt][logo]\n",
			want:   []ReferenceUse{{Label: "logo", Position: Position{Line: 1, Column: 2}}},
		},
		{
			name:   "undefined before a later definition is still resolved",
			source: "[Guide][guide]\n\n[guide]: guide.md\n",
			want:   nil,
		},
		{
			name:   "label case folding resolves against the definition",
			source: "[Guide][GUIDE]\n\n[guide]: guide.md\n",
			want:   nil,
		},
		{
			name:   "shortcut brackets stay prose",
			source: "Array indexing like [index] is not a link.\n",
			want:   nil,
		},
		{
			name:   "escaped brackets are not uses",
			source: "\\[foo\\][bar]\n",
			want:   nil,
		},
		{
			name:   "inline code protects the use",
			source: "Use `[Guide][guide]` verbatim.\n",
			want:   nil,
		},
		{
			name:   "fenced code protects the use",
			source: "```md\n[Guide][guide]\n```\n",
			want:   nil,
		},
		{
			name:   "indented code protects the use",
			source: "    [Guide][guide]\n",
			want:   nil,
		},
		{
			name:   "defined label is never undefined",
			source: "[Guide][guide]\n\n[guide]: guide.md\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []ReferenceUse{}
			}
			if !slices.Equal(inspection.UndefinedReferences, want) {
				t.Fatalf("undefined references = %+v, want %+v", inspection.UndefinedReferences, want)
			}
		})
	}
}

func TestInspectUndefinedReferenceSkipsMixedForms(t *testing.T) {
	t.Parallel()

	// Only the genuinely undefined uses survive; the resolved neighbor on
	// the same line does not.
	source := []byte("[Good][good] and [Bad][bad]\n\n[good]: good.md\n")
	inspection := Inspect(source)
	want := []ReferenceUse{{Label: "bad", Position: Position{Line: 1, Column: 18}}}
	if !slices.Equal(inspection.UndefinedReferences, want) {
		t.Fatalf("undefined references = %+v, want %+v", inspection.UndefinedReferences, want)
	}
}

func TestInspectUndefinedReferenceSkipsRawHTML(t *testing.T) {
	t.Parallel()

	// Goldmark never interprets brackets inside raw HTML — comment blocks,
	// inline comments, HTML blocks and the literal-content elements like
	// <code> — so however link-like the text there looks, it is prose the
	// parser never attempted to resolve and can never be undefined.
	tests := []struct {
		name   string
		source string
		want   []ReferenceUse
	}{
		{
			name:   "comment block protects all but the real use",
			source: "[Real][x]\n\n<!--\n[Example][x]\n-->\n",
			want:   []ReferenceUse{{Label: "x", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "inline comment protects the shape",
			source: "Text <!-- [Example][x] --> here.\n",
			want:   []ReferenceUse{},
		},
		{
			name:   "html block protects the shape",
			source: "<div>\n[Example][x]\n</div>\n",
			want:   []ReferenceUse{},
		},
		{
			name:   "literal code element protects the shape",
			source: "Use <code>[Example][x]</code> verbatim.\n",
			want:   []ReferenceUse{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []ReferenceUse{}
			}
			if !slices.Equal(inspection.UndefinedReferences, want) {
				t.Fatalf("undefined references = %+v, want %+v", inspection.UndefinedReferences, want)
			}
		})
	}
}

func TestInspectUndefinedReferenceCappedByParserRejections(t *testing.T) {
	t.Parallel()

	// The source scan can find more bracket pairs than the parser attempted
	// to resolve — a link title is literal text and never triggers a lookup —
	// so the reported uses are capped by the parser's actual rejection count
	// per label: the title's [missing] here must not become a diagnostic.
	// A collapsed use registers exactly one lookup (its leftover bracket
	// pair looks up the empty label), leaving no budget for the fake pair.
	source := []byte("See [missing][] or [a](b \"t [x][missing]\").\n")
	inspection := Inspect(source)

	want := []ReferenceUse{{Label: "missing", Position: Position{Line: 1, Column: 5}}}
	if !slices.Equal(inspection.UndefinedReferences, want) {
		t.Fatalf("undefined references = %+v, want %+v", inspection.UndefinedReferences, want)
	}
}

func TestInspectUndefinedFootnoteSkipsRawHTML(t *testing.T) {
	t.Parallel()

	// Same boundary as undefined references: a marker inside raw HTML is
	// displayed verbatim, never parsed as a footnote, so it cannot be
	// undefined.
	tests := []struct {
		name   string
		source string
		want   []FootnoteReference
	}{
		{
			name:   "comment block protects the marker",
			source: "# Document\n\n<!--\nExample syntax: [^demo]\n-->\n",
			want:   []FootnoteReference{},
		},
		{
			name:   "inline comment protects the marker",
			source: "Syntax <!-- [^demo] --> shown.\n",
			want:   []FootnoteReference{},
		},
		{
			name:   "literal code element protects the marker",
			source: "Use <code>[^demo]</code> verbatim.\n",
			want:   []FootnoteReference{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []FootnoteReference{}
			}
			if !slices.Equal(inspection.UndefinedFootnotes, want) {
				t.Fatalf("undefined footnotes = %+v, want %+v", inspection.UndefinedFootnotes, want)
			}
		})
	}
}

func TestInspectCollectsFootnotes(t *testing.T) {
	t.Parallel()

	source := []byte("# Guide\n\nText[^a] and [^a] again.\n\n[^a]: first note\n\n[^b]: orphan\n\n[^e]:\n")
	inspection := Inspect(source)

	want := []Footnote{
		{Label: "a", Used: true, Empty: false, Position: Position{Line: 5, Column: 1}},
		{Label: "b", Used: false, Empty: false, Position: Position{Line: 7, Column: 1}},
		{Label: "e", Used: false, Empty: true, Position: Position{Line: 9, Column: 1}},
	}
	if !slices.Equal(inspection.Footnotes, want) {
		t.Fatalf("footnotes = %+v, want %+v", inspection.Footnotes, want)
	}
}

func TestInspectMultilineFootnoteIsNotEmpty(t *testing.T) {
	t.Parallel()

	source := []byte("[^m]:\n    multiline\n    content\n\nText[^m].\n")
	inspection := Inspect(source)
	if len(inspection.Footnotes) != 1 || inspection.Footnotes[0].Empty || !inspection.Footnotes[0].Used {
		t.Fatalf("footnotes = %+v, want one used multiline definition", inspection.Footnotes)
	}
}

func TestInspectUndefinedFootnotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []FootnoteReference
	}{
		{
			name:   "marker without definition",
			source: "Text[^missing].\n",
			want:   []FootnoteReference{{Label: "missing", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "defined marker stays clean",
			source: "Text[^ok].\n\n[^ok]: note\n",
			want:   nil,
		},
		{
			name:   "use before definition still resolves",
			source: "Text[^later].\n\n[later]: x\n\n[^later]: note\n",
			want:   nil,
		},
		{
			name:   "labels compare by exact bytes",
			source: "Text[^Missing].\n\n[^missing]: note\n",
			want:   []FootnoteReference{{Label: "Missing", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "inline code protects the marker",
			source: "Use `[^missing]` verbatim.\n",
			want:   nil,
		},
		{
			name:   "fenced code protects the marker",
			source: "```md\n[^missing]\n```\n",
			want:   nil,
		},
		{
			name:   "escaped marker stays text",
			source: "Use \\[^missing] verbatim.\n",
			want:   nil,
		},
		{
			name:   "empty marker is not a use",
			source: "Use [^] verbatim.\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []FootnoteReference{}
			}
			if !slices.Equal(inspection.UndefinedFootnotes, want) {
				t.Fatalf("undefined footnotes = %+v, want %+v", inspection.UndefinedFootnotes, want)
			}
		})
	}
}

func TestInspectCollectsCodeFences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []CodeFence
	}{
		{
			name:   "language and position",
			source: "Text.\n\n```go\nfmt.Println(1)\n```\n",
			want:   []CodeFence{{Language: "go", Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "info string keeps only the language",
			source: "```go title=main.go\nx\n```\n",
			want:   []CodeFence{{Language: "go", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "tilde fences with language",
			source: "~~~python\nx\n~~~\n",
			want:   []CodeFence{{Language: "python", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "no language",
			source: "```\nx\n```\n",
			want:   []CodeFence{{Language: "", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "indented fence with language",
			source: "  ```js\nx\n  ```\n",
			want:   []CodeFence{{Language: "js", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "unclosed fence runs to the end",
			source: "```ruby\nx\n",
			want:   []CodeFence{{Language: "ruby", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "indented code block is not a fence",
			source: "    plain code\n",
			want:   []CodeFence{},
		},
		{
			name:   "fence inside inline code is not a fence",
			source: "Use ```go inline.\n",
			want:   []CodeFence{},
		},
		{
			name:   "empty fence without language",
			source: "```\n```\n",
			want:   []CodeFence{{Language: "", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "fence inside a blockquote",
			source: "> ```\n> code\n> ```\n",
			want:   []CodeFence{{Language: "", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "named fence inside a list item",
			source: "- ```js\n  code\n  ```\n",
			want:   []CodeFence{{Language: "js", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "empty fence inside a blockquote",
			source: "> ```\n> ```\n",
			want:   []CodeFence{{Language: "", Position: Position{Line: 1, Column: 1}}},
		},
		{
			name:   "two consecutive empty fences",
			source: "```\n```\n\n```\n```\n",
			want: []CodeFence{
				{Language: "", Position: Position{Line: 1, Column: 1}},
				{Language: "", Position: Position{Line: 4, Column: 1}},
			},
		},
		{
			name:   "fence-looking lines inside an html block are not fences",
			source: "<div>\n```\nnot fence\n```\n</div>\n",
			want:   []CodeFence{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			if !slices.Equal(inspection.CodeFences, test.want) {
				t.Fatalf("code fences = %+v, want %+v", inspection.CodeFences, test.want)
			}
		})
	}
}

func TestInspectCodeFenceInfoStringIsProtected(t *testing.T) {
	t.Parallel()

	// A reference-looking info string is literal text, never a link the
	// parser attempted, so it cannot become an undefined reference.
	inspection := Inspect([]byte("```[a][b]\nx\n```\n"))
	if len(inspection.UndefinedReferences) != 0 {
		t.Fatalf("undefined references = %+v, want none for fence info strings", inspection.UndefinedReferences)
	}
	if len(inspection.CodeFences) != 1 || inspection.CodeFences[0].Language != "[a][b]" {
		t.Fatalf("code fences = %+v, want the info string kept as the language", inspection.CodeFences)
	}
}

func TestInspectionShiftLines(t *testing.T) {
	t.Parallel()

	source := []byte("# Title\n\nSee [Good][defined] and [Bad][missing].\n\n[defined]: x.md\n\n```go\ncode\n```\n")
	inspection := Inspect(source)
	inspection.ShiftLines(3)

	if inspection.Headings[0].Line != 4 {
		t.Fatalf("heading line = %d, want 4", inspection.Headings[0].Line)
	}
	if inspection.References[0].Line != 6 || inspection.References[0].Column != 6 {
		t.Fatalf("reference position = %d:%d, want 6:6", inspection.References[0].Line, inspection.References[0].Column)
	}
	if inspection.UndefinedReferences[0].Position.Line != 6 {
		t.Fatalf("undefined reference line = %d, want 6", inspection.UndefinedReferences[0].Position.Line)
	}
	if inspection.ReferenceDefinitions[0].Position.Line != 8 {
		t.Fatalf("definition line = %d, want 8", inspection.ReferenceDefinitions[0].Position.Line)
	}
	if inspection.CodeFences[0].Position.Line != 10 {
		t.Fatalf("code fence line = %d, want 10", inspection.CodeFences[0].Position.Line)
	}
}

func TestMatchBracketNestingAndEscapes(t *testing.T) {
	t.Parallel()

	source := []byte("[a [b] c][label] tail")
	if close, ok := matchBracket(source, 0, true); !ok || close != 8 {
		t.Fatalf("matchBracket(nesting) = %d,%v want 8,true", close, ok)
	}
	if close, ok := matchBracket(source, 9, false); !ok || close != 15 {
		t.Fatalf("matchBracket(label) = %d,%v want 15,true", close, ok)
	}
	if _, ok := matchBracket([]byte("[a\\]b"), 0, true); ok {
		t.Fatal("matchBracket should honor backslash escapes")
	}
	if _, ok := matchBracket([]byte("[a\n\nb"), 0, true); ok {
		t.Fatal("matchBracket should stop at a blank line")
	}
}
