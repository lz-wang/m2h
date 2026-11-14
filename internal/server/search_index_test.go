package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/files"
)

// fakeSearchClock is a controllable searchClock: tests advance it instead of
// sleeping, so TTL expiry and retry backoff are deterministic. Its own lock
// makes Advance safe even while a background refresh reads Now concurrently.
type fakeSearchClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeSearchClock() *fakeSearchClock {
	return &fakeSearchClock{now: time.Now()}
}

func (clock *fakeSearchClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fakeSearchClock) Advance(step time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(step)
}

// indexedSearchHandler builds a handler whose internal search index is
// reachable for white-box assertions: the tests below watch snapshot
// identity (which entries were reused by pointer, which were re-parsed)
// while going through the real HTTP surface for the queries themselves. The
// returned clock controls the index's TTL; swap time forward instead of
// sleeping.
func indexedSearchHandler(t *testing.T, root string, discovery files.DiscoverOptions) (*documentHandler, http.Handler, *fakeSearchClock) {
	t.Helper()
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: discovery}),
		ui:        directoryTestUI(),
	}
	routed := handler.routes(nil)
	clock := newFakeSearchClock()
	handler.search.clock = clock
	return handler, routed, clock
}

// indexedSearchQuery runs one search request and fails the test unless it
// answers 200.
func indexedSearchQuery(t *testing.T, handler http.Handler, query string) searchResponse {
	t.Helper()
	response := performRequest(handler, http.MethodGet, "/api/search?q="+url.QueryEscape(query))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/search?q=%s status = %d, body = %s", query, response.Code, response.Body.String())
	}
	return decodeSearch(t, response)
}

// expectRefresh arms the notification hook for the next refresh that
// finishes and reports its outcome channel.
func (index *searchIndex) expectRefresh() <-chan error {
	done := make(chan error, 1)
	index.mu.Lock()
	index.onRefreshed = func(err error) { done <- err }
	index.mu.Unlock()
	return done
}

// forceRefresh advances the clock past the TTL, runs one query (which
// triggers exactly one background refresh and answers from the old snapshot),
// waits for the refresh to land, and answers from the new snapshot — the
// deterministic stand-in for "some time passed and a search happened".
func forceRefresh(t *testing.T, index *searchIndex, clock *fakeSearchClock, routed http.Handler, query string) searchResponse {
	t.Helper()
	clock.Advance(2 * searchRefreshInterval)
	done := index.expectRefresh()
	indexedSearchQuery(t, routed, query)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("background refresh: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("background refresh did not complete in time")
	}
	return indexedSearchQuery(t, routed, query)
}

// touchForRefresh rewrites a file and pushes its mtime far forward, so the
// change is detectable through size or mtime no matter how coarse the
// filesystem's timestamps are.
func touchForRefresh(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(name, future, future); err != nil {
		t.Fatal(err)
	}
}

// Unchanged files keep their cached projection by pointer across refreshes;
// only the edited file is re-read and re-parsed; a deleted file leaves the
// next snapshot. This is the incremental contract every refresh (initial
// build, TTL-triggered, background) is built on.
func TestSearchIndexReusesUnchangedDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	stable := filepath.Join(root, "stable.md")
	edited := filepath.Join(root, "edited.md")
	writeTestFile(t, stable, "# Stable\n\nstable-token\n")
	writeTestFile(t, edited, "# Edited\n\nold-token\n")
	handler, routed, clock := indexedSearchHandler(t, root, files.DiscoverOptions{Depth: 2})

	if payload := indexedSearchQuery(t, routed, "old-token"); len(payload.Results) != 1 {
		t.Fatalf("initial results = %+v, want edited.md", payload.Results)
	}
	first := handler.search.snapshot.Load()

	// A refresh with no filesystem change publishes a fresh snapshot whose
	// entries are the very same pointers: nothing was re-parsed.
	if payload := forceRefresh(t, handler.search, clock, routed, "stable-token"); len(payload.Results) != 1 {
		t.Fatalf("refreshed results = %+v, want stable.md", payload.Results)
	}
	second := handler.search.snapshot.Load()
	if second == first {
		t.Fatal("refresh did not publish a new snapshot")
	}
	for _, path := range []string{"stable.md", "edited.md"} {
		if first.Files[path] == nil || second.Files[path] == nil {
			t.Fatalf("snapshot lost %s: first=%v second=%v", path, first.Files, second.Files)
		}
		if first.Files[path] != second.Files[path] {
			t.Fatalf("%s was re-parsed although its file did not change", path)
		}
	}

	// Editing one file re-parses exactly that entry.
	touchForRefresh(t, edited, "# Edited\n\nnew-token\n")
	if payload := forceRefresh(t, handler.search, clock, routed, "new-token"); len(payload.Results) != 1 || payload.Results[0].Path != "edited.md" {
		t.Fatalf("results after edit = %+v, want edited.md", payload.Results)
	}
	third := handler.search.snapshot.Load()
	if third.Files["edited.md"] == first.Files["edited.md"] {
		t.Fatal("edited entry was reused despite the file changing")
	}
	if third.Files["stable.md"] != first.Files["stable.md"] {
		t.Fatal("stable entry was re-parsed despite no change")
	}
	if payload := indexedSearchQuery(t, routed, "old-token"); len(payload.Results) != 0 {
		t.Fatalf("stale token survived the refresh: %+v", payload.Results)
	}

	// Deleting a file removes it from the next snapshot.
	if err := os.Remove(edited); err != nil {
		t.Fatal(err)
	}
	if payload := forceRefresh(t, handler.search, clock, routed, "stable-token"); len(payload.Results) != 1 {
		t.Fatalf("refreshed results after delete = %+v, want stable.md", payload.Results)
	}
	fourth := handler.search.snapshot.Load()
	if _, exists := fourth.Files["edited.md"]; exists {
		t.Fatal("deleted document still indexed")
	}
	if fourth.Files["stable.md"] != first.Files["stable.md"] {
		t.Fatal("stable entry was re-parsed despite no change")
	}
	if payload := indexedSearchQuery(t, routed, "new-token"); len(payload.Results) != 0 {
		t.Fatalf("deleted document still searchable: %+v", payload.Results)
	}
}

// Files added after the first build join the index on the next refresh and
// are searchable, including when they appear in a directory no earlier pass
// had seen.
func TestSearchIndexPicksUpNewFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "first.md"), "# First\n\nfirst-token\n")
	handler, routed, clock := indexedSearchHandler(t, root, files.DiscoverOptions{Depth: 4})

	if payload := indexedSearchQuery(t, routed, "added-token"); len(payload.Results) != 0 {
		t.Fatalf("results before the file exists = %+v, want none", payload.Results)
	}

	writeTestFile(t, filepath.Join(root, "nested", "added.md"), "# Added\n\nadded-token\n")
	if payload := forceRefresh(t, handler.search, clock, routed, "added-token"); len(payload.Results) != 1 || payload.Results[0].Path != "nested/added.md" {
		t.Fatalf("results after add = %+v, want nested/added.md", payload.Results)
	}
}

// Each root keeps its own public identity: an edit in one root re-parses
// only that root's entry and never bleeds into the sibling root's results.
func TestSearchIndexMultiRootUpdates(t *testing.T) {
	t.Parallel()

	rootA := t.TempDir()
	rootB := t.TempDir()
	writeTestFile(t, filepath.Join(rootA, "doc.md"), "# A\n\ndual-token alpha\n")
	writeTestFile(t, filepath.Join(rootB, "doc.md"), "# B\n\ndual-token beta\n")
	handler := &documentHandler{
		workspace: workspace{roots: []workspaceRoot{
			{id: "r0", label: "a", input: files.Input{Kind: files.KindDirectory, Path: rootA}, scope: rootScope{root: canonicalDirectory(t, rootA), discovery: files.DiscoverOptions{Depth: 2}}},
			{id: "r1", label: "b", input: files.Input{Kind: files.KindDirectory, Path: rootB}, scope: rootScope{root: canonicalDirectory(t, rootB), discovery: files.DiscoverOptions{Depth: 2}}},
		}},
		ui: directoryTestUI(),
	}
	routed := handler.routes(nil)
	clock := newFakeSearchClock()
	handler.search.clock = clock

	if payload := indexedSearchQuery(t, routed, "dual-token"); len(payload.Results) != 2 {
		t.Fatalf("initial results = %+v, want one per root", payload.Results)
	}
	first := handler.search.snapshot.Load()

	touchForRefresh(t, filepath.Join(rootA, "doc.md"), "# A\n\ndual-token alpha-v2\n")
	if payload := forceRefresh(t, handler.search, clock, routed, "alpha-v2"); len(payload.Results) != 1 || payload.Results[0].Path != "r0/doc.md" {
		t.Fatalf("results after r0 edit = %+v, want r0/doc.md", payload.Results)
	}
	if payload := indexedSearchQuery(t, routed, "beta"); len(payload.Results) != 1 || payload.Results[0].Path != "r1/doc.md" {
		t.Fatalf("r1 lost its identity after an r0 edit: %+v", payload.Results)
	}
	second := handler.search.snapshot.Load()
	if second.Files["r0/doc.md"] == first.Files["r0/doc.md"] {
		t.Fatal("r0 entry was reused despite the edit")
	}
	if second.Files["r1/doc.md"] != first.Files["r1/doc.md"] {
		t.Fatal("r1 entry was re-parsed although its root did not change")
	}
}

// A query inside the TTL answers entirely from memory: no discovery, no file
// read, no re-parse. Discovery runs exactly once for the initial build and
// once per TTL window afterwards.
func TestSearchHotQuerySkipsDiscovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n\nhot-token\n")
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		ui:        directoryTestUI(),
	}
	discoveries := 0
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		discoveries++
		return scope.discover(ctx)
	}
	routed := handler.routes(nil)
	clock := newFakeSearchClock()
	handler.search.clock = clock

	for range 3 {
		if payload := indexedSearchQuery(t, routed, "hot-token"); len(payload.Results) != 1 {
			t.Fatalf("hot results = %+v", payload.Results)
		}
	}
	if discoveries != 1 {
		t.Fatalf("hot queries ran discovery %d times, want exactly the initial build", discoveries)
	}

	// Only crossing the TTL brings discovery back — once for that window.
	forceRefresh(t, handler.search, clock, routed, "hot-token")
	if discoveries != 2 {
		t.Fatalf("after one expired window discovery ran %d times, want 2", discoveries)
	}
	for range 3 {
		if payload := indexedSearchQuery(t, routed, "hot-token"); len(payload.Results) != 1 {
			t.Fatalf("hot results after refresh = %+v", payload.Results)
		}
	}
	if discoveries != 2 {
		t.Fatalf("hot queries inside the new window ran discovery %d times, want 2", discoveries)
	}
}

// When the TTL lapses, concurrent queries still answer immediately and only
// one background refresh starts; the rest share its result.
func TestSearchSuppressesConcurrentRefreshes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for index := range 20 {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("doc-%02d.md", index)), "# Doc\n\nconcurrent-token\n")
	}
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		ui:        directoryTestUI(),
	}
	discoveries := 0
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		discoveries++
		return scope.discover(ctx)
	}
	routed := handler.routes(nil)
	clock := newFakeSearchClock()
	handler.search.clock = clock

	indexedSearchQuery(t, routed, "concurrent-token") // initial build
	if discoveries != 1 {
		t.Fatalf("initial build ran discovery %d times, want 1", discoveries)
	}
	done := handler.search.expectRefresh()
	clock.Advance(2 * searchRefreshInterval)

	var wait sync.WaitGroup
	for range 8 {
		wait.Go(func() {
			response := performRequest(routed, http.MethodGet, "/api/search?q=concurrent-token")
			if response.Code != http.StatusOK {
				t.Errorf("concurrent query status = %d", response.Code)
			}
		})
	}
	wait.Wait()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("background refresh: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("background refresh did not complete in time")
	}
	if discoveries != 2 {
		t.Fatalf("8 concurrent stale queries triggered %d refreshes, want exactly 1", discoveries-1)
	}
}

// A failed refresh keeps the previous snapshot answering (validated per
// result), backs off for a TTL, and recovers on its own once the failure
// clears — no wedged state, no rebuild from scratch.
func TestSearchRefreshFailureKeepsSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n\nresilient-token\n")
	handler := &documentHandler{
		workspace: singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		ui:        directoryTestUI(),
	}
	sabotaged := false
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		if sabotaged {
			return files.Discovery{}, errors.New("disk on fire")
		}
		return scope.discover(ctx)
	}
	routed := handler.routes(nil)
	clock := newFakeSearchClock()
	handler.search.clock = clock

	indexedSearchQuery(t, routed, "resilient-token") // initial build
	before := handler.search.snapshot.Load()

	// The refresh fails; the query that triggered it still answers from the
	// intact old snapshot.
	sabotaged = true
	clock.Advance(2 * searchRefreshInterval)
	done := handler.search.expectRefresh()
	if payload := indexedSearchQuery(t, routed, "resilient-token"); len(payload.Results) != 1 {
		t.Fatalf("stale query during failed refresh lost results: %+v", payload.Results)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("sabotaged refresh unexpectedly succeeded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("sabotaged refresh did not finish in time")
	}
	if handler.search.snapshot.Load() != before {
		t.Fatal("failed refresh replaced the snapshot")
	}

	// Inside the backoff window no new refresh is attempted.
	clock.Advance(searchRefreshInterval / 2)
	if payload := indexedSearchQuery(t, routed, "resilient-token"); len(payload.Results) != 1 {
		t.Fatalf("backoff query lost results: %+v", payload.Results)
	}
	if handler.search.snapshot.Load() != before {
		t.Fatal("a refresh ran inside the backoff window")
	}

	// After the backoff the next stale query retries and recovers.
	sabotaged = false
	writeTestFile(t, filepath.Join(root, "b.md"), "# B\n\nrecovered-token\n")
	if payload := forceRefresh(t, handler.search, clock, routed, "recovered-token"); len(payload.Results) != 1 || payload.Results[0].Path != "b.md" {
		t.Fatalf("recovery results = %+v, want b.md", payload.Results)
	}
	if payload := indexedSearchQuery(t, routed, "resilient-token"); len(payload.Results) != 1 {
		t.Fatalf("pre-failure content lost after recovery: %+v", payload.Results)
	}
}

// Shutting the server down cancels an in-flight background refresh through
// the lifecycle context: it stops cleanly, keeps the last snapshot, and the
// backoff keeps a cancelled server from thrashing the disk afterwards.
func TestSearchIndexStopsRefreshingOnShutdown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n\nshutdown-token\n")
	runContext, cancel := context.WithCancel(context.Background())
	handler := &documentHandler{
		workspace:  singleRootWorkspace(rootScope{root: canonicalDirectory(t, root), discovery: files.DiscoverOptions{Depth: 2}}),
		ui:         directoryTestUI(),
		runContext: runContext,
	}
	hang := false
	blocked := make(chan struct{}, 1)
	handler.discover = func(ctx context.Context, scope rootScope) (files.Discovery, error) {
		if !hang {
			return scope.discover(ctx)
		}
		select {
		case blocked <- struct{}{}:
		default:
		}
		<-ctx.Done() // the refresh hangs until the server goes away
		return files.Discovery{}, ctx.Err()
	}
	routed := handler.routes(nil)
	clock := newFakeSearchClock()
	handler.search.clock = clock

	if payload := indexedSearchQuery(t, routed, "shutdown-token"); len(payload.Results) != 1 {
		t.Fatalf("initial results = %+v", payload.Results)
	}
	before := handler.search.snapshot.Load()

	// Cross the TTL with discovery hanging: the query answers from the old
	// snapshot while the background refresh waits inside discovery.
	hang = true
	done := handler.search.expectRefresh()
	clock.Advance(2 * searchRefreshInterval)
	if payload := indexedSearchQuery(t, routed, "shutdown-token"); len(payload.Results) != 1 {
		t.Fatalf("query during hanging refresh lost results: %+v", payload.Results)
	}
	<-blocked

	cancel() // the server goes away
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled refresh reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled refresh did not finish in time")
	}
	if handler.search.snapshot.Load() != before {
		t.Fatal("cancelled refresh touched the snapshot")
	}

	// Queries after shutdown keep answering from the intact snapshot for the
	// remainder of the process lifetime; no further refresh is attempted
	// inside the backoff window.
	clock.Advance(searchRefreshInterval / 2)
	if payload := indexedSearchQuery(t, routed, "shutdown-token"); len(payload.Results) != 1 {
		t.Fatalf("post-shutdown query lost results: %+v", payload.Results)
	}
	if handler.search.snapshot.Load() != before {
		t.Fatal("a refresh ran after shutdown")
	}
}

// A symlink swap that happens while an old snapshot keeps answering must not
// leak the alias's new target: the per-result revalidation refuses entries
// whose canonical target moved, exactly as /api/document refuses them.
func TestSearchStaleSnapshotSurvivesSymlinkSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	t.Parallel()

	root := canonicalDirectory(t, t.TempDir())
	outside := canonicalDirectory(t, t.TempDir())
	writeTestFile(t, filepath.Join(outside, "secret.md"), "# Secret\n\nswap-after-index-token\n")
	writeTestFile(t, filepath.Join(root, "target.md"), "# Target\n\nkeep-token\n")
	alias := filepath.Join(root, "alias.md")
	if err := os.Symlink(filepath.Join(root, "target.md"), alias); err != nil {
		t.Fatal(err)
	}
	handler, routed, clock := indexedSearchHandler(t, root, files.DiscoverOptions{Depth: 4})

	if payload := indexedSearchQuery(t, routed, "keep-token"); len(payload.Results) != 2 {
		t.Fatalf("initial results = %+v, want alias and target", payload.Results)
	}

	// Re-point the alias outside the root, then query before the TTL lapses:
	// the stale snapshot still lists alias.md, but revalidation must drop it.
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), alias); err != nil {
		t.Fatal(err)
	}
	if payload := indexedSearchQuery(t, routed, "swap-after-index-token"); len(payload.Results) != 0 {
		t.Fatalf("stale snapshot leaked the swapped alias target: %+v", payload.Results)
	}
	payload := indexedSearchQuery(t, routed, "keep-token")
	if len(payload.Results) != 1 || payload.Results[0].Path != "target.md" {
		t.Fatalf("results from stale snapshot = %+v, want only target.md", payload.Results)
	}

	// After a refresh the swapped alias is gone from the index itself.
	if payload := forceRefresh(t, handler.search, clock, routed, "keep-token"); len(payload.Results) != 1 || payload.Results[0].Path != "target.md" {
		t.Fatalf("refreshed results = %+v, want only target.md", payload.Results)
	}
}
