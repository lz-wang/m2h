package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lz-wang/m2h/internal/annotation"
	"github.com/lz-wang/m2h/internal/auth"
)

func tokenFixture(owner string, expiresIn time.Duration) storeTokenFixture {
	secret, hash, _ := auth.GenerateTokenSecret()
	scope, _ := auth.MarshalScope(func() auth.TokenScope {
		var scope auth.TokenScope
		scope.Permissions = []string{auth.PermissionDocumentRead}
		scope.Resource.Type = auth.ResourceAllPublished
		return scope
	}())
	return storeTokenFixture{
		record: TokenRecord{
			SecretHash: hash,
			Name:       "llm-agent",
			Kind:       string(auth.TokenPersistent),
			OwnerID:    owner,
			ScopeJSON:  scope,
			ExpiresAt:  time.Now().Add(expiresIn).UnixMilli(),
		},
		secret: secret,
		hash:   hash,
	}
}

type storeTokenFixture struct {
	record TokenRecord
	secret string
	hash   string
}

// TestTokenOwnerNameSurvivesRoundTrip pins the column authorization
// re-checks read: creation persists it, lookups return it.
func TestTokenOwnerNameSurvivesRoundTrip(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()

	fixture := tokenFixture("local:alice", time.Hour)
	fixture.record.OwnerName = "alice"
	if err := dataStore.CreateToken(ctx, &fixture.record); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	found, err := dataStore.FindTokenByHash(ctx, fixture.hash)
	if err != nil {
		t.Fatalf("FindTokenByHash failed: %v", err)
	}
	if found.OwnerName != "alice" {
		t.Fatalf("owner name = %q, want %q", found.OwnerName, "alice")
	}

	listed, err := dataStore.ListTokens(ctx, fixture.record.OwnerID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListTokens = %v err %v", listed, err)
	}
	if listed[0].OwnerName != "alice" {
		t.Fatalf("listed owner name = %q", listed[0].OwnerName)
	}
}

// TestTokenOwnerNameBackfilledFromV3 upgrades a schema-v3 database (tokens
// without the owner_name column) and checks the backfill: local ids are
// reversible into usernames, LDAP uuid ids are not and must keep the empty
// fail-closed marker.
func TestTokenOwnerNameBackfilledFromV3(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "m2h.db"))
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`,
		`INSERT INTO schema_migrations (version) VALUES (3)`,
		`CREATE TABLE api_tokens (
			id TEXT PRIMARY KEY,
			secret_hash TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			scope_json TEXT NOT NULL,
			document_hash TEXT,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			last_used_at INTEGER,
			revoked_at INTEGER
		)`,
		`INSERT INTO api_tokens (id, secret_hash, name, kind, owner_id, scope_json,
			created_at, expires_at)
			VALUES ('t1', 'hash-1', 'a', 'persistent', 'local:alice', '{}', 1, 99999999999999)`,
		`INSERT INTO api_tokens (id, secret_hash, name, kind, owner_id, scope_json,
			created_at, expires_at)
			VALUES ('t2', 'hash-2', 'b', 'persistent', 'ldap:5f0c8e40-uuid', '{}', 1, 99999999999999)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed v3 database: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	dataStore, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer dataStore.Close()
	if err := dataStore.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	local, err := dataStore.FindTokenByHash(context.Background(), "hash-1")
	if err != nil {
		t.Fatalf("local token = %v", err)
	}
	if local.OwnerName != "alice" {
		t.Fatalf("local owner name = %q, want backfilled %q", local.OwnerName, "alice")
	}
	ldap, err := dataStore.FindTokenByHash(context.Background(), "hash-2")
	if err != nil {
		t.Fatalf("ldap token = %v", err)
	}
	if ldap.OwnerName != "" {
		t.Fatalf("irreversible ldap id must keep the empty fail-closed marker, got %q", ldap.OwnerName)
	}
}

// TestVacuumIntoBackupRestore validates the consistency backup the
// deployment documentation prescribes: VACUUM INTO produces a standalone
// snapshot even while the source database runs in WAL mode, and the
// snapshot opens as a fully working store.
func TestVacuumIntoBackupRestore(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()

	fixture := tokenFixture("local:alice", time.Hour)
	fixture.record.OwnerName = "alice"
	if err := dataStore.CreateToken(ctx, &fixture.record); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	rootID, err := dataStore.EnsureDocumentRoot(ctx, "/tmp/m2h-backup-docs", RootKindDirectory)
	if err != nil {
		t.Fatalf("EnsureDocumentRoot failed: %v", err)
	}
	entry := annotation.Annotation{
		RootID: rootID, DocumentPath: "guide.md",
		AuthorID: "local:alice", AuthorName: "alice",
		Selector: annotation.Selector{
			Quote:    annotation.QuoteSelector{Exact: "Body text"},
			Position: annotation.PositionSelector{Start: 10, End: 19},
			Version:  1,
		},
		DocumentHash: "sha256:abc",
	}
	if err := dataStore.CreateAnnotation(ctx, &entry); err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}

	backupDir := t.TempDir()
	if _, err := dataStore.DB().ExecContext(ctx, `VACUUM INTO ?`, filepath.Join(backupDir, "m2h.db")); err != nil {
		t.Fatalf("VACUUM INTO failed: %v", err)
	}

	restored, err := Open(backupDir)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer restored.Close()
	tokens, err := restored.ListTokens(ctx, "local:alice")
	if err != nil || len(tokens) != 1 || tokens[0].OwnerName != "alice" {
		t.Fatalf("restored tokens = %+v err %v", tokens, err)
	}
	annotations, err := restored.ListAnnotations(ctx, rootID, "guide.md")
	if err != nil || len(annotations) != 1 || annotations[0].Selector.Quote.Exact != "Body text" {
		t.Fatalf("restored annotations = %+v err %v", annotations, err)
	}
	// The snapshot is a complete database, not a page-copy needing
	// recovery: it reports the live schema version.
	version, err := restored.Version(ctx)
	if err != nil || version != latestVersion() {
		t.Fatalf("restored version = %d err %v, want %d", version, err, latestVersion())
	}
}

func TestTokenLifecycle(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()

	fixture := tokenFixture("local:alice", time.Hour)
	if err := dataStore.CreateToken(ctx, &fixture.record); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if fixture.record.ID == "" || fixture.record.CreatedAt == 0 {
		t.Fatalf("create did not fill id/timestamps")
	}

	found, err := dataStore.FindTokenByHash(ctx, fixture.hash)
	if err != nil {
		t.Fatalf("FindTokenByHash failed: %v", err)
	}
	if found.ID != fixture.record.ID || found.OwnerID != "local:alice" {
		t.Fatalf("found = %+v", found)
	}
	if !found.Active() {
		t.Fatalf("fresh token must be active")
	}

	listed, err := dataStore.ListTokens(ctx, "local:alice")
	if err != nil || len(listed) != 1 || listed[0].ID != fixture.record.ID {
		t.Fatalf("ListTokens = %v (err %v)", listed, err)
	}
	foreign, err := dataStore.ListTokens(ctx, "local:bob")
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign listing = %v (err %v)", foreign, err)
	}

	if err := dataStore.TouchToken(ctx, fixture.record.ID, time.Now()); err != nil {
		t.Fatalf("TouchToken failed: %v", err)
	}
	touched, _ := dataStore.FindTokenByHash(ctx, fixture.hash)
	if touched.LastUsedAt == 0 {
		t.Fatalf("last_used_at not stamped")
	}

	// A foreign owner cannot revoke.
	if revoked, err := dataStore.RevokeToken(ctx, fixture.record.ID, "local:bob"); err != nil || revoked {
		t.Fatalf("foreign revoke = %v %v", revoked, err)
	}
	if _, err := dataStore.RevokeToken(ctx, fixture.record.ID, "local:alice"); err != nil {
		t.Fatalf("owner revoke failed: %v", err)
	}
	if _, err := dataStore.FindTokenByHash(ctx, fixture.hash); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("revoked token still found: %v", err)
	}
	if listed, _ := dataStore.ListTokens(ctx, "local:alice"); len(listed) != 0 {
		t.Fatalf("revoked token still listed")
	}
}

func TestExpiredTokensAreMissing(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()

	fixture := tokenFixture("local:alice", -time.Minute)
	if err := dataStore.CreateToken(ctx, &fixture.record); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if _, err := dataStore.FindTokenByHash(ctx, fixture.hash); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expired token found: %v", err)
	}
}

func TestTokenCleanupRetention(t *testing.T) {
	dataStore := openMigrated(t)
	ctx := context.Background()
	old := tokenFixture("local:alice", -8*24*time.Hour)
	old.record.Kind = string(auth.TokenTemporary)
	recent := tokenFixture("local:alice", -time.Hour)
	recent.record.Kind = string(auth.TokenTemporary)
	active := tokenFixture("local:alice", time.Hour)
	for _, entry := range []*storeTokenFixture{&old, &recent, &active} {
		if err := dataStore.CreateToken(ctx, &entry.record); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := dataStore.ListTokens(ctx, "local:alice")
	if err != nil || len(listed) != 1 || listed[0].ID != active.record.ID {
		t.Fatalf("active list = %v, %v", listed, err)
	}
	if err := dataStore.CleanupTokens(ctx, time.Now().Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := dataStore.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM api_tokens").Scan(&count); err != nil || count != 2 {
		t.Fatalf("remaining = %d, %v", count, err)
	}
}

func TestUnknownTokenHashIsMissing(t *testing.T) {
	dataStore := openMigrated(t)
	if _, err := dataStore.FindTokenByHash(context.Background(), "deadbeef"); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("unknown hash = %v", err)
	}
}
