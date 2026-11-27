package markdown

import (
	"slices"
	"strings"
	"testing"
)

func TestRenderShareUsesOnlyRetainedLocalReferences(t *testing.T) {
	t.Parallel()
	source := []byte(`# Shared Guide

[PDF](/docs/guide.pdf#page=2) [Other](other.md) [Heading](#shared-guide)

![Picture](image%20one.png) ![Outside](../../outside.png)

<a href="https://example.com/topic">external</a>
<a href="//example.com/path">external without protocol</a>
<a href="http://user@example.com/topic">credentialed link</a>
<a href="javascript:alert(1)">blocked</a>
<a href="%ZZ">bad URL</a>

<picture><source srcset="image%20one.png 1x, data:image/png;base64,AAAA 2x, hi.png 3x"><img src="image%20one.png" onerror="bad()" alt="image"></picture>
<video controls poster="hi.png"><source src="movie.mp4"></video>

<iframe src="removed.txt"></iframe>
<form><img src="nested.txt"></form>
<script>not shared</script>
<div style="text-align: center;">Centered</div>
<div style="background: url(removed.png);">No arbitrary CSS</div>
<input type="text" autofocus>
`)
	var requested []string
	rendered, err := RenderShare(source, "sub%20dir/guide.md", func(relative string) string {
		requested = append(requested, relative)
		if relative == "sub%20dir/other.md" {
			return ""
		}
		return "/share/example/assets/" + strings.ReplaceAll(relative, " ", "%20")
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Title != "Shared Guide" || len(rendered.Headings) != 1 {
		t.Fatalf("missing heading metadata: %+v", rendered)
	}
	for _, want := range []string{
		`href="/share/example/assets/docs/guide.pdf#page=2"`,
		`href="#shared-guide"`,
		`href="https://example.com/topic"`,
		`href="//example.com/path"`,
		`srcset="/share/example/assets/sub%20dir/image%20one.png 1x, /share/example/assets/sub%20dir/hi.png 3x"`,
		`poster="/share/example/assets/sub%20dir/hi.png"`,
		`src="/share/example/assets/sub%20dir/movie.mp4"`,
		`style="text-align:center;"`, `<a>Other</a>`,
	} {
		if !strings.Contains(rendered.Body, want) {
			t.Errorf("missing %q in %s", want, rendered.Body)
		}
	}
	for _, forbidden := range []string{
		"outside.png", "example.com@",
		"javascript:", "onerror", "<iframe", "<script", "<form", "not shared",
		"background:", "<input", "data:image",
	} {
		if strings.Contains(rendered.Body, forbidden) {
			t.Errorf("retained %q in %s", forbidden, rendered.Body)
		}
	}
	for _, forbidden := range []string{"outside.png", "sub%20dir/removed.txt", "sub%20dir/nested.txt"} {
		if slices.Contains(requested, forbidden) {
			t.Errorf("asked to authorize removed reference %q", forbidden)
		}
	}
}

func TestRenderShareRetainsTabbedContentAndGFMTasks(t *testing.T) {
	t.Parallel()
	source := []byte("- [x] Checked\n\n=== \"First\"\n\n    Content.\n\n=== \"Second\"\n\n    More content.\n")
	rendered, err := RenderShare(source, "guide.md", func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`type="checkbox"`, `disabled=""`, `class="m2h-tabbed-radio"`, `data-m2h-tab-id=`, "More content."} {
		if !strings.Contains(rendered.Body, want) {
			t.Errorf("missing %q in %s", want, rendered.Body)
		}
	}
	if _, err := RenderShare(nil, "/outside.md", nil); err == nil {
		t.Fatal("accepted an absolute source path")
	}
}
