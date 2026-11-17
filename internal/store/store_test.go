package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func openMigrated(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	return store
}

func TestOpenCreatesDirectoryAndDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "m2h")
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("data directory missing: %v", err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("data directory mode = %v, want 0700 dir", info.Mode().Perm())
	}
	database := filepath.Join(dir, DatabaseName)
	fileInfo, err := os.Stat(database)
	if err != nil {
		t.Fatalf("database file missing: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("database mode = %v, want 0600", fileInfo.Mode().Perm())
	}
}

func TestOpenRejectsEmptyDirectory(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatalf("Open(\"\") accepted")
	}
}

func TestMigrateCreatesSchemaAndIsIdempotent(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	for _, table := range []string{"sessions", "document_roots"} {
		var name string
		err := store.db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing after migration: %v", table, err)
		}
	}
	version, err := store.Version(ctx)
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}
	if version != latestVersion() {
		t.Fatalf("version = %d, want %d", version, latestVersion())
	}
	// Repeated startup re-migrates nothing and keeps the version stable.
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
	version, err = store.Version(ctx)
	if err != nil || version != latestVersion() {
		t.Fatalf("version after re-migrate = %d (err %v), want %d", version, err, latestVersion())
	}
}

func TestMigrateRefusesNewerSchema(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO schema_migrations (version) VALUES (?)`, latestVersion()+5); err != nil {
		t.Fatalf("seed future version failed: %v", err)
	}
	err := store.Migrate(ctx)
	if err == nil {
		t.Fatalf("newer schema accepted")
	}
	if want := "newer than this build supports"; !contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err, want)
	}
}

func TestMigrateRollsBackFailedStep(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	original := migrations
	supportedLatest := latestVersion()
	t.Cleanup(func() { migrations = original })
	migrations = append(slices.Clone(original), migration{
		version: supportedLatest + 1,
		statements: []string{
			`CREATE TABLE rollback_probe (id TEXT PRIMARY KEY)`,
			`THIS IS NOT VALID SQL`,
		},
	})

	if err := store.Migrate(context.Background()); err == nil {
		t.Fatalf("broken migration accepted")
	}
	var name string
	err = store.db.QueryRowContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'rollback_probe'`).Scan(&name)
	if err == sql.ErrNoRows {
		// expected: the transaction rolled back fully
	} else if err != nil {
		t.Fatalf("probe lookup failed: %v", err)
	} else {
		t.Fatalf("partial migration leaked table rollback_probe")
	}
	version, err := store.Version(context.Background())
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}
	if version != supportedLatest {
		t.Fatalf("version after rollback = %d, want %d", version, supportedLatest)
	}
}

func TestEnsureDocumentRootStableIdentity(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	first, err := store.EnsureDocumentRoot(ctx, "/srv/docs", RootKindDirectory)
	if err != nil {
		t.Fatalf("EnsureDocumentRoot failed: %v", err)
	}
	if !isValidUUID(first) {
		t.Fatalf("root id %q is not a uuid", first)
	}
	again, err := store.EnsureDocumentRoot(ctx, "/srv/docs", RootKindDirectory)
	if err != nil {
		t.Fatalf("second EnsureDocumentRoot failed: %v", err)
	}
	if first != again {
		t.Fatalf("identity changed: %q then %q", first, again)
	}
	other, err := store.EnsureDocumentRoot(ctx, "/srv/other", RootKindDirectory)
	if err != nil {
		t.Fatalf("EnsureDocumentRoot(other) failed: %v", err)
	}
	if other == first {
		t.Fatalf("different paths share identity %q", first)
	}

	// Restarting the process (a fresh Store on the same file) keeps ids.
	reopened, err := Open(filepath.Dir(store.dbPath(t)))
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer reopened.Close()
	if err := reopened.Migrate(ctx); err != nil {
		t.Fatalf("Migrate after reopen failed: %v", err)
	}
	persisted, err := reopened.EnsureDocumentRoot(ctx, "/srv/docs", RootKindDirectory)
	if err != nil {
		t.Fatalf("EnsureDocumentRoot after reopen failed: %v", err)
	}
	if persisted != first {
		t.Fatalf("identity did not survive restart: %q then %q", first, persisted)
	}
}

func TestEnsureDocumentRootRejectsInvalidInput(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if _, err := store.EnsureDocumentRoot(ctx, "", RootKindDirectory); err == nil {
		t.Fatalf("empty path accepted")
	}
	if _, err := store.EnsureDocumentRoot(ctx, "/srv/docs", "weird"); err == nil {
		t.Fatalf("invalid kind accepted")
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	store := openMigrated(t)
	var enabled int
	if err := store.db.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1 (per-connection DSN pragma missing)", enabled)
	}
}

func (store *Store) dbPath(t *testing.T) string {
	t.Helper()
	var sequence int
	var name, path string
	if err := store.db.QueryRowContext(context.Background(), `PRAGMA database_list`).Scan(&sequence, &name, &path); err != nil {
		t.Fatalf("read database path: %v", err)
	}
	return path
}

func isValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
				return false
			}
		}
	}
	return true
}

func contains(haystack, needle string) bool {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		if haystack[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}
