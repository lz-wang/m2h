package markdown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func parseTabs(source string) ast.Node {
	return newEngine().Parser().Parse(text.NewReader([]byte(source)), parser.WithContext(parser.NewContext(parser.WithIDs(newGitHubIDs()))))
}

func TestTabbedParser(t *testing.T) {
	source := "=== \"Linux\"\n\n    ## 安装\n\n    - one\n    - two\n\n===+ \"macOS\"\n\n    ## 安装\n\n    ```go\n    === \"literal\"\n    ```\n\n===+ \"Third\"\n\n    body\n\n===! \"New\"\n\n    | A | B |\n    | - | - |\n    | 1 | 2 |\n"
	doc := parseTabs(source)
	first, ok := doc.FirstChild().(*TabSet)
	if !ok || first.ChildCount() != 3 || first.Selected != 1 {
		t.Fatalf("first set: %#v", doc.FirstChild())
	}
	second, ok := first.NextSibling().(*TabSet)
	if !ok || second.ChildCount() != 1 || second.Selected != 0 {
		t.Fatalf("second set: %#v", first.NextSibling())
	}
	if first.FirstChild().(*TabItem).ID != "m2h-tab-1-1" || second.FirstChild().(*TabItem).ID != "m2h-tab-2-1" {
		t.Fatal("unstable IDs")
	}
	counts := map[ast.NodeKind]int{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			counts[n.Kind()]++
		}
		return ast.WalkContinue, nil
	})
	if counts[ast.KindHeading] != 2 || counts[ast.KindFencedCodeBlock] != 1 || counts[ast.KindList] != 1 {
		t.Fatalf("children: %v", counts)
	}
}

func TestTabbedParserBoundaries(t *testing.T) {
	for _, source := range []string{
		"=== missing\n\n    body\n", "=== + \"spaced modifier\"\n\n    body\n", "=== \"\"\n\n    body\n", "===\"title\"\n\n    body\n", "=== \"title\" trailing\n\n    body\n",
		"    === \"code\"\n", "```\n=== \"code\"\n```\n", "- list\n\n    === \"nested\"\n\n        body\n", "> === \"quote\"\n>\n>     body\n",
	} {
		t.Run(source, func(t *testing.T) {
			doc := parseTabs(source)
			_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if entering && n.Kind() == kindTabSet {
					t.Fatal("unexpected tab")
				}
				return ast.WalkContinue, nil
			})
		})
	}
	for _, source := range []string{"=== \"Empty\"\n", "   === \"Indent\"\n\n       paragraph\n", "=== \"Tabs\"\n\n\tparagraph\n"} {
		if _, ok := parseTabs(source).FirstChild().(*TabSet); !ok {
			t.Fatalf("not recognized: %q", source)
		}
	}
}

func TestTabbedHTML(t *testing.T) {
	source := "=== \"<script>& label\"\n\n    first\n\n===+ \"Second\"\n\n    second\n\n===+ \"Third\"\n\n    third\n"
	result, err := Render([]byte(source), RenderOptions{SourcePath: "tabs.md"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`role="group"`, `id="m2h-tab-1-2" checked`, `data-m2h-tab-id="m2h-tab-1-3"`, `&lt;script&gt;&amp; label`, `for="m2h-tab-1-1"`} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("missing %q: %s", want, result.Body)
		}
	}
	if strings.Count(result.Body, " checked") != 1 || strings.Contains(result.Body, `role="tab"`) {
		t.Fatal("radio semantics")
	}
	var many strings.Builder
	for i := 0; i < 33; i++ {
		fmt.Fprintf(&many, "=== \"Tab %d\"\n\n    content%d\n\n", i, i)
	}
	result, err = Render([]byte(many.String()), RenderOptions{SourcePath: "many.md"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Body, `type="radio"`) || !strings.Contains(result.Body, "m2h-tabbed-stacked") || strings.Count(result.Body, `<section class="m2h-tabbed-panel"`) != 33 || !strings.Contains(result.Body, "content32") {
		t.Fatal("overflow lost content")
	}
}

func TestTabbedInspectionContracts(t *testing.T) {
	source := []byte("# Guide\n\n=== \"One\"\n\n    ## 安装\n\n    [missing](missing.md)\n\n    ![picture](image.png)\n\n    ```text\n    [ignored](ignored.md)\n    === \"literal\"\n    ```\n\n    $x^2$\n\n=== \"Two\"\n\n    ## 安装\n\n    [same](#安装)\n")
	result, err := Render(source, RenderOptions{URLMode: URLWeb, SourcePath: "docs/tabs.md"})
	if err != nil {
		t.Fatal(err)
	}
	facts := Inspect(source)
	if len(facts.Headings) != 3 || facts.Headings[1].ID != "安装" || facts.Headings[2].ID != "安装-1" || facts.Headings[1].Line != 5 || facts.Headings[2].Line != 20 {
		t.Fatalf("headings: %+v", facts.Headings)
	}
	for i, h := range facts.Headings {
		if result.Headings[i].ID != h.ID || !strings.Contains(result.Body, `id="`+h.ID+`"`) {
			t.Fatal("render/inspect anchors differ")
		}
	}
	if len(facts.References) != 3 || facts.References[0].Line != 7 || facts.References[0].Destination != "missing.md" || facts.References[1].Line != 9 {
		t.Fatalf("references: %+v", facts.References)
	}
	for _, want := range []string{`href="/doc/docs/missing.md"`, `src="/assets/docs/image.png"`, `class="m2h-math"`, `x^2`} {
		if !strings.Contains(result.Body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if len(facts.CodeFences) != 1 {
		t.Fatalf("fences: %+v", facts.CodeFences)
	}
}

func TestTabbedStrictIndentAndNestedFallback(t *testing.T) {
	for _, source := range []string{"=== \"A\"\n    inside\noutside\n", "=== \"A\"\n    - inside\n      continued\noutside\n"} {
		doc := parseTabs(source)
		if doc.ChildCount() != 2 || doc.LastChild().Kind() != ast.KindParagraph {
			t.Fatalf("lazy continuation crossed tab: %s", source)
		}
	}
	for _, source := range []string{"=== \"A\"\nwrong indent\n", "=== \"A\"\n  wrong indent\n"} {
		result, err := Render([]byte(source), RenderOptions{SourcePath: "a.md"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(result.Body, "m2h-tabbed-set") || !strings.Contains(result.Body, "wrong indent") || !strings.Contains(result.Body, "===") {
			t.Fatalf("incomplete syntax lost: %s", result.Body)
		}
	}
	doc := parseTabs("=== \"Outer\"\n\n    === \"Nested\"\n\n        code\n")
	outer := doc.FirstChild().(*TabSet).FirstChild()
	if outer.FirstChild().Kind() != ast.KindParagraph {
		t.Fatal("nested marker not literal")
	}
}

func TestTabbedEmptyItemBeforeRootContent(t *testing.T) {
	doc := parseTabs("=== \"Empty\"\n\n## After\n\nOutside\n")
	if doc.ChildCount() != 3 || doc.FirstChild().Kind() != kindTabSet || doc.FirstChild().FirstChild().ChildCount() != 0 {
		t.Fatal("empty tab consumed root content")
	}
}

func TestTabbedIDsAvoidHeadingCollisions(t *testing.T) {
	source := []byte("# m2h-tab-1-1\n\n## m2h-tab-1-1-1\n\n=== \"Tab\"\n\n    unique-collision-token\n")
	rendered, err := Render(source, RenderOptions{SourcePath: "a.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.Body, `id="m2h-tab-1-1-2"`) || strings.Count(rendered.Body, `id="m2h-tab-1-1"`) != 1 {
		t.Fatalf("ambiguous native label IDs: %s", rendered.Body)
	}
	projection, err := ProjectForSearch(source, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range projection.Chunks {
		if chunk.Text == "unique-collision-token" && chunk.TabID != "m2h-tab-1-1-2" {
			t.Fatalf("projection drift: %+v", chunk)
		}
	}
}
