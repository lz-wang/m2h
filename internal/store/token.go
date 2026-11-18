package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrTokenNotFound marks a missing or inactive token row.
var ErrTokenNotFound = errors.New("token not found")

// TokenRecord is one persisted API token. SecretHash is the sha256 hex of
// the presented secret; the plaintext exists only at creation time.
type TokenRecord struct {
	ID           string
	SecretHash   string
	Name         string
	Kind         string
	OwnerID      string
	OwnerName    string
	ScopeJSON    string
	DocumentHash string
	CreatedAt    int64
	ExpiresAt    int64
	LastUsedAt   int64 // 0 = never
	RevokedAt    int64 // 0 = active
}

// Expired reports whether the token's lifetime is over.
func (record TokenRecord) Expired() bool {
	return record.ExpiresAt != 0 && record.ExpiresAt <= time.Now().UnixMilli()
}

// Active reports whether the token currently authorizes anything.
func (record TokenRecord) Active() bool {
	return record.RevokedAt == 0 && !record.Expired()
}

// CreateToken persists a new token.
func (store *Store) CreateToken(ctx context.Context, record *TokenRecord) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO api_tokens (
			id, secret_hash, name, kind, owner_id, owner_name, scope_json, document_hash,
			created_at, expires_at, last_used_at, revoked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL)`,
		id, record.SecretHash, record.Name, record.Kind, record.OwnerID, record.OwnerName,
		record.ScopeJSON, nullString(record.DocumentHash), now, record.ExpiresAt)
	if err != nil {
		return err
	}
	record.ID = id
	record.CreatedAt = now
	return nil
}

// ListTokens returns one owner's active tokens, newest first.
func (store *Store) ListTokens(ctx context.Context, ownerID string) ([]TokenRecord, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, secret_hash, name, kind, owner_id, owner_name, scope_json,
			COALESCE(document_hash, ''), created_at, expires_at,
			COALESCE(last_used_at, 0), COALESCE(revoked_at, 0)
		FROM api_tokens
		WHERE owner_id = ? AND revoked_at IS NULL AND (expires_at = 0 OR expires_at > ?)
		ORDER BY created_at DESC, id`, ownerID, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []TokenRecord
	for rows.Next() {
		record, scanErr := scanToken(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// CleanupTokens removes expired temporary tokens and revoked tokens after retention.
func (store *Store) CleanupTokens(ctx context.Context, before time.Time) error {
	_, err := store.db.ExecContext(ctx, `DELETE FROM api_tokens
		WHERE (kind = 'temporary' AND revoked_at IS NULL AND expires_at > 0 AND expires_at < ?) OR (revoked_at IS NOT NULL AND revoked_at < ?)`,
		before.UnixMilli(), before.UnixMilli())
	return err
}

// FindTokenByHash locates a token by its secret hash — the only lookup a
// presented bearer can drive. Revoked and expired tokens are returned as
// missing: an invalid credential is a 401, never a tombstone.
func (store *Store) FindTokenByHash(ctx context.Context, secretHash string) (TokenRecord, error) {
	row := store.db.QueryRowContext(ctx, `
		SELECT id, secret_hash, name, kind, owner_id, owner_name, scope_json,
			COALESCE(document_hash, ''), created_at, expires_at,
			COALESCE(last_used_at, 0), COALESCE(revoked_at, 0)
		FROM api_tokens
		WHERE secret_hash = ? AND revoked_at IS NULL`, secretHash)
	record, err := scanToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TokenRecord{}, ErrTokenNotFound
	}
	if err != nil {
		return TokenRecord{}, err
	}
	if record.Expired() {
		return TokenRecord{}, ErrTokenNotFound
	}
	return record, nil
}

// RevokeToken tombstones one of the owner's tokens.
func (store *Store) RevokeToken(ctx context.Context, id, ownerID string) (bool, error) {
	result, err := store.db.ExecContext(ctx,
		`UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND owner_id = ? AND revoked_at IS NULL`,
		time.Now().UnixMilli(), id, ownerID)
	if err != nil {
		return false, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return updated == 1, nil
}

// TouchToken stamps last_used_at, throttled by the caller (a per-request
// write would bury SQLite in bookkeeping).
func (store *Store) TouchToken(ctx context.Context, id string, when time.Time) error {
	_, err := store.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, when.UnixMilli(), id)
	return err
}

type tokenScanner interface {
	Scan(dest ...any) error
}

func scanToken(scanner tokenScanner) (TokenRecord, error) {
	var record TokenRecord
	err := scanner.Scan(
		&record.ID, &record.SecretHash, &record.Name, &record.Kind, &record.OwnerID,
		&record.OwnerName, &record.ScopeJSON, &record.DocumentHash,
		&record.CreatedAt, &record.ExpiresAt, &record.LastUsedAt, &record.RevokedAt)
	if err != nil {
		return TokenRecord{}, err
	}
	return record, nil
}
