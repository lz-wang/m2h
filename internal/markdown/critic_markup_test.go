package markdown

import (
	"strings"
	"testing"
)

func TestCriticInline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  string
		want    string
		notWant string
	}{
		{
			name:   "highlight",
			source: "{==重要==}",
			want:   `<mark class="m2h-critic">重要</mark>`,
		},
		{
			name:   "comment",
			source: "{>>备注内容<<}",
			want:   `<span class="m2h-critic m2h-critic-comment">备注内容</span>`,
		},
		{
			name:   "delete",
			source: "{--弃用--}",
			want:   `<del class="m2h-critic m2h-critic-delete">弃用</del>`,
		},
		{
			name:    "insert is not keys",
			source:  "{++新增++}",
			want:    `<ins class="m2h-critic m2h-critic-insert">新增</ins>`,
			notWant: `<span class="keys">`,
		},
		{
			name:   "substitution splits into del and ins",
			source: "{~~旧内容~>新内容~~}",
			want:   `<del class="m2h-critic m2h-critic-delete">旧内容</del><ins class="m2h-critic m2h-critic-insert">新内容</ins>`,
		},
		{
			name:   "highlight inside a sentence",
			source: "前文 {==词==} 后文",
			want:   `前文 <mark class="m2h-critic">词</mark> 后文`,
		},
		{
			name:   "no braces left behind",
			source: "{==x==}",
			want:   `<mark class="m2h-critic">x</mark>`,
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

func TestCriticInlineEscapesContent(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "{~~a<b~>c>d~~}")
	containsInOrder(t, body,
		`<del class="m2h-critic m2h-critic-delete">a&lt;b</del>`,
		`<ins class="m2h-critic m2h-critic-insert">c&gt;d</ins>`)
}

func TestCriticInlineUnclosedStaysLiteral(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "{==未闭合")
	// The inner text must not be silently swallowed, and no critic mark is
	// produced for an opener without a closer on the same line.
	if !strings.Contains(body, "未闭合") {
		t.Errorf("unclosed critic swallowed content:\n%s", body)
	}
	if strings.Contains(body, `<mark class="m2h-critic">`) {
		t.Errorf("unclosed critic produced a mark:\n%s", body)
	}
}

func TestCriticInlineDoesNotFireInCodeSpan(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "`{==not critic==}`")
	if !strings.Contains(body, "<code>{==not critic==}</code>") {
		t.Errorf("critic fired inside a code span:\n%s", body)
	}
}

func TestCriticBlockHighlight(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "{==\n\n段落内容\n\n==}")
	containsInOrder(t, body,
		`<div class="m2h-critic-block m2h-critic-block-highlight">`,
		"<p>段落内容</p>",
		"</div>")
}

func TestCriticBlockInsertKeepsList(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "{++\n\n新增：\n\n- 一\n- 二\n\n++}")
	containsInOrder(t, body,
		`<div class="m2h-critic-block m2h-critic-block-insert">`,
		"<ul>",
		"<li>一</li>",
		"<li>二</li>",
		"</ul>",
		"</div>")
}

func TestCriticBlockDeleteKeepsInlineMarkup(t *testing.T) {
	t.Parallel()

	body := renderBody(t, "{--\n\n**重要说明**\n\n--}")
	containsInOrder(t, body,
		`<div class="m2h-critic-block m2h-critic-block-delete">`,
		"<strong>重要说明</strong>",
		"</div>")
}

func TestCriticBlockOpenerMustBeStandalone(t *testing.T) {
	t.Parallel()

	// An opener followed by text on the same line is inline critic, not a block.
	body := renderBody(t, "{==内联==}")
	if strings.Contains(body, "m2h-critic-block") {
		t.Errorf("inline critic produced a block:\n%s", body)
	}
	if !strings.Contains(body, `<mark class="m2h-critic">内联</mark>`) {
		t.Errorf("standalone opener check misclassified inline critic:\n%s", body)
	}
}
