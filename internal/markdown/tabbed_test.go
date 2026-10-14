package markdown

import (
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
