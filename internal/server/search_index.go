package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/search"
)

// searchRefreshInterval is how long a published snapshot stays fresh. A query
// inside the window answers straight from memory; the first query after the
// window kicks off one background refresh while still answering from the old
// snapshot (stale-while-revalidate). The TTL is measured from the last
// successful refresh, never from query time, so frequent searching cannot
// keep extending a stale snapshot's life.
const searchRefreshInterval = 30 * time.Second

// Refresh failures carry one of these sentinels so serveSearch can answer
// with the same 500 message the full-scan implementation used, telling the
// client whether discovery or document reading broke.
var (
	errSearchDiscover = errors.New("discover Markdown files")
	errSearchRead     = errors.New("search Markdown documents")
)

// searchClock abstracts the wall clock the index lives on. Production uses
// the system clock; tests inject a controllable one so TTL expiry and retry
// backoff are exercised by advancing time, never by sleeping.
type searchClock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// indexedDocument is one published document's cache entry: the canonical file
// identity its projection was built from, plus the projection in its
// precompiled search form. Target, Size and ModTime together decide whether
// a later refresh can reuse the projection or must re-read and re-parse the
// file. An entry whose file was swapped underneath it (same target path,
// different size or mtime) is simply replaced; content equality is never
// hashed, so an equal-length overwrite that preserves mtime stays undetected
// until the next metadata change — the documented weak-consistency trade for
// not re-reading every file.
type indexedDocument struct {
	Target   string
	Size     int64
	ModTime  time.Time
	Prepared search.PreparedDocument
}

// searchSnapshot is the immutable result of one successful refresh. Files is
// keyed by workspace-public path (r0/docs/a.md in a multi-root workspace,
// docs/a.md otherwise); the documented Document.Path keeps its root-relative
// meaning, and responses overwrite the result path with the public key so
// search identity stays exactly what /api/document addresses. Refreshed is
// when that refresh completed — the anchor the TTL is measured from.
type searchSnapshot struct {
	Files     map[string]*indexedDocument
	Refreshed time.Time
}

// searchIndex owns the shared snapshot behind /api/search.
//
// Lifecycle: the first query starts one initial build on the server's
// lifecycle context — never a request context, because the WebUI cancels
// superseded searches and a build tied to a dying request would restart
// from scratch with every keystroke — and waits for it through the ready
// channel like every concurrent first query. A cancelled waiter abandons
// only its own wait; server shutdown is what cancels the build itself. A
// failed build reports its error to the waiters as their 500 and the next
// request may retry. Later queries answer from the published snapshot
// without any filesystem work; once it is older than the TTL, one query
// triggers a single background refresh (the same lifecycle context) while
// every query keeps answering from the old snapshot until the new one is
// published. A failed refresh keeps the old snapshot and backs off for one
// TTL before another may start.
//
// Locking: mu coordinates refresh state only — it is never held while
// Markdown is read or parsed. Matching runs lock-free over an immutable
// snapshot; refreshes build the next map in full (reusing unchanged entries
// by pointer) and swap it in atomically, so a query already walking an older
// snapshot never sees a mutation. clock is written only at construction
// (before the first request) and read concurrently afterwards, so it needs
// no lock; an injected test clock provides its own synchronization.
type searchIndex struct {
	handler *documentHandler
	// ctx scopes the initial build and background refreshes. It is the
	// server's lifecycle context: request contexts die with their HTTP round
	// trip and would abort the work the moment the triggering response is
	// written or the client walks away.
	ctx context.Context

	clock searchClock

	mu          sync.Mutex
	snapshot    atomic.Pointer[searchSnapshot]
	refreshing  bool
	ready       chan struct{} // closed when an initial build finishes
	buildErr    error         // outcome of the initial build, for its waiters
	nextRetry   time.Time     // no new refresh may start before this
	onRefreshed func(error)   // test notification, invoked outside mu
}

func newSearchIndex(handler *documentHandler) *searchIndex {
	return &searchIndex{
		handler: handler,
		ctx:     handler.runContext,
		clock:   systemClock{},
	}
}

// acquire returns the snapshot that answers one query.
func (index *searchIndex) acquire(ctx context.Context) (*searchSnapshot, error) {
	for {
		index.mu.Lock()
		current := index.snapshot.Load()
		if current != nil && index.clock.Now().Before(current.Refreshed.Add(searchRefreshInterval)) {
			index.mu.Unlock()
			return current, nil
		}

		if current == nil {
			// No snapshot yet: one shared initial build runs on the server's
			// lifecycle context, and every first query — including the one
			// that started it — waits for the same outcome through ready. A
			// cancelled waiter abandons only its own wait, never the build.
			if !index.refreshing {
				index.refreshing = true
				index.buildErr = nil
				index.ready = make(chan struct{})
				go index.initialBuild()
			}
			ready := index.ready
			index.mu.Unlock()
			select {
			case <-ready:
				// Build finished: success published a snapshot (the loop
				// re-reads it), failure left none and reports to waiters.
				index.mu.Lock()
				err := index.buildErr
				index.mu.Unlock()
				if err != nil {
					return nil, err
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Stale snapshot: answer from it immediately and let one background
		// refresh bring the next one — unless a refresh is already running or
		// the failure backoff has not elapsed yet.
		if !index.refreshing && index.clock.Now().After(index.nextRetry) {
			index.refreshing = true
			index.mu.Unlock()
			go index.backgroundRefresh()
		} else {
			index.mu.Unlock()
		}
		return current, nil
	}
}

// initialBuild runs the first full build off the request path, on the
// server's lifecycle context, and wakes every waiter with the outcome in
// place: success publishes the snapshot, failure records the error for the
// waiters (their 500). With no snapshot yet there is nothing to serve, so
// the next request retries the build immediately — nextRetry's backoff
// only governs refreshes once a snapshot exists.
func (index *searchIndex) initialBuild() {
	snapshot, err := index.refresh(index.ctx, nil)

	index.mu.Lock()
	index.refreshing = false
	ready := index.ready
	index.ready = nil
	index.buildErr = err
	if err != nil {
		index.nextRetry = index.clock.Now().Add(searchRefreshInterval)
	} else {
		index.snapshot.Store(snapshot)
		index.nextRetry = time.Time{}
	}
	hook := index.onRefreshed
	index.mu.Unlock()

	close(ready) // wake every waiter with the outcome in place
	if hook != nil {
		hook(err)
	}
}

// backgroundRefresh runs one refresh off the request path, on the server's
// lifecycle context. It publishes only on full success; a failure keeps the
// previous snapshot and delays the next attempt by one TTL.
func (index *searchIndex) backgroundRefresh() {
	previous := index.snapshot.Load()
	snapshot, err := index.refresh(index.ctx, previous)

	index.mu.Lock()
	index.refreshing = false
	if err != nil {
		index.nextRetry = index.clock.Now().Add(searchRefreshInterval)
	} else {
		index.snapshot.Store(snapshot)
		index.nextRetry = time.Time{}
	}
	hook := index.onRefreshed
	index.mu.Unlock()

	if hook != nil {
		hook(err)
	}
}

// refresh runs one full discovery pass and returns the next snapshot.
// Unchanged files (same canonical target, size and mtime) keep their cached
// projection by pointer; changed and new files are read and re-projected
// through searchDocument; files that no longer appear in discovery simply
// never enter the next map. Nothing is published here — the caller stores
// the returned snapshot only after a fully successful pass, so a failed
// refresh leaves the previous snapshot untouched.
func (index *searchIndex) refresh(ctx context.Context, previous *searchSnapshot) (*searchSnapshot, error) {
	if previous == nil {
		previous = &searchSnapshot{}
	}
	next := make(map[string]*indexedDocument, len(previous.Files))
	handler := index.handler

	for _, root := range handler.workspace.roots {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		discovered, err := handler.discover(ctx, root.scope)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				discovered = files.Discovery{}
			} else {
				return nil, fmt.Errorf("%w: %v", errSearchDiscover, err)
			}
		}
		for _, entry := range discovered.Markdown {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			virtual := handler.workspace.publicPath(root.id, entry.RelativePath)
			// Discovery only records candidates; the read re-enters the same
			// resolveVisibleDocument boundary /api/document and /raw use, so
			// an entry that vanished, was re-hidden or now escapes its root
			// after discovery is simply absent from the index — exactly the
			// answer the document routes give for the same file.
			_, relative, target, err := handler.resolveVisibleDocument(virtual)
			if err != nil {
				// An ignore-rule failure is a server fault even mid-scan;
				// everything else mirrors the document routes' 404 answer:
				// the entry is simply left out.
				if errors.Is(err, errIgnoreUnavailable) {
					return nil, err
				}
				continue
			}
			info, err := os.Stat(target)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("%w: %v", errSearchRead, err)
			}
			if cached, reused := previous.Files[virtual]; reused &&
				cached.Target == target &&
				cached.Size == info.Size() &&
				cached.ModTime.Equal(info.ModTime()) {
				next[virtual] = cached
				continue
			}
			contents, err := os.ReadFile(target)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("%w: %v", errSearchRead, err)
			}
			// An unstable file may be rewritten mid-read; a projection bound
			// to attributes that no longer match the bytes is worse than a
			// temporarily missing entry, so the file sits this refresh out
			// and the next one picks up its settled state.
			after, err := os.Stat(target)
			if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
				continue
			}
			document, err := searchDocument(contents, relative)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errSearchRead, err)
			}
			next[virtual] = &indexedDocument{
				Target:   target,
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				Prepared: search.PrepareDocument(document),
			}
		}
	}
	return &searchSnapshot{Files: next, Refreshed: index.clock.Now()}, nil
}
