package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"github.com/lz-wang/m2h/internal/files"
	"github.com/lz-wang/m2h/internal/search"
)

// Refresh failures carry one of these sentinels so serveSearch can answer
// with the same 500 message the full-scan implementation used, telling the
// client whether discovery or document reading broke.
var (
	errSearchDiscover = errors.New("discover Markdown files")
	errSearchRead     = errors.New("search Markdown documents")
)

// indexedDocument is one published document's cache entry: the canonical file
// identity its projection was built from, plus the projection itself. Target,
// Size and ModTime together decide whether a later refresh can reuse the
// projection or must re-read and re-parse the file. An entry whose file was
// swapped underneath it (same target path, different size or mtime) is simply
// replaced; content equality is never hashed, so an equal-length overwrite
// that preserves mtime stays undetected until the next metadata change — the
// documented weak-consistency trade for not re-reading every file.
type indexedDocument struct {
	Target   string
	Size     int64
	ModTime  time.Time
	Document search.Document
}

// searchSnapshot is the immutable result of one successful refresh. Files is
// keyed by workspace-public path (r0/docs/a.md in a multi-root workspace,
// docs/a.md otherwise); the documented Document.Path keeps its root-relative
// meaning, and responses overwrite the result path with the public key so
// search identity stays exactly what /api/document addresses.
type searchSnapshot struct {
	Files     map[string]*indexedDocument
	Refreshed time.Time
}

// searchIndex owns the shared snapshot behind /api/search. Refreshes reuse
// the workspace's own discovery and visibility boundary — the index caches
// projections, it never re-decides what is publishable. Published snapshots
// are immutable: a refresh builds the next map in full (reusing unchanged
// entries by pointer) and swaps it in atomically, so a query already walking
// an older snapshot never sees a mutation.
type searchIndex struct {
	handler  *documentHandler
	snapshot atomic.Pointer[searchSnapshot]
}

func newSearchIndex(handler *documentHandler) *searchIndex {
	return &searchIndex{handler: handler}
}

// acquire returns the snapshot that answers one query. This first version
// refreshes synchronously on every request — the file set is re-discovered
// and every changed file re-parsed before the query runs — so search keeps
// the read-your-writes behavior of the full scan while the incremental
// machinery is proven in. Lazy TTL refresh lands on top of it later.
func (index *searchIndex) acquire(ctx context.Context) (*searchSnapshot, error) {
	previous := index.snapshot.Load()
	next, err := index.refresh(ctx, previous)
	if err != nil {
		return nil, err
	}
	index.snapshot.Store(next)
	return next, nil
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
				Document: document,
			}
		}
	}
	return &searchSnapshot{Files: next, Refreshed: time.Now()}, nil
}
