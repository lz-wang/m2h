package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// RootKind records what a workspace root is: a directory tree ("directory")
// or one explicit Markdown file ("file"). It is stored with the identity so
// future tooling can tell them apart.
const (
	RootKindDirectory = "directory"
	RootKindFile      = "file"
)

// DocumentRoot is one persisted workspace-root identity.
type DocumentRoot struct {
	ID            string
	CanonicalPath string
	Kind          string
	CreatedAt     int64
}

// newUUID returns a random RFC 4122 version-4 UUID in canonical text form.
func newUUID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}

// EnsureDocumentRoot returns the stable root id for canonicalPath, creating
// the identity on first sight. Identity follows the canonical filesystem
// path — reordering CLI inputs, or re-creating a deleted file, maps back to
// the same id — while a moved or renamed root deliberately starts a new
// identity rather than guessing from content.
func (store *Store) EnsureDocumentRoot(ctx context.Context, canonicalPath, kind string) (string, error) {
	if canonicalPath == "" {
		return "", fmt.Errorf("canonical path is required")
	}
	if kind != RootKindDirectory && kind != RootKindFile {
		return "", fmt.Errorf("invalid root kind %q", kind)
	}
	var id string
	err := store.queryDocumentRoot(ctx, canonicalPath, &id)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, sql.ErrNoRows):
		// fall through to insert
	default:
		return "", fmt.Errorf("look up document root %q: %w", canonicalPath, err)
	}

	id, err = newUUID()
	if err != nil {
		return "", err
	}
	_, err = store.db.ExecContext(ctx,
		`INSERT INTO document_roots (id, canonical_path, root_kind, created_at) VALUES (?, ?, ?, ?)`,
		id, canonicalPath, kind, time.Now().UnixMilli())
	if err != nil {
		// A concurrent creator may have won the unique race; adopt its row.
		var existing string
		if lookupErr := store.queryDocumentRoot(ctx, canonicalPath, &existing); lookupErr == nil {
			return existing, nil
		}
		return "", fmt.Errorf("register document root %q: %w", canonicalPath, err)
	}
	return id, nil
}

func (store *Store) queryDocumentRoot(ctx context.Context, canonicalPath string, id *string) error {
	return store.db.QueryRowContext(ctx,
		`SELECT id FROM document_roots WHERE canonical_path = ?`, canonicalPath).Scan(id)
}
