package markdown

import (
	"slices"
	"testing"
)

func TestInspectCollectsHeadings(t *testing.T) {
	t.Parallel()

	source := []byte("# Hello World\n# Hello World\n## 中文 标题\n")
	inspection := Inspect(source)

	ids := make([]string, 0, len(inspection.Headings))
	for _, heading := range inspection.Headings {
		ids = append(ids, heading.ID)
	}
	// Duplicate headings get GitHub-style suffixes and CJK characters survive
	// the slug, so anchors match what Render emits for the WebUI.
	want := []string{"hello-world", "hello-world-1", "中文-标题"}
	if !slices.Equal(ids, want) {
		t.Fatalf("heading ids = %v, want %v", ids, want)
	}
	if inspection.H1Count != 2 {
		t.Fatalf("H1Count = %d, want 2", inspection.H1Count)
	}
}

func TestInspectCountsSetextHeadingAsH1(t *testing.T) {
	t.Parallel()

	inspection := Inspect([]byte("Title\n=====\n\n## Section\n"))
	if inspection.H1Count != 1 {
		t.Fatalf("H1Count = %d, want 1 for a setext heading", inspection.H1Count)
	}
	if len(inspection.Headings) != 2 || inspection.Headings[0].ID != "title" {
		t.Fatalf("headings = %+v, want a titled setext H1 plus the section", inspection.Headings)
	}
}

func TestInspectCountsBlockquoteH1(t *testing.T) {
	t.Parallel()

	inspection := Inspect([]byte("> # Quoted\n"))
	if inspection.H1Count != 1 || len(inspection.Headings) != 1 || inspection.Headings[0].ID != "quoted" {
		t.Fatalf("inspection = %+v, want the quoted H1 counted", inspection)
	}
}

func TestInspectCollectsMarkdownReferences(t *testing.T) {
	t.Parallel()

	source := []byte("Intro with a [link](guide.md) here.\n\n![alt text](images/logo.png)\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceLink, Destination: "guide.md", Line: 1, Column: 15},
		{Kind: ReferenceImage, Destination: "images/logo.png", Line: 3, Column: 3},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectLocatesReferenceStyleLinks(t *testing.T) {
	t.Parallel()

	source := []byte("[Guide][guide] and [collapsed][]\n\n[guide]: docs/guide.md\n[collapsed]: other.md\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceLink, Destination: "docs/guide.md", Line: 1, Column: 2},
		{Kind: ReferenceLink, Destination: "other.md", Line: 1, Column: 21},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectLocatesChildlessImagesByDestinationSearch(t *testing.T) {
	t.Parallel()

	// An image with no alt text has no child segment, so its position falls
	// back to the first literal occurrence of the destination — and repeated
	// childless images each locate their own occurrence.
	source := []byte("Some text.\n\n![](images/missing.png)\n\n![](images/missing.png)\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceImage, Destination: "images/missing.png", Line: 3, Column: 5},
		{Kind: ReferenceImage, Destination: "images/missing.png", Line: 5, Column: 5},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectLocatesNestedImageInsideLink(t *testing.T) {
	t.Parallel()

	source := []byte("[![badge](badge.svg)](target.md)\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceLink, Destination: "target.md", Line: 1, Column: 4},
		{Kind: ReferenceImage, Destination: "badge.svg", Line: 1, Column: 4},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectKeepsEmptyDestinations(t *testing.T) {
	t.Parallel()

	source := []byte("[empty]() and ![]()\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceLink, Destination: "", Line: 1, Column: 2},
		{Kind: ReferenceImage, Destination: "", Line: 1, Column: 15},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectCollectsRawHTMLBlockURLs(t *testing.T) {
	t.Parallel()

	source := []byte("<a href=\"docs/guide.md\">Guide</a>\n\n<img src=\"images/logo.png\">\n\n<video poster=\"images/poster.jpg\"></video>\n<object data=\"files/spec.pdf\"></object>\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceRawHTML, Destination: "docs/guide.md", Line: 1, Column: 1},
		{Kind: ReferenceRawHTML, Destination: "images/logo.png", Line: 3, Column: 1},
		{Kind: ReferenceRawHTML, Destination: "images/poster.jpg", Line: 5, Column: 1},
		{Kind: ReferenceRawHTML, Destination: "files/spec.pdf", Line: 6, Column: 1},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectCollectsInlineRawHTMLURLs(t *testing.T) {
	t.Parallel()

	inspection := Inspect([]byte("Text with <img src=\"images/logo.png\"> inline.\n"))
	want := []Reference{
		{Kind: ReferenceRawHTML, Destination: "images/logo.png", Line: 1, Column: 11},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectIgnoresNonURLRawHTMLAttributes(t *testing.T) {
	t.Parallel()

	inspection := Inspect([]byte("<div id=\"x\" class=\"y\" title=\"z.md\" alt=\"a.png\">keep</div>\n"))
	if len(inspection.References) != 0 {
		t.Fatalf("references = %+v, want none from non-URL attributes", inspection.References)
	}
}

func TestInspectKeepsSchemeURLs(t *testing.T) {
	t.Parallel()

	// Autolinks (<https://…>) become ast.AutoLink nodes, not ast.Link, and
	// always carry a scheme — reference collection deliberately ignores them
	// because check only examines relative local destinations.
	source := []byte("[site](https://example.com) and [mail](mailto:a@b.c) and [cdn](//cdn.example.com/a.png)\n")
	inspection := Inspect(source)

	want := []Reference{
		{Kind: ReferenceLink, Destination: "https://example.com", Line: 1, Column: 2},
		{Kind: ReferenceLink, Destination: "mailto:a@b.c", Line: 1, Column: 34},
		{Kind: ReferenceLink, Destination: "//cdn.example.com/a.png", Line: 1, Column: 59},
	}
	if !slices.Equal(inspection.References, want) {
		t.Fatalf("references = %+v, want %+v", inspection.References, want)
	}
}

func TestInspectIgnoresCodeContent(t *testing.T) {
	t.Parallel()

	source := []byte("Inline `![not image](missing.png)` and `[link](missing.md)`.\n\n```markdown\n![not image](missing.png)\n[link](missing.md)\n```\n")
	inspection := Inspect(source)

	if len(inspection.References) != 0 {
		t.Fatalf("references = %+v, want none from code spans and fenced blocks", inspection.References)
	}
	if len(inspection.Headings) != 0 {
		t.Fatalf("headings = %+v, want none", inspection.Headings)
	}
}

func TestInspectTableOfContentsAnchorsMatchRender(t *testing.T) {
	t.Parallel()

	source := []byte("# Title\n\n## 7. 代码\n\n### C++ API\n")
	inspection := Inspect(source)
	rendered, err := Render(source, RenderOptions{SourcePath: "doc.md"})
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if len(inspection.Headings) != len(rendered.Headings) {
		t.Fatalf("Inspect found %d headings, Render found %d", len(inspection.Headings), len(rendered.Headings))
	}
	for index, heading := range inspection.Headings {
		if heading.ID != rendered.Headings[index].ID {
			t.Fatalf("heading %d id = %q, want Render's %q", index, heading.ID, rendered.Headings[index].ID)
		}
	}
}

func TestSourceLocator(t *testing.T) {
	t.Parallel()

	source := []byte("first line\nsecond line\nthird\n")
	locator := newSourceLocator(source)

	tests := []struct {
		offset int
		line   int
		column int
	}{
		{offset: 0, line: 1, column: 1},
		{offset: 10, line: 1, column: 11},
		{offset: 11, line: 2, column: 1},
		{offset: 15, line: 2, column: 5},
		{offset: 22, line: 2, column: 12},
		{offset: 23, line: 3, column: 1},
		{offset: 1000, line: 4, column: 972},
		{offset: -5, line: 1, column: 1},
	}
	for _, test := range tests {
		line, column := locator.locate(test.offset)
		if line != test.line || column != test.column {
			t.Errorf("locate(%d) = %d:%d, want %d:%d", test.offset, line, column, test.line, test.column)
		}
	}
}
