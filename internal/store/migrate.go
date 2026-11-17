package store

import (
	"context"
	"fmt"
)

// migration is one schema step. All statements of a step apply inside a
// single transaction together with the version bookkeeping, so a database
// is always at exactly one known version.
type migration struct {
	version    int
	statements []string
}

// migrations lists every schema step in order. Version N must only depend
// on the schema left behind by version N-1.
//
// Ruling (recorded in the plan ledger): the plan's sessions table carried
// principal_id/provider columns, but the session library's store contract
// only commits (token, blob, expiry); the principal and provider travel
// inside the signed session payload and are enforced by the auth
// middleware instead, so the columns are dropped from v1.
var migrations = []migration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE sessions (
				id_hash TEXT PRIMARY KEY,
				data BLOB NOT NULL,
				created_at INTEGER NOT NULL,
				expires_at INTEGER NOT NULL
			)`,
			`CREATE TABLE document_roots (
				id TEXT PRIMARY KEY,
				canonical_path TEXT NOT NULL UNIQUE,
				root_kind TEXT NOT NULL,
				created_at INTEGER NOT NULL
			)`,
		},
	},
}

// latestVersion is the highest schema version this build understands.
func latestVersion() int {
	return migrations[len(migrations)-1].version
}

// Version reports the database's current schema version (0 for a fresh
// database with no migrations applied).
func (store *Store) Version(ctx context.Context) (int, error) {
	if _, err := store.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY
	)`); err != nil {
		return 0, fmt.Errorf("prepare schema migrations table: %w", err)
	}
	var version int
	if err := store.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

// Migrate brings the database up to the latest schema version, applying
// each pending step in its own transaction. A database created by a newer
// build is refused rather than downgraded.
func (store *Store) Migrate(ctx context.Context) error {
	current, err := store.Version(ctx)
	if err != nil {
		return err
	}
	latest := latestVersion()
	if current > latest {
		return fmt.Errorf("database schema version %d is newer than this build supports (%d); upgrade m2h", current, latest)
	}
	for _, step := range migrations {
		if step.version <= current {
			continue
		}
		if err := store.apply(ctx, step); err != nil {
			return fmt.Errorf("migrate to schema version %d: %w", step.version, err)
		}
	}
	return nil
}

func (store *Store) apply(ctx context.Context, step migration) error {
	transaction, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	for _, statement := range step.statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply %q: %w", summarize(statement), err)
		}
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, step.version); err != nil {
		return err
	}
	return transaction.Commit()
}

func summarize(statement string) string {
	if len(statement) > 40 {
		return statement[:40]
	}
	return statement
}
