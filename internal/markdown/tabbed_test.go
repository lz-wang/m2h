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
		"=== missing\n\n    body\n", "=== \"\"\n\n    body\n", "===\"title\"\n\n    body\n", "=== \"title\" trailing\n\n    body\n",
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
