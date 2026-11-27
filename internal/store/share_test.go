package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestShareLifecycleAndConcurrentCreation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dataStore := openMigrated(t)
	root, err := dataStore.EnsureDocumentRoot(ctx, t.TempDir(), "directory")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	candidate := Share{ID: "first", OwnerID: "local:alice", OwnerName: "alice",
		RootID: root, DocumentPath: "guide.md", CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(time.Hour).UnixMilli()}
	results := make(chan Share, 8)
	var workers sync.WaitGroup
	for i := range cap(results) {
		workers.Go(func() {
			record := candidate
			record.ID = fmt.Sprintf("concurrent-%d", i)
			created, err := dataStore.CreateShare(ctx, record)
			if err != nil {
				t.Error(err)
				return
			}
			results <- created
		})
	}
	workers.Wait()
	close(results)
	var active Share
	for record := range results {
		if active.ID != "" && active != record {
			t.Fatalf("concurrent creation returned different records: %+v and %+v", active, record)
		}
		active = record
	}
	if active.ID == "" || active.ExpiresAt != candidate.ExpiresAt {
		t.Fatalf("invalid active record: %+v", active)
	}
	if _, err := dataStore.FindShare(ctx, active.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.RevokeShare(ctx, "local:bob", active.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("other owner revoked share: %v", err)
	}
	next := time.UnixMilli(active.ExpiresAt)
	if _, err := dataStore.FindShare(ctx, active.ID, next); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired share lookup: %v", err)
	}
	items, err := dataStore.ListShares(ctx, active.OwnerID, next)
	if err != nil || len(items) != 0 {
		t.Fatalf("expired share list: %+v, %v", items, err)
	}
	candidate.CreatedAt, candidate.ExpiresAt = next.UnixMilli(), 0
	replacement, err := dataStore.CreateShare(ctx, candidate)
	if err != nil || replacement != candidate {
		t.Fatalf("replace expired share: %+v, %v", replacement, err)
	}
	candidate.ID, candidate.DocumentPath, candidate.ExpiresAt = "expired", "other.md", next.UnixMilli()
	if _, err := dataStore.CreateShare(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.CleanupShares(ctx, next); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.RevokeShare(ctx, candidate.OwnerID, candidate.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cleanup retained expired record: %v", err)
	}
	if err := dataStore.RevokeShare(ctx, replacement.OwnerID, replacement.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dataStore.FindShare(ctx, replacement.ID, next); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("revoked share lookup: %v", err)
	}
}
