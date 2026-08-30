package markdown

import (
	"slices"
	"testing"
)

func TestInspectEmptySections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []EmptySection
	}{
		{
			name:   "parent heading with only subsections",
			source: "# Guide\n\n## Installation\n\n### Linux\n\nbody\n",
			want: []EmptySection{
				{Level: 1, Text: "Guide", Position: Position{Line: 1, Column: 1}},
				{Level: 2, Text: "Installation", Position: Position{Line: 3, Column: 1}},
			},
		},
		{
			name:   "heading with a paragraph has content",
			source: "# Guide\n\nIntro.\n\n## Install\n\nRun it.\n",
			want:   nil,
		},
		{
			name:   "comments and definitions render nothing",
			source: "# Guide\n\nIntro.\n\n## Install\n\n<!-- note -->\n\n[ref]: https://example.com\n\n## Next\n\nbody\n",
			want: []EmptySection{
				{Level: 2, Text: "Install", Position: Position{Line: 5, Column: 1}},
			},
		},
		{
			name:   "thematic break renders nothing",
			source: "# Guide\n\nIntro.\n\n## Install\n\n***\n\n## Next\n\nbody\n",
			want: []EmptySection{
				{Level: 2, Text: "Install", Position: Position{Line: 5, Column: 1}},
			},
		},
		{
			name:   "lists, tables and code count as content",
			source: "# Guide\n\nIntro.\n\n## A\n\n- item\n\n## B\n\n| a |\n|---|\n| 1 |\n\n## C\n\n```\nx\n```\n\n## D\n\n> quote\n",
			want:   nil,
		},
		{
			name:   "final heading without content",
			source: "# Guide\n\nbody\n\n## Tail\n",
			want: []EmptySection{
				{Level: 2, Text: "Tail", Position: Position{Line: 5, Column: 1}},
			},
		},
		{
			name:   "raw HTML counts as content",
			source: "# Guide\n\nIntro.\n\n## Panel\n\n<div>panel</div>\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []EmptySection{}
			}
			if !slices.Equal(inspection.EmptySections, want) {
				t.Fatalf("empty sections = %+v, want %+v", inspection.EmptySections, want)
			}
		})
	}
}

func TestInspectMojibake(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []Mojibake
	}{
		{
			name:   "re-encoded accented letter",
			source: "Grab a CafÃ©.\n",
			want:   []Mojibake{{Pattern: "Ã©", Position: Position{Line: 1, Column: 11}}},
		},
		{
			name:   "re-encoded curly punctuation",
			source: "Itâ€™s here.\n",
			want:   []Mojibake{{Pattern: "â€™", Position: Position{Line: 1, Column: 3}}},
		},
		{
			name:   "stray BOM in the middle of a document",
			source: "Text\n\nï»¿More\n",
			want:   []Mojibake{{Pattern: "ï»¿", Position: Position{Line: 3, Column: 1}}},
		},
		{
			name:   "correct text never matches",
			source: "Grab a Café. It’s fine. ❤️\n",
			want:   nil,
		},
		{
			name:   "code regions are ignored",
			source: "Use `CafÃ©` and:\n\n```md\nItâ€™s\n```\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []Mojibake{}
			}
			if !slices.Equal(inspection.Mojibake, want) {
				t.Fatalf("mojibake = %+v, want %+v", inspection.Mojibake, want)
			}
		})
	}
}

func TestInspectInvisibleCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []InvisibleCharacter
	}{
		{
			name:   "zero width space at line start",
			source: "clean\n​hidden line\n",
			want:   []InvisibleCharacter{{Rune: 0x200B, Name: "ZERO WIDTH SPACE", Position: Position{Line: 2, Column: 1}}},
		},
		{
			name:   "zero width space next to a space",
			source: "word ​word\n",
			want:   []InvisibleCharacter{{Rune: 0x200B, Name: "ZERO WIDTH SPACE", Position: Position{Line: 1, Column: 6}}},
		},
		{
			name:   "consecutive invisible characters",
			source: "word​​word\n",
			want: []InvisibleCharacter{
				{Rune: 0x200B, Name: "ZERO WIDTH SPACE", Position: Position{Line: 1, Column: 5}},
				{Rune: 0x200B, Name: "ZERO WIDTH SPACE", Position: Position{Line: 1, Column: 8}},
			},
		},
		{
			name:   "bidi override is suspicious anywhere",
			source: "safe‮safe\n",
			want:   []InvisibleCharacter{{Rune: 0x202E, Name: "RIGHT-TO-LEFT OVERRIDE", Position: Position{Line: 1, Column: 5}}},
		},
		{
			name:   "ZWSP between letters stays legitimate",
			source: "word​word\n",
			want:   nil,
		},
		{
			name:   "emoji sequences never match",
			source: "Love ❤️ and 👩‍❤️‍👨 and 🏳️‍🌈.\n",
			want:   nil,
		},
		{
			name:   "soft hyphen inside a word stays legitimate",
			source: "pro­gram\n",
			want:   nil,
		},
		{
			name:   "code regions are ignored",
			source: "Use `​` and:\n\n```md\n​\n```\n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inspection := Inspect([]byte(test.source))
			want := test.want
			if want == nil {
				want = []InvisibleCharacter{}
			}
			if !slices.Equal(inspection.InvisibleCharacters, want) {
				t.Fatalf("invisible characters = %+v, want %+v", inspection.InvisibleCharacters, want)
			}
		})
	}
}
