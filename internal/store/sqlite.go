// Package store owns m2h's optional SQLite persistence: connection
// management, schema migration and the data-access helpers the
// authenticated features (sessions, document identity, annotations, API
// tokens) build on. The default unauthenticated server never opens a
// database; every type here exists only for explicit --auth runs.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DatabaseName is the file the store keeps inside the data directory.
const DatabaseName = "m2h.db"

// Store wraps one SQLite database with the pragmas m2h relies on.
type Store struct {
	db *sql.DB
}

// dataSourceName builds the file DSN. Connection-scoped pragmas —
// busy_timeout and foreign_keys — ride along as DSN parameters so every
// pooled connection applies them; journal_mode=WAL is database-scoped but
// harmless to repeat per connection.
func dataSourceName(path string) string {
	// The path may contain characters DSN parsers treat specially (?,
	// #); percent-encode the filesystem path as a URL path first.
	return "file:" + url.PathEscape(path) +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)"
}

// Open prepares the data directory (mode 0700) and opens the SQLite
// database file (mode 0600) inside it. The returned Store has no schema
// yet — call Migrate.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, DatabaseName)
	db, err := sql.Open("sqlite", dataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	// A small bounded pool keeps concurrent writers within busy_timeout's
	// reach; m2h's write volume (sessions, annotations, tokens) never
	// needs more.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)
	// Touch the database so the file exists before the permission fixup.
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(name, 0o600); err != nil && !os.IsNotExist(err) {
			db.Close()
			return nil, fmt.Errorf("restrict permissions on %q: %w", name, err)
		}
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying connection pool for the data-access files in
// this package.
func (store *Store) DB() *sql.DB {
	return store.db
}

// Close releases the connection pool.
func (store *Store) Close() error {
	if store == nil || store.db == nil {
		return nil
	}
	return store.db.Close()
}
