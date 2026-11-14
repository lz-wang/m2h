package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// heavyDocumentSource builds a document dense in the constructs the
// document-integrity rules analyze: closed fences at the top level, in list
// items and in blockquotes, conflict-marker examples inside code, and Type
// 6/7 HTML blocks whose follow-up text lacks the separating blank line.
func heavyDocumentSource() string {
	var builder strings.Builder
	builder.WriteString("# Heavy\n\n")
	for index := range 150 {
		fmt.Fprintf(&builder, "```go\n// block %d\nx(%d)\n```\n\n", index, index)
		fmt.Fprintf(&builder, "- ```js\n  code(%d)\n  ```\n\n", index)
		builder.WriteString("> ```text\n> quoted\n> ```\n\n")
		fmt.Fprintf(&builder, "<div id=\"d%d\">\n  content %d\n</div>\nafter %d\n\n", index, index, index)
		builder.WriteString("```text\n<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> feature\n```\n\n")
	}
	builder.WriteString("```go\n// deliberately unclosed\nx()\n")
	return builder.String()
}

// BenchmarkCheckHeavyDocument measures a full check run over a document
// dense in fences and HTML blocks, guarding the linear-scan expectation of
// the document-integrity rules: 150 html blank-line warnings and one
// unclosed fence error.
func BenchmarkCheckHeavyDocument(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heavy.md"), []byte(heavyDocumentSource()), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		result, err := Run(context.Background(), Options{Input: root})
		if err != nil {
			b.Fatal(err)
		}
		if result.Errors != 1 || result.Warnings != 150 {
			b.Fatalf("counts = %d errors, %d warnings, want 1/150", result.Errors, result.Warnings)
		}
	}
}
