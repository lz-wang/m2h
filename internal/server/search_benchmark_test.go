package server

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/lz-wang/m2h/internal/files"
)

// The search benchmarks measure one query against a synthetic library built
// by benchmarkWorkspace. Every query shape targets content that generator is
// known to produce, so each benchmark stresses a distinct match path:
//
//	title hit        "document 0042"  one document, title and H1 heading
//	body hit         "中英文混合"       every document's prose chunks
//	miss             "missing-token…"  full scan with zero matches
//	multi keyword    "benchmark 混合文本" AND across title and body
//	cjk              "性能基准"         substring search without segmentation
//	high frequency   "benchmark"       title, tag and heading on every document
//
// Sizes mirror the /api/files benchmarks: 2,000 documents for a large real
// library and 10,000 for the hostile end of the range.
//
// benchmarkSearch measures the steady state: the handler is warmed by one
// request before the timer starts, so per-request setup that any caller would
// amortize (index construction, file discovery) stays out of the numbers.
func benchmarkSearch(b *testing.B, count int, query string) {
	b.Helper()
	handler, _, _ := benchmarkWorkspace(b, count)
	target := "/api/search?q=" + url.QueryEscape(query)
	warmup := performRequest(handler, http.MethodGet, target)
	if warmup.Code != http.StatusOK {
		b.Fatalf("GET %s warmup status = %d, body = %s", target, warmup.Code, warmup.Body.String())
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		response := performRequest(handler, http.MethodGet, target)
		if response.Code != http.StatusOK {
			b.Fatalf("GET %s status = %d, body = %s", target, response.Code, response.Body.String())
		}
	}
}

// benchmarkSearchColdStart measures the first query a fresh handler answers.
// Files are written once outside the loop; each iteration mounts a new
// handler over the same tree and pays whatever the current implementation
// charges before it can answer — today the full workspace scan, after an
// index exists, the initial build (discovery, read, parse) plus one query.
func benchmarkSearchColdStart(b *testing.B, count int) {
	b.Helper()
	_, root, _ := benchmarkWorkspace(b, count)
	target := "/api/search?q=" + url.QueryEscape("document 0042")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		handler := newDocumentHandler(
			singleRootWorkspace(rootScope{root: root, discovery: files.DiscoverOptions{Depth: 4, Pattern: "**/*.md"}}),
			nil,
			directoryTestUI(),
		)
		response := performRequest(handler, http.MethodGet, target)
		if response.Code != http.StatusOK {
			b.Fatalf("GET %s status = %d, body = %s", target, response.Code, response.Body.String())
		}
	}
}

func BenchmarkSearchTitleHit2K(b *testing.B)      { benchmarkSearch(b, 2000, "document 0042") }
func BenchmarkSearchTitleHit10K(b *testing.B)     { benchmarkSearch(b, 10000, "document 0042") }
func BenchmarkSearchBodyHit2K(b *testing.B)       { benchmarkSearch(b, 2000, "中英文混合") }
func BenchmarkSearchBodyHit10K(b *testing.B)      { benchmarkSearch(b, 10000, "中英文混合") }
func BenchmarkSearchMiss2K(b *testing.B)          { benchmarkSearch(b, 2000, "missing-token-zzz") }
func BenchmarkSearchMiss10K(b *testing.B)         { benchmarkSearch(b, 10000, "missing-token-zzz") }
func BenchmarkSearchMultiKeyword2K(b *testing.B)  { benchmarkSearch(b, 2000, "benchmark 混合文本") }
func BenchmarkSearchMultiKeyword10K(b *testing.B) { benchmarkSearch(b, 10000, "benchmark 混合文本") }
func BenchmarkSearchCJK2K(b *testing.B)           { benchmarkSearch(b, 2000, "性能基准") }
func BenchmarkSearchCJK10K(b *testing.B)          { benchmarkSearch(b, 10000, "性能基准") }
func BenchmarkSearchHighFrequency2K(b *testing.B) { benchmarkSearch(b, 2000, "benchmark") }
func BenchmarkSearchHighFrequency10K(b *testing.B) {
	benchmarkSearch(b, 10000, "benchmark")
}

func BenchmarkSearchColdStart2K(b *testing.B)  { benchmarkSearchColdStart(b, 2000) }
func BenchmarkSearchColdStart10K(b *testing.B) { benchmarkSearchColdStart(b, 10000) }
