package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

// benchmarkDocument is one synthetic workspace document: real frontmatter plus
// a body long enough that reading and parsing it has measurable cost, so the
// benchmarks can prove /api/files no longer scales with Markdown content.
func benchmarkDocument(index int) string {
	var body strings.Builder
	fmt.Fprintf(&body, "---\ntitle: Benchmark document %04d\n", index)
	body.WriteString("description: 一段用于性能基准的文档描述，验证文件列表不再读取正文。\n")
	body.WriteString("tags:\n  - benchmark\n---\n")
	fmt.Fprintf(&body, "# Benchmark document %04d\n\n", index)
	for range 16 {
		body.WriteString("段落：这一段正文用于模拟真实文档库中的内容长度，包含中英文混合文本 mixed English text 以及少量标点。性能基准需要足够长的正文，才能证明文件列表接口的成本不再随所有 Markdown 的正文总大小增长。\n\n")
	}
	return body.String()
}

// benchmarkWorkspace writes count synthetic documents under per-group
// directories (group-NNN/doc-NNNN.md, 25 documents per group) so the layout
// mirrors a real library with directories instead of one flat directory.
func benchmarkWorkspace(b *testing.B, count int) (http.Handler, string) {
	b.Helper()
	root := b.TempDir()
	const perGroup = 25
	for index := range count {
		group := index / perGroup
		directory := filepath.Join(root, fmt.Sprintf("group-%03d", group))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			b.Fatal(err)
		}
		name := filepath.Join(directory, fmt.Sprintf("doc-%04d.md", index))
		if err := os.WriteFile(name, []byte(benchmarkDocument(index)), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	canonical, err := files.Resolve(root)
	if err != nil {
		b.Fatal(err)
	}
	options := files.DiscoverOptions{Depth: 4, Pattern: "**/*.md"}
	handler := newDocumentHandler(
		singleRootWorkspace(rootScope{root: canonical.Path, discovery: options}),
		nil,
		directoryTestUI(),
	)
	return handler, fmt.Sprintf("group-%03d/doc-%04d.md", 0, 0)
}

func benchmarkServeFiles(b *testing.B, count int) {
	b.Helper()
	handler, _ := benchmarkWorkspace(b, count)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := performRequest(handler, http.MethodGet, "/api/files")
		if response.Code != http.StatusOK {
			b.Fatalf("GET /api/files status = %d", response.Code)
		}
	}
}

func BenchmarkServeFiles2K(b *testing.B) {
	benchmarkServeFiles(b, 2000)
}

func BenchmarkServeFiles10K(b *testing.B) {
	benchmarkServeFiles(b, 10000)
}

func benchmarkServePath(b *testing.B, target string) {
	b.Helper()
	handler, documentPath := benchmarkWorkspace(b, 2000)
	request := strings.ReplaceAll(target, "{path}", documentPath)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := performRequest(handler, http.MethodGet, request)
		if response.Code != http.StatusOK {
			b.Fatalf("GET %s status = %d", target, response.Code)
		}
	}
}

func BenchmarkServeFileMetadata(b *testing.B) {
	benchmarkServePath(b, "/api/file-metadata?path={path}")
}

func BenchmarkServeDocument(b *testing.B) {
	benchmarkServePath(b, "/api/document?path={path}")
}
