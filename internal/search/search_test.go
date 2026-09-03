package search

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lz-wang/m2h/internal/markdown"
)

func headingChunk(id, text string) markdown.SearchChunk {
	return markdown.SearchChunk{Kind: markdown.SearchChunkHeading, Text: text, HeadingID: id, HeadingText: text}
}

func textChunk(text, headingID, headingText string) markdown.SearchChunk {
	return markdown.SearchChunk{Kind: markdown.SearchChunkText, Text: text, HeadingID: headingID, HeadingText: headingText}
}

func codeChunk(text, headingID, headingText string) markdown.SearchChunk {
	return markdown.SearchChunk{Kind: markdown.SearchChunkCode, Text: text, HeadingID: headingID, HeadingText: headingText}
}

func TestMatchQueryRules(t *testing.T) {
	t.Parallel()

	document := Document{
		Path:  "docs/markdown.md",
		Title: "Markdown Rendering",
		Tags:  []string{"渲染"},
		Chunks: []markdown.SearchChunk{
			headingChunk("parser", "Parser"),
			textChunk("解析通过 Goldmark AST 完成", "parser", "Parser"),
		},
	}

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "single token in body", query: "goldmark", want: true},
		{name: "case-insensitive", query: "GOLDMARK", want: true},
		{name: "tokens distributed across title and body", query: "rendering goldmark", want: true},
		{name: "tokens distributed across path and body", query: "docs goldmark", want: true},
		{name: "AND requires every token", query: "goldmark missing-token", want: false},
		{name: "chinese substring", query: "解析通过", want: true},
		{name: "empty query", query: "", want: false},
		{name: "blank query", query: "   \t ", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, got := Match(document, test.query)
			if got != test.want {
				t.Errorf("Match(%q) matched = %v, want %v", test.query, got, test.want)
			}
		})
	}
}

// The weight order is the product contract; the test pins relative order,
// never the concrete numbers, so the constants can be retuned freely.
func TestMatchFieldRankingOrder(t *testing.T) {
	t.Parallel()

	documents := []Document{
		{Path: "a-title.md", Title: "goldmark 解析指南"},
		{Path: "b-title-contains.md", Title: "关于 goldmark 的说明"},
		{Path: "goldmark.md", Title: "与查询无关的标题"},
		{Path: "c-tag.md", Title: "与查询无关的标题", Tags: []string{"goldmark"}},
		{Path: "d-description.md", Title: "与查询无关的标题", Description: "本文讨论 goldmark"},
		{Path: "e-heading.md", Title: "与查询无关的标题", Chunks: []markdown.SearchChunk{
			headingChunk("goldmark", "goldmark 指南"),
		}},
		{Path: "f-body.md", Title: "与查询无关的标题", Chunks: []markdown.SearchChunk{
			textChunk("正文提到 goldmark", "", ""),
		}},
		{Path: "g-code.md", Title: "与查询无关的标题", Chunks: []markdown.SearchChunk{
			codeChunk("use goldmark here()", "", ""),
		}},
	}

	wantOrder := []string{
		"a-title.md",
		"b-title-contains.md",
		"goldmark.md",
		"c-tag.md",
		"d-description.md",
		"e-heading.md",
		"f-body.md",
		"g-code.md",
	}

	results := make([]Result, 0, len(documents))
	for _, document := range documents {
		result, matched := Match(document, "goldmark")
		if !matched {
			t.Fatalf("document %s should match", document.Path)
		}
		results = append(results, result)
	}
	SortResults(results)

	got := make([]string, 0, len(results))
	for _, result := range results {
		got = append(got, result.Path)
	}
	if strings.Join(got, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("rank order = %v, want %v", got, wantOrder)
	}
}

func TestMatchBestSectionOnly(t *testing.T) {
	t.Parallel()

	document := Document{
		Path:  "README.md",
		Title: "README",
		Chunks: []markdown.SearchChunk{
			headingChunk("backend", "Backend"),
			textChunk("backend 正文 unique-backend", "backend", "Backend"),
			textChunk("backend 另一段 unique-backend", "backend", "Backend"),
			headingChunk("frontend", "Frontend"),
			textChunk("frontend 正文", "frontend", "Frontend"),
		},
	}

	result, matched := Match(document, "unique-backend")
	if !matched {
		t.Fatal("expected a match")
	}
	if result.HeadingID != "backend" || result.HeadingText != "Backend" {
		t.Errorf("section = %q/%q, want backend/Backend", result.HeadingID, result.HeadingText)
	}
	// Two matches in one section, one result: the excerpt comes from the
	// best chunk, the document never floods the result list per section.
	if !strings.Contains(result.Snippet, "unique-backend") {
		t.Errorf("snippet = %q, want it around the match", result.Snippet)
	}
}

// A heading match outranks body matches in other sections, so the reported
// section is the one whose own heading matched.
func TestMatchSectionPrefersHeadingOverDistantBody(t *testing.T) {
	t.Parallel()

	document := Document{
		Path:  "README.md",
		Title: "README",
		Chunks: []markdown.SearchChunk{
			headingChunk("install", "Install"),
			textChunk("先看 install 之外的内容 alpha", "", ""),
			headingChunk("alpha", "Alpha"),
			textChunk("普通段落", "alpha", "Alpha"),
		},
	}

	result, matched := Match(document, "alpha")
	if !matched {
		t.Fatal("expected a match")
	}
	if result.HeadingID != "alpha" {
		t.Errorf("section = %q, want alpha", result.HeadingID)
	}
}

func TestMatchHeadingResultFields(t *testing.T) {
	t.Parallel()

	document := Document{
		Path:  "guide.md",
		Title: "Guide",
		Chunks: []markdown.SearchChunk{
			headingChunk("安装指南", "安装指南"),
			textChunk("运行安装命令", "安装指南", "安装指南"),
		},
	}

	result, matched := Match(document, "安装指南")
	if !matched {
		t.Fatal("expected a match")
	}
	if result.HeadingID != "安装指南" || result.HeadingText != "安装指南" {
		t.Errorf("section = %q/%q, want 安装指南/安装指南", result.HeadingID, result.HeadingText)
	}

	bodyResult, matched := Match(document, "安装命令")
	if !matched {
		t.Fatal("expected a body match")
	}
	if bodyResult.HeadingID != "安装指南" {
		t.Errorf("body match section = %q, want 安装指南", bodyResult.HeadingID)
	}
}

func TestMatchCodeResult(t *testing.T) {
	t.Parallel()

	document := Document{
		Path:  "api.md",
		Title: "API",
		Chunks: []markdown.SearchChunk{
			textChunk("调用示例", "", ""),
			codeChunk("func SearchDocument() {}", "", ""),
		},
	}

	result, matched := Match(document, "SearchDocument")
	if !matched {
		t.Fatal("expected a match")
	}
	if !strings.Contains(result.Snippet, "func SearchDocument() {}") {
		t.Errorf("snippet = %q, want the code source", result.Snippet)
	}
	if result.HeadingID != "" || result.HeadingText != "" {
		t.Errorf("pre-heading content carries a section: %q/%q", result.HeadingID, result.HeadingText)
	}
}

func TestMatchMetadataOnly(t *testing.T) {
	t.Parallel()

	t.Run("description match yields description snippet", func(t *testing.T) {
		t.Parallel()
		document := Document{
			Path:        "a.md",
			Title:       "A",
			Description: "这是一段文档摘要描述",
		}
		result, matched := Match(document, "摘要")
		if !matched {
			t.Fatal("expected a match")
		}
		if result.Snippet != "这是一段文档摘要描述" {
			t.Errorf("snippet = %q, want the description", result.Snippet)
		}
		if result.HeadingID != "" || result.HeadingText != "" {
			t.Errorf("metadata-only match carries a section: %q/%q", result.HeadingID, result.HeadingText)
		}
	})

	t.Run("title, path and tag matches leave the snippet empty", func(t *testing.T) {
		t.Parallel()
		document := Document{
			Path:  "note-release.md",
			Title: "Release",
			Tags:  []string{"release"},
		}
		result, matched := Match(document, "release")
		if !matched {
			t.Fatal("expected a match")
		}
		if result.Snippet != "" {
			t.Errorf("snippet = %q, want empty", result.Snippet)
		}
	})
}

func TestMatchSnippetShaping(t *testing.T) {
	t.Parallel()

	t.Run("short text returns verbatim", func(t *testing.T) {
		t.Parallel()
		document := Document{Path: "a.md", Title: "A", Chunks: []markdown.SearchChunk{
			textChunk("短文本 needle", "", ""),
		}}
		result, _ := Match(document, "needle")
		if result.Snippet != "短文本 needle" {
			t.Errorf("snippet = %q", result.Snippet)
		}
	})

	t.Run("long text is excerpted around the match with ellipses", func(t *testing.T) {
		t.Parallel()
		// Both sides long enough that the 200-rune window cannot reach
		// either end of the chunk.
		prefix := strings.Repeat("前", 150)
		suffix := strings.Repeat("后", 300)
		document := Document{Path: "a.md", Title: "A", Chunks: []markdown.SearchChunk{
			textChunk(prefix+" needle "+suffix, "", ""),
		}}
		result, _ := Match(document, "needle")
		snippet := result.Snippet
		if !strings.HasPrefix(snippet, "…") || !strings.HasSuffix(snippet, "…") {
			t.Errorf("snippet = %q, want leading and trailing ellipses", snippet)
		}
		if !strings.Contains(snippet, "needle") {
			t.Errorf("snippet = %q, want the match inside", snippet)
		}
		// The two ellipses plus the excerpt stay within the cap (in runes).
		if got := len([]rune(snippet)) - 2; got > maxSnippetRunes {
			t.Errorf("snippet body = %d runes, want ≤ %d", got, maxSnippetRunes)
		}
	})

	t.Run("cut never splits a rune", func(t *testing.T) {
		t.Parallel()
		// A long CJK body where the excerpt window boundary lands inside
		// multibyte content; a byte-slice cut would produce invalid UTF-8.
		body := strings.Repeat("汉字", 300) + " needle " + strings.Repeat("词组", 300)
		document := Document{Path: "a.md", Title: "A", Chunks: []markdown.SearchChunk{
			textChunk(body, "", ""),
		}}
		result, _ := Match(document, "needle")
		if !utf8.ValidString(result.Snippet) {
			t.Errorf("snippet is not valid UTF-8: %q", result.Snippet)
		}
		if !strings.Contains(result.Snippet, "needle") {
			t.Errorf("snippet = %q, want the match inside", result.Snippet)
		}
	})
}

func TestSortResultsTieBreakByPath(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Path: "z.md", score: 300},
		{Path: "a.md", score: 300},
		{Path: "m.md", score: 900},
	}
	SortResults(results)

	want := []string{"m.md", "a.md", "z.md"}
	got := make([]string, 0, len(results))
	for _, result := range results {
		got = append(got, result.Path)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// One document, one result: even when the query hits several sections, the
// result carries only the best section.
func TestMatchSingleResultPerDocument(t *testing.T) {
	t.Parallel()

	document := Document{
		Path:  "README.md",
		Title: "README",
		Chunks: []markdown.SearchChunk{
			headingChunk("backend", "Backend"),
			textChunk("backend 内容 token", "backend", "Backend"),
			headingChunk("frontend", "Frontend"),
			textChunk("frontend 内容 token", "frontend", "Frontend"),
			codeChunk("token in code", "", ""),
		},
	}

	result, matched := Match(document, "token")
	if !matched {
		t.Fatal("expected a match")
	}
	if result.HeadingID != "backend" {
		t.Errorf("section = %q, want backend (the strongest section)", result.HeadingID)
	}
}
