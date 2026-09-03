package markdown

import (
	"reflect"
	"strings"
	"testing"
)

func projectForSearchSource(t *testing.T, source string) SearchProjection {
	t.Helper()
	return projectForSearchPath(t, source, "doc.md")
}

func projectForSearchPath(t *testing.T, source, sourcePath string) SearchProjection {
	t.Helper()
	projection, err := ProjectForSearch([]byte(source), sourcePath)
	if err != nil {
		t.Fatalf("ProjectForSearch() returned error: %v", err)
	}
	return projection
}

func TestProjectForSearchTitleFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		path   string
		want   string
	}{
		{
			name:   "first H1 becomes the title",
			source: "# 指南标题\n\n正文",
			path:   "doc.md",
			want:   "指南标题",
		},
		{
			name:   "no H1 falls back to the file name",
			source: "## 只有二级标题\n\n正文",
			path:   "fallback-name.md",
			want:   "fallback-name.md",
		},
		{
			name:   "first H1 wins over later H1s",
			source: "# First\n\n# Second",
			path:   "doc.md",
			want:   "First",
		},
		{
			name:   "empty H1 falls back to the file name",
			source: "#\n\n正文",
			path:   "doc.md",
			want:   "doc.md",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			projection := projectForSearchPath(t, test.source, test.path)
			if projection.Title != test.want {
				t.Errorf("Title = %q, want %q", projection.Title, test.want)
			}
		})
	}
}

func TestProjectForSearchInvalidSourcePath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"", "/abs/path.md", "../escape.md"} {
		if _, err := ProjectForSearch([]byte("# t"), path); err == nil {
			t.Errorf("ProjectForSearch(%q) expected an error", path)
		}
	}
}

// Frontmatter is not the projection's concern: callers split it before
// projecting the body (invalid frontmatter keeps the whole source
// searchable, per the server contract). A source that still contains the
// block is ordinary Markdown — here the YAML line surfaces as regular
// content, and the frontmatter title is never preferred over the body H1.
func TestProjectForSearchDoesNotHandleFrontmatter(t *testing.T) {
	t.Parallel()

	projection := projectForSearchSource(t, "---\ntitle: FM 标题\n---\n\n# 正文标题\n")
	var texts []string
	for _, chunk := range projection.Chunks {
		texts = append(texts, chunk.Text)
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "title: FM 标题") {
		t.Errorf("expected the unsplit frontmatter to stay searchable: %v", texts)
	}
	if projection.Title != "正文标题" {
		t.Errorf("Title = %q, want 正文标题", projection.Title)
	}
}

// The heading ids must be byte-identical to the anchors Render produces, so
// a search hit can deep-link to the exact section.
func TestProjectForSearchHeadingIDsMatchRender(t *testing.T) {
	t.Parallel()

	source := "## 指南 安装\n\n## Guide\n\n### Guide\n\n## 7. 代码\n"

	projection := projectForSearchSource(t, source)
	rendered, err := Render([]byte(source), RenderOptions{SourcePath: "doc.md"})
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	ids := make([]string, 0, len(rendered.Headings))
	for _, chunk := range projection.Chunks {
		if chunk.Kind == SearchChunkHeading {
			ids = append(ids, chunk.HeadingID)
		}
	}
	want := make([]string, 0, len(rendered.Headings))
	for _, heading := range rendered.Headings {
		want = append(want, heading.ID)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("heading ids = %v, want %v", ids, want)
	}
	for i, heading := range rendered.Headings {
		if projection.Chunks[i].Text != heading.Text {
			t.Errorf("heading %d text = %q, want %q", i, projection.Chunks[i].Text, heading.Text)
		}
	}
}

func TestProjectForSearchDuplicateHeadingIDs(t *testing.T) {
	t.Parallel()

	projection := projectForSearchSource(t, "## Backend\n\nA\n\n## Backend\n\nB\n")

	var ids []string
	for _, chunk := range projection.Chunks {
		if chunk.Kind == SearchChunkText {
			ids = append(ids, chunk.HeadingID)
		}
	}
	want := []string{"backend", "backend-1"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("section ids = %v, want %v", ids, want)
	}
}

func TestProjectForSearchChunks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   []SearchChunk
	}{
		{
			name:   "paragraph text",
			source: "使用 Goldmark 进行解析。",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "使用 Goldmark 进行解析。"},
			},
		},
		{
			name:   "link text stays, link URL does not",
			source: "使用 [Goldmark](https://github.com/yuin/goldmark) 进行解析。",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "使用 Goldmark 进行解析。"},
			},
		},
		{
			name:   "image alt stays, image URL does not",
			source: "![系统架构](architecture.png)",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "系统架构"},
			},
		},
		{
			name:   "autolink URL is not searchable text",
			source: "访问 <https://example.com/goldmark> 了解更多",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "访问 了解更多"},
			},
		},
		{
			name:   "inline code",
			source: "调用 `SearchDocument` 完成匹配。",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "调用 SearchDocument 完成匹配。"},
			},
		},
		{
			name:   "inline raw HTML markup is dropped",
			source: "一种 <b>加粗</b> 的写法",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "一种 加粗 的写法"},
			},
		},
		{
			name:   "raw HTML block is not indexed",
			source: "<div class=\"widget\">widget 内文字</div>",
			want:   []SearchChunk{},
		},
		{
			name:   "fenced code keeps its source",
			source: "```go\nfunc SearchDocument() {}\n```",
			want: []SearchChunk{
				{Kind: SearchChunkCode, Text: "func SearchDocument() {}"},
			},
		},
		{
			name:   "indented code",
			source: "正文：\n\n    indented_code_line()",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "正文："},
				{Kind: SearchChunkCode, Text: "indented_code_line()"},
			},
		},
		{
			name:   "tight list items",
			source: "- 第一项\n- 第二项\n",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "第一项"},
				{Kind: SearchChunkText, Text: "第二项"},
			},
		},
		{
			name:   "loose list paragraphs",
			source: "- 第一项\n\n- 第二项\n",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "第一项"},
				{Kind: SearchChunkText, Text: "第二项"},
			},
		},
		{
			name:   "list item with code",
			source: "- 列表说明\n\n  ```go\n  code_in_list()\n  ```\n",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "列表说明"},
				{Kind: SearchChunkCode, Text: "code_in_list()"},
			},
		},
		{
			name:   "blockquote",
			source: "> 引用文字",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "引用文字"},
			},
		},
		{
			name:   "table rows become text chunks",
			source: "| 引擎 | 用途 |\n| --- | --- |\n| Goldmark | 解析 |\n",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "引擎 用途"},
				{Kind: SearchChunkText, Text: "Goldmark 解析"},
			},
		},
		{
			name:   "task list markers leave no text",
			source: "- [ ] 未完成事项\n- [x] 已完成事项\n",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "未完成事项"},
				{Kind: SearchChunkText, Text: "已完成事项"},
			},
		},
		{
			name:   "emoji is searchable",
			source: "发布 :rocket: 版本",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "发布 🚀 版本"},
			},
		},
		{
			name:   "strikethrough and highlights",
			source: "~~移除~~ ==标记== 文本",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "移除 标记 文本"},
			},
		},
		{
			name:   "mermaid and vega sources are searchable",
			source: "```mermaid\ngraph TD\n  A-->B\n```\n\n```vega-lite\n{\"mark\": \"bar\"}\n```\n",
			want: []SearchChunk{
				{Kind: SearchChunkCode, Text: "graph TD\n  A-->B"},
				{Kind: SearchChunkCode, Text: "{\"mark\": \"bar\"}"},
			},
		},
		{
			name:   "footnote definition text stays searchable",
			source: "正文[^1]\n\n[^1]: 脚注内容",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "正文"},
				{Kind: SearchChunkText, Text: "脚注内容"},
			},
		},
		{
			name:   "HTML entities resolve like rendered text",
			source: "a &amp; b",
			want: []SearchChunk{
				{Kind: SearchChunkText, Text: "a & b"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			projection := projectForSearchSource(t, test.source)
			if !reflect.DeepEqual(projection.Chunks, test.want) {
				t.Errorf("chunks mismatch:\n got %+v\nwant %+v", projection.Chunks, test.want)
			}
			for _, chunk := range projection.Chunks {
				if chunk.HeadingID != "" || chunk.HeadingText != "" {
					t.Errorf("chunk before any heading carries section %q/%q", chunk.HeadingID, chunk.HeadingText)
				}
			}
		})
	}
}

func TestProjectForSearchSectionAttribution(t *testing.T) {
	t.Parallel()

	projection := projectForSearchSource(t, "## Backend\n\nA\n\nB\n\n```go\ncode()\n```\n\n### Inner\n\nC\n")

	want := []SearchChunk{
		{Kind: SearchChunkHeading, Text: "Backend", HeadingID: "backend", HeadingText: "Backend"},
		{Kind: SearchChunkText, Text: "A", HeadingID: "backend", HeadingText: "Backend"},
		{Kind: SearchChunkText, Text: "B", HeadingID: "backend", HeadingText: "Backend"},
		{Kind: SearchChunkCode, Text: "code()", HeadingID: "backend", HeadingText: "Backend"},
		{Kind: SearchChunkHeading, Text: "Inner", HeadingID: "inner", HeadingText: "Inner"},
		{Kind: SearchChunkText, Text: "C", HeadingID: "inner", HeadingText: "Inner"},
	}
	if !reflect.DeepEqual(projection.Chunks, want) {
		t.Errorf("chunks mismatch:\n got %+v\nwant %+v", projection.Chunks, want)
	}
}

func TestProjectForSearchChineseHeadingSection(t *testing.T) {
	t.Parallel()

	projection := projectForSearchSource(t, "## 安装指南\n\n运行安装命令。\n")

	if len(projection.Chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(projection.Chunks))
	}
	heading, body := projection.Chunks[0], projection.Chunks[1]
	if heading.HeadingID != "安装指南" {
		t.Errorf("heading id = %q, want 安装指南", heading.HeadingID)
	}
	if body.HeadingID != heading.HeadingID || body.HeadingText != heading.Text {
		t.Errorf("body section = %q/%q, want %q/%q", body.HeadingID, body.HeadingText, heading.HeadingID, heading.Text)
	}
}

// Chunks always carry a non-nil slice so range walks and appends behave
// uniformly for callers.
func TestProjectForSearchEmptyDocument(t *testing.T) {
	t.Parallel()

	projection := projectForSearchSource(t, "")
	if projection.Chunks == nil || len(projection.Chunks) != 0 {
		t.Errorf("chunks = %#v, want an empty non-nil slice", projection.Chunks)
	}
	if projection.Title != "doc.md" {
		t.Errorf("Title = %q, want doc.md", projection.Title)
	}
}
