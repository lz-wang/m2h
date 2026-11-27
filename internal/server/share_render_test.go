package server

import (
	"crypto/sha256"
	"strconv"
	"testing"
)

func TestShareRenderCacheBoundsAndInvalidation(t *testing.T) {
	var cache shareRenderCache
	hash := sha256.Sum256([]byte("document"))
	revision := cache.snapshot()
	for index := range shareRenderEntries {
		cache.put(shareRenderEntry{id: strconv.Itoa(index), hash: hash, bytes: 1}, revision)
	}
	cache.get("0", hash)
	cache.put(shareRenderEntry{id: "last", hash: hash, bytes: 1}, revision)
	if _, ok := cache.get("1", hash); ok {
		t.Fatal("least recently used entry survived")
	}
	if _, ok := cache.get("0", hash); !ok {
		t.Fatal("recently used entry was evicted")
	}
	if _, ok := cache.get("0", sha256.Sum256([]byte("updated"))); ok {
		t.Fatal("outdated document version was reused")
	}
	cache.put(shareRenderEntry{id: "oversized", hash: hash, bytes: shareRenderBytes + 1}, revision)
	if _, ok := cache.get("oversized", hash); ok {
		t.Fatal("oversized render was cached")
	}
	cache.put(shareRenderEntry{id: "large", hash: hash, bytes: shareRenderBytes}, revision)
	if cache.lru.Len() != 1 || cache.bytes != shareRenderBytes {
		t.Fatalf("byte limit not enforced: %d entries, %d bytes", cache.lru.Len(), cache.bytes)
	}
	cache.remove("large")
	cache.put(shareRenderEntry{id: "large", hash: hash, bytes: 1}, revision)
	if _, ok := cache.get("large", hash); ok {
		t.Fatal("revoked share was restored by a previous cache fill")
	}
}
