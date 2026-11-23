package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// SessionCookieName is the browser cookie m2h's session layer uses.
const SessionCookieName = "m2h_session"

// SQLiteSessionStore persists scs sessions in m2h's SQLite database. Only
// the SHA-256 hash of the session token is stored — a leaked database file
// cannot be replayed as a valid cookie. It implements scs.Store.
type SQLiteSessionStore struct {
	db *sql.DB
}

// NewSQLiteSessionStore wraps an open database pool.
func NewSQLiteSessionStore(db *sql.DB) *SQLiteSessionStore {
	return &SQLiteSessionStore{db: db}
}

func hashSessionToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// Find returns the session blob for a token unless it is unknown or
// expired.
func (store *SQLiteSessionStore) Find(token string) ([]byte, bool, error) {
	var data []byte
	var expiresAt int64
	err := store.db.QueryRow(
		`SELECT data, expires_at FROM sessions WHERE id_hash = ?`, hashSessionToken(token),
	).Scan(&data, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if expiresAt <= time.Now().UnixMilli() {
		return nil, false, nil
	}
	return data, true, nil
}

// Commit upserts the session blob and expiry for a token, keeping the
// original creation timestamp on update.
func (store *SQLiteSessionStore) Commit(token string, data []byte, expiry time.Time) error {
	idHash := hashSessionToken(token)
	now := time.Now().UnixMilli()
	_, err := store.db.Exec(
		`INSERT INTO sessions (id_hash, data, created_at, expires_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id_hash) DO UPDATE SET data = excluded.data, expires_at = excluded.expires_at`,
		idHash, data, now, expiry.UnixMilli())
	return err
}

// Refresh extends an active session only when minInterval has elapsed since
// its last renewal. Zero expiry means the session is missing or expired.
func (store *SQLiteSessionStore) Refresh(ctx context.Context, token string, lifetime, minInterval time.Duration) (time.Time, bool, error) {
	now := time.Now()
	idHash := hashSessionToken(token)
	expiry, err := store.activeExpiry(ctx, idHash, now)
	if err != nil || expiry.IsZero() {
		return expiry, false, err
	}
	nextExpiry := now.Add(lifetime)
	threshold := nextExpiry.Add(-minInterval)
	if expiry.After(threshold) {
		return expiry, false, nil
	}

	var renewed int64
	err = store.db.QueryRowContext(ctx,
		`UPDATE sessions SET expires_at = ?
		 WHERE id_hash = ? AND expires_at > ? AND expires_at <= ? RETURNING expires_at`,
		nextExpiry.UnixMilli(), idHash, time.Now().UnixMilli(), threshold.UnixMilli(),
	).Scan(&renewed)
	if err == sql.ErrNoRows {
		// Another request renewed or removed the session after our read.
		expiry, err = store.activeExpiry(ctx, idHash, time.Now())
		return expiry, false, err
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("refresh session expiry: %w", err)
	}
	return time.UnixMilli(renewed), true, nil
}

func (store *SQLiteSessionStore) activeExpiry(ctx context.Context, idHash string, now time.Time) (time.Time, error) {
	var expiry int64
	err := store.db.QueryRowContext(ctx,
		`SELECT expires_at FROM sessions WHERE id_hash = ? AND expires_at > ?`,
		idHash, now.UnixMilli(),
	).Scan(&expiry)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read session expiry: %w", err)
	}
	return time.UnixMilli(expiry), nil
}

// Delete removes the session for a token; missing tokens are a no-op.
func (store *SQLiteSessionStore) Delete(token string) error {
	_, err := store.db.Exec(`DELETE FROM sessions WHERE id_hash = ?`, hashSessionToken(token))
	return err
}

// CleanupSessions drops expired rows. The authenticated server runs it on
// a slow ticker; nothing in the request path depends on it.
func (store *SQLiteSessionStore) CleanupSessions(ctx context.Context) error {
	_, err := store.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UnixMilli())
	return err
}
