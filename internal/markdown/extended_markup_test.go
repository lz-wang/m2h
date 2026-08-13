package markdown

import (
	"strings"
	"testing"
)

// renderBody is a small helper that renders source as convert output and
// returns only the document body, failing the test on render errors.
func renderBody(t *testing.T, source string) string {
	t.Helper()
	result, err := Render([]byte(source), RenderOptions{
		Mode:       ModeLight,
		Target:     TargetConvert,
		SourcePath: "doc.md",
	})
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}
	return result.Body
}

// containsInOrder asserts that every substring in parts appears in body, and
// in the same order. It is used where the exact attribute set produced by
// link rewriting is not the point of the test.
func containsInOrder(t *testing.T, body string, parts ...string) bool {
	t.Helper()
	from := 0
	for _, want := range parts {
		index := strings.Index(body[from:], want)
		if index < 0 {
			t.Errorf("body does not contain %q (after previous match):\n%s", want, body)
			return false
		}
		from += index + len(want)
	}
	return true
}

func TestExtendedMarkupMarkAndCaret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  string
		want    string
		notWant string
	}{
		{
			name:   "double equals renders mark",
			source: "==高亮==",
			want:   "<mark>高亮</mark>",
		},
		{
			name:   "double caret renders ins",
			source: "^^插入^^",
			want:   "<ins>插入</ins>",
		},
		{
			name:   "mark wraps nested strong",
			source: "==**bold**==",
			want:   "<mark><strong>bold</strong></mark>",
		},
		{
			name:    "triple equals is literal",
			source:  "===foo===",
			want:    "===foo===",
			notWant: "<mark>",
		},
		{
			name:    "unmatched opener stays literal",
			source:  "a==b",
			want:    "a==b",
			notWant: "<mark>",
		},
		{
			name:   "single equals is literal punctuation",
			source: "key=value",
			want:   "key=value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := renderBody(t, tt.source)
			if tt.want != "" && !strings.Contains(body, tt.want) {
				t.Errorf("body does not contain %q:\n%s", tt.want, body)
			}
			if tt.notWant != "" && strings.Contains(body, tt.notWant) {
				t.Errorf("body unexpectedly contains %q:\n%s", tt.notWant, body)
			}
		})
	}
}

func TestExtendedMarkupCaretWrapsLink(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "^^[guide](docs/guide.md)^^")
	containsInOrder(t, body, "<ins>", "guide</a>", "</ins>")
}

func TestExtendedMarkupStrikethroughUnchanged(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "~~deleted~~")
	if !strings.Contains(body, "<del>deleted</del>") {
		t.Fatalf("strikethrough no longer renders <del>:\n%s", body)
	}
}
