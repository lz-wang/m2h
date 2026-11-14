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

// BenchmarkCheckFenceScaling measures full check runs as the fence count
// grows 100 → 500 → 1000, guarding the near-linear expectation of the
// closer analysis: the candidate close of every fence is checked against the
// occupied offsets, so a rescan-per-fence would show superlinear growth
// across the sub-benchmarks. Ratios between sizes, not absolute times,
// carry the signal.
func BenchmarkCheckFenceScaling(b *testing.B) {
	for _, count := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("fences-%d", count), func(b *testing.B) {
			blocks := make([]string, 0, count)
			for index := range count {
				blocks = append(blocks, fmt.Sprintf("```go\n// block %d\nx(%d)\n```", index, index))
			}
			// Joined with single blank separators and one final newline so
			// the fixture itself stays free of trailing-blank findings.
			document := "# Scaling\n\n" + strings.Join(blocks, "\n\n") + "\n"
			root := b.TempDir()
			if err := os.WriteFile(filepath.Join(root, "scaling.md"), []byte(document), 0o644); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for b.Loop() {
				result, err := Run(context.Background(), Options{Input: root})
				if err != nil {
					b.Fatal(err)
				}
				if result.Errors != 0 || result.Warnings != 0 {
					b.Fatalf("counts = %d errors, %d warnings, want 0/0", result.Errors, result.Warnings)
				}
			}
		})
	}
}
