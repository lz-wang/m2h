package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

func BenchmarkSearchTitleHit2K(b *testing.B)     { benchmarkSearch(b, 2000, "document 0042") }
func BenchmarkSearchTitleHit10K(b *testing.B)    { benchmarkSearch(b, 10000, "document 0042") }
func BenchmarkSearchBodyHit2K(b *testing.B)      { benchmarkSearch(b, 2000, "中英文混合") }
func BenchmarkSearchBodyHit10K(b *testing.B)     { benchmarkSearch(b, 10000, "中英文混合") }
func BenchmarkSearchMiss2K(b *testing.B)         { benchmarkSearch(b, 2000, "missing-token-zzz") }
func BenchmarkSearchMiss10K(b *testing.B)        { benchmarkSearch(b, 10000, "missing-token-zzz") }
func BenchmarkSearchMultiKeyword2K(b *testing.B) { benchmarkSearch(b, 2000, "benchmark 混合文本") }
func BenchmarkSearchMultiKeyword10K(b *testing.B) {
	benchmarkSearch(b, 10000, "benchmark 混合文本")
}
func BenchmarkSearchCJK2K(b *testing.B)           { benchmarkSearch(b, 2000, "性能基准") }
func BenchmarkSearchCJK10K(b *testing.B)          { benchmarkSearch(b, 10000, "性能基准") }
func BenchmarkSearchHighFrequency2K(b *testing.B) { benchmarkSearch(b, 2000, "benchmark") }
func BenchmarkSearchHighFrequency10K(b *testing.B) {
	benchmarkSearch(b, 10000, "benchmark")
}

func BenchmarkSearchColdStart2K(b *testing.B)  { benchmarkSearchColdStart(b, 2000) }
func BenchmarkSearchColdStart10K(b *testing.B) { benchmarkSearchColdStart(b, 10000) }

// benchmarkSearchRefresh measures one incremental refresh pass: every file
// is re-discovered and stat'ed, and `changed` documents per pass are
// rewritten with a fresh mtime so exactly they must be re-read and
// re-parsed. changed=0 is the "nothing happened" floor (metadata scan
// only); changed=1 is the single-edit case a live workspace hits most.
func benchmarkSearchRefresh(b *testing.B, count, changed int) {
	b.Helper()
	_, root, _ := benchmarkWorkspace(b, count)
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: root, discovery: files.DiscoverOptions{Depth: 4, Pattern: "**/*.md"}}),
		ui:        directoryTestUI(),
	}
	routed := handler.routes(nil)
	if response := performRequest(routed, http.MethodGet, "/api/search?q="+url.QueryEscape("document 0042")); response.Code != http.StatusOK {
		b.Fatalf("initial query status = %d", response.Code)
	}
	index := handler.search
	target := filepath.Join(root, "group-000", "doc-0000.md")
	stamp := time.Now().Add(time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if changed > 0 {
			contents := fmt.Sprintf("# Benchmark document rewritten %d\n\n第 %d 次增量刷新改写的正文段落。\n", i, i)
			if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
				b.Fatal(err)
			}
			stamp = stamp.Add(time.Second)
			if err := os.Chtimes(target, stamp, stamp); err != nil {
				b.Fatal(err)
			}
		}
		snapshot, err := index.refresh(context.Background(), index.snapshot.Load())
		if err != nil {
			b.Fatalf("refresh: %v", err)
		}
		index.snapshot.Store(snapshot)
	}
}

func BenchmarkSearchRefreshUnchanged2K(b *testing.B)  { benchmarkSearchRefresh(b, 2000, 0) }
func BenchmarkSearchRefreshUnchanged10K(b *testing.B) { benchmarkSearchRefresh(b, 10000, 0) }
func BenchmarkSearchRefreshOneChanged2K(b *testing.B) { benchmarkSearchRefresh(b, 2000, 1) }
func BenchmarkSearchRefreshOneChanged10K(b *testing.B) {
	benchmarkSearchRefresh(b, 10000, 1)
}

// BenchmarkSearchConcurrent2K drives parallel queries against a warm index —
// the shape a room full of readers produces. Matching is lock-free over the
// immutable snapshot, so the only shared state is the atomic snapshot load.
func BenchmarkSearchConcurrent2K(b *testing.B) {
	b.Helper()
	handler, _, _ := benchmarkWorkspace(b, 2000)
	target := "/api/search?q=" + url.QueryEscape("中英文混合")
	if response := performRequest(handler, http.MethodGet, target); response.Code != http.StatusOK {
		b.Fatalf("warmup status = %d", response.Code)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			response := performRequest(handler, http.MethodGet, target)
			if response.Code != http.StatusOK {
				b.Errorf("GET %s status = %d", target, response.Code)
				return
			}
		}
	})
}

// benchmarkSearchIndexMemory reports the heap a fully built index retains
// per document: the search projections plus their precompiled lowercase
// forms, measured around one cold build after a GC.
func benchmarkSearchIndexMemory(b *testing.B, count int) {
	b.Helper()
	_, root, _ := benchmarkWorkspace(b, count)
	scope := rootScope{root: root, discovery: files.DiscoverOptions{Depth: 4, Pattern: "**/*.md"}}
	var total float64
	b.ResetTimer()
	for range b.N {
		handler := &documentHandler{workspace: singleRootWorkspace(scope), ui: directoryTestUI()}
		routed := handler.routes(nil)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		response := performRequest(routed, http.MethodGet, "/api/search?q="+url.QueryEscape("missing-token-zzz"))
		if response.Code != http.StatusOK {
			b.Fatalf("build query status = %d", response.Code)
		}
		runtime.ReadMemStats(&after)
		total += float64(after.HeapAlloc) - float64(before.HeapAlloc)
	}
	b.ReportMetric(total/float64(b.N)/float64(count), "bytes/doc")
}

func BenchmarkSearchIndexMemory2K(b *testing.B)  { benchmarkSearchIndexMemory(b, 2000) }
func BenchmarkSearchIndexMemory10K(b *testing.B) { benchmarkSearchIndexMemory(b, 10000) }
