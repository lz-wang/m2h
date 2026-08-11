package markdown

import (
	"testing"

	"github.com/yuin/goldmark/ast"
)

func TestGitHubSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ascii words", input: "Hello World", want: "hello-world"},
		{name: "leading number and punctuation", input: "7. 代码", want: "7-代码"},
		{name: "plus signs stripped", input: "C++ API", want: "c-api"},
		{name: "cjk preserved", input: "Hello 世界", want: "hello-世界"},
		{name: "emphasis markers stripped", input: "Foo *Bar*", want: "foo-bar"},
		{name: "underscore kept", input: "foo_bar baz", want: "foo_bar-baz"},
		{name: "collapse whitespace runs", input: "Foo   Bar", want: "foo-bar"},
		{name: "trim surrounding space", input: "  Hello  ", want: "hello"},
		{name: "only punctuation falls back empty", input: "!!!", want: ""},
		{name: "mixed punctuation", input: "What's New? (v2)", want: "whats-new-v2"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := githubSlug(test.input); got != test.want {
				t.Fatalf("githubSlug(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestGitHubIDsGenerateDedups(t *testing.T) {
	t.Parallel()

	ids := newGitHubIDs()
	cases := []struct {
		input string
		want  string
	}{
		{input: "API", want: "api"},
		{input: "API", want: "api-1"},
		{input: "API", want: "api-2"},
		{input: "7. 代码", want: "7-代码"},
		{input: "7. 代码", want: "7-代码-1"},
		{input: "Hello World", want: "hello-world"},
		{input: "!!!", want: "section"},
		{input: "!!!", want: "section-1"},
	}
	for _, c := range cases {
		got := string(ids.Generate([]byte(c.input), ast.KindHeading))
		if got != c.want {
			t.Fatalf("Generate(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestGitHubIDsPutRecordsExisting(t *testing.T) {
	t.Parallel()

	ids := newGitHubIDs()
	ids.Put([]byte("taken"))
	if got := string(ids.Generate([]byte("taken"), ast.KindHeading)); got != "taken-1" {
		t.Fatalf("Generate after Put = %q, want taken-1", got)
	}
}
