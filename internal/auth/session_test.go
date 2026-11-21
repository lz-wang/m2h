package auth

import (
	"context"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/store"
)

func openSessionStore(t *testing.T) *SQLiteSessionStore {
	t.Helper()
	dataStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	return NewSQLiteSessionStore(dataStore.DB())
}

func TestSessionStoreCommitFindDelete(t *testing.T) {
	sessionStore := openSessionStore(t)

	if _, found, err := sessionStore.Find("unknown-token"); found || err != nil {
		t.Fatalf("find unknown = %v %v, want not found", found, err)
	}
	if err := sessionStore.Commit("token-1", []byte("payload"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	data, found, err := sessionStore.Find("token-1")
	if err != nil || !found || string(data) != "payload" {
		t.Fatalf("find = %q %v %v", data, found, err)
	}

	// Update overwrites the payload.
	if err := sessionStore.Commit("token-1", []byte("updated"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("recommit failed: %v", err)
	}
	data, _, _ = sessionStore.Find("token-1")
	if string(data) != "updated" {
		t.Fatalf("payload after recommit = %q", data)
	}

	if err := sessionStore.Delete("token-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, found, _ := sessionStore.Find("token-1"); found {
		t.Fatalf("deleted session still found")
	}
	if expiry, err := sessionStore.Refresh(context.Background(), "token-1", time.Hour); err != nil || !expiry.IsZero() {
		t.Fatalf("refresh deleted session = %v, %v", expiry, err)
	}
	// Deleting again is a no-op.
	if err := sessionStore.Delete("token-1"); err != nil {
		t.Fatalf("delete again failed: %v", err)
	}
}

func TestSessionStoreExpiredSessionsNotServed(t *testing.T) {
	sessionStore := openSessionStore(t)
	if err := sessionStore.Commit("stale", []byte("old"), time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if _, found, err := sessionStore.Find("stale"); found || err != nil {
		t.Fatalf("expired session served: %v %v", found, err)
	}
	if expiry, err := sessionStore.Refresh(context.Background(), "stale", time.Hour); err != nil || !expiry.IsZero() {
		t.Fatalf("refresh expired session = %v, %v", expiry, err)
	}
	if err := sessionStore.CleanupSessions(context.Background()); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func TestSessionStoreStoresOnlyTokenHashes(t *testing.T) {
	sessionStore := openSessionStore(t)
	if err := sessionStore.Commit("secret-token", []byte("x"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	query, err := sessionStore.db.Query("SELECT id_hash FROM sessions")
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	defer query.Close()
	var keys []string
	for query.Next() {
		var hash string
		if scanErr := query.Scan(&hash); scanErr != nil {
			t.Fatalf("scan: %v", scanErr)
		}
		keys = append(keys, hash)
	}
	if err := query.Err(); err != nil {
		t.Fatalf("iterate sessions: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("rows = %v, want one session", keys)
	}
	if keys[0] == "secret-token" {
		t.Fatalf("raw token stored as key")
	}
	if len(keys[0]) != 64 { // sha256 hex digest length
		t.Fatalf("key %q is not a sha256 hex digest", keys[0])
	}
}

func TestLoginLimiterLocksAfterFailures(t *testing.T) {
	now := time.Now()
	limiter := NewLoginLimiter(3, 10*time.Minute)
	limiter.now = func() time.Time { return now }

	key := "1.2.3.4|alice"
	for i := range 3 {
		if !limiter.Allowed(key) {
			t.Fatalf("blocked before %d failures", i+1)
		}
		limiter.RecordFailure(key)
	}
	if limiter.Allowed(key) {
		t.Fatalf("allowed after 3 failures")
	}

	// Another key is unaffected.
	if !limiter.Allowed("5.6.7.8|alice") {
		t.Fatalf("unrelated key blocked")
	}

	// The window slides: old failures expire.
	now = now.Add(11 * time.Minute)
	if !limiter.Allowed(key) {
		t.Fatalf("failures did not expire with the window")
	}

	// Success resets.
	limiter.RecordFailure(key)
	limiter.Reset(key)
	if !limiter.Allowed(key) {
		t.Fatalf("reset did not clear failures")
	}
}
