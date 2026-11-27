package server

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/lz-wang/m2h/internal/markdown"
)

const (
	shareRenderEntries = 64
	shareRenderBytes   = 16 << 20
)

// preparedShare holds sanitized markup and logical references. The cache
// never holds authorization decisions or resolved filesystem paths.
type preparedShare struct {
	document   publicDocumentResponse
	references map[string]struct{}
}

type shareRenderEntry struct {
	id    string
	hash  [sha256.Size]byte
	value preparedShare
	bytes int
}

type shareRenderCache struct {
	mu       sync.Mutex
	lru      list.List
	entries  map[string]*list.Element
	bytes    int
	revision uint64
	inflight singleflight.Group
}

func (cache *shareRenderCache) snapshot() uint64 {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.revision
}

func (cache *shareRenderCache) get(id string, hash [sha256.Size]byte) (preparedShare, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if element := cache.entries[id]; element != nil {
		entry := element.Value.(shareRenderEntry)
		if entry.hash == hash {
			cache.lru.MoveToFront(element)
			return entry.value, true
		}
	}
	return preparedShare{}, false
}

func (cache *shareRenderCache) put(entry shareRenderEntry, revision uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if revision != cache.revision || entry.bytes > shareRenderBytes {
		return
	}
	if cache.entries == nil {
		cache.entries = make(map[string]*list.Element)
	}
	if previous := cache.entries[entry.id]; previous != nil {
		cache.evict(previous)
	}
	cache.entries[entry.id] = cache.lru.PushFront(entry)
	cache.bytes += entry.bytes
	for cache.lru.Len() > shareRenderEntries || cache.bytes > shareRenderBytes {
		cache.evict(cache.lru.Back())
	}
}

func (cache *shareRenderCache) evict(element *list.Element) {
	entry := element.Value.(shareRenderEntry)
	delete(cache.entries, entry.id)
	cache.bytes -= entry.bytes
	cache.lru.Remove(element)
}

func (cache *shareRenderCache) remove(id string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.revision++
	if element := cache.entries[id]; element != nil {
		cache.evict(element)
	}
}

func (handler *documentHandler) loadShareRender(request *http.Request, id string) (preparedShare, workspaceRoot, error) {
	cache := &handler.shareRenders
	revision := cache.snapshot()
	record, root, target, err := handler.loadPublicShare(request, id)
	if err != nil {
		return preparedShare{}, workspaceRoot{}, err
	}
	contents, err := readShareFile(root, target)
	if err != nil {
		return preparedShare{}, workspaceRoot{}, err
	}
	hash := sha256.Sum256(contents)
	if value, cached := cache.get(id, hash); cached {
		return value, root, nil
	}
	value, err, _ := cache.inflight.Do(id+hex.EncodeToString(hash[:]), func() (any, error) {
		if value, cached := cache.get(id, hash); cached {
			return value, nil
		}
		entry, err := prepareShareRender(id, record.DocumentPath, hash, contents)
		if err != nil {
			return nil, err
		}
		cache.put(entry, revision)
		return entry.value, nil
	})
	if err != nil {
		return preparedShare{}, workspaceRoot{}, err
	}
	return value.(preparedShare), root, nil
}

func prepareShareRender(id, path string, hash [sha256.Size]byte, contents []byte) (shareRenderEntry, error) {
	body, metadata, err := markdown.ParseFrontMatter(contents)
	if err != nil {
		return shareRenderEntry{}, fmt.Errorf("parse shared document: %w", err)
	}
	references := make(map[string]struct{})
	rendered, err := markdown.RenderShare(body, path, func(relative string) string {
		references[relative] = struct{}{}
		return escapeShareAssetPath(relative)
	})
	if err != nil {
		return shareRenderEntry{}, err
	}
	document := publicDocumentResponse{Title: markdown.PreferredTitle(metadata, rendered.Title),
		HTML: rendered.Body, TOC: tocEntriesFrom(rendered.Headings)}
	size := len(id) + len(document.Title) + len(document.HTML) + 256
	for reference := range references {
		size += len(reference) + 32
	}
	for _, heading := range document.TOC {
		size += len(heading.ID) + len(heading.Text) + 48
	}
	return shareRenderEntry{id: id, hash: hash,
		value: preparedShare{document: document, references: references}, bytes: size}, nil
}

func escapeShareAssetPath(relative string) string {
	escaped := (&url.URL{Path: "/" + relative}).EscapedPath()
	return strings.ReplaceAll(escaped, ",", "%2C")
}
