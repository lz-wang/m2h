package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Share struct {
	ID           string
	OwnerID      string
	OwnerName    string
	RootID       string
	DocumentPath string
	CreatedAt    int64
	ExpiresAt    int64
}

const shareColumns = "id, owner_id, owner_name, root_id, document_path, created_at, expires_at"

func scanShare(row interface{ Scan(...any) error }) (Share, error) {
	var share Share
	err := row.Scan(&share.ID, &share.OwnerID, &share.OwnerName, &share.RootID,
		&share.DocumentPath, &share.CreatedAt, &share.ExpiresAt)
	return share, err
}

// CreateShare keeps an existing active link, including its original expiry.
// A single SQLite statement serializes replacement of expired links.
func (store *Store) CreateShare(ctx context.Context, candidate Share) (Share, error) {
	share, err := scanShare(store.db.QueryRowContext(ctx, `
		INSERT INTO document_shares (`+shareColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (owner_id, root_id, document_path) DO UPDATE SET
			id = iif(expires_at != 0 AND expires_at <= excluded.created_at, excluded.id, id),
			owner_name = excluded.owner_name,
			created_at = iif(expires_at != 0 AND expires_at <= excluded.created_at, excluded.created_at, created_at),
			expires_at = iif(expires_at != 0 AND expires_at <= excluded.created_at, excluded.expires_at, expires_at)
		RETURNING `+shareColumns, candidate.ID, candidate.OwnerID, candidate.OwnerName,
		candidate.RootID, candidate.DocumentPath, candidate.CreatedAt, candidate.ExpiresAt))
	if err != nil {
		return Share{}, fmt.Errorf("create document share: %w", err)
	}
	return share, nil
}

func (store *Store) FindShare(ctx context.Context, id string, now time.Time) (Share, error) {
	share, err := scanShare(store.db.QueryRowContext(ctx, `SELECT `+shareColumns+`
		FROM document_shares WHERE id = ? AND (expires_at = 0 OR expires_at > ?)`, id, now.UnixMilli()))
	if err != nil {
		return Share{}, fmt.Errorf("find document share: %w", err)
	}
	return share, nil
}

func (store *Store) FindDocumentShare(ctx context.Context, ownerID, rootID, path string, now time.Time) (Share, error) {
	share, err := scanShare(store.db.QueryRowContext(ctx, `SELECT `+shareColumns+`
		FROM document_shares WHERE owner_id = ? AND root_id = ? AND document_path = ?
		AND (expires_at = 0 OR expires_at > ?)`, ownerID, rootID, path, now.UnixMilli()))
	if err != nil {
		return Share{}, fmt.Errorf("find share for document: %w", err)
	}
	return share, nil
}

func (store *Store) ListShares(ctx context.Context, ownerID string, now time.Time) ([]Share, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT `+shareColumns+` FROM document_shares
		WHERE (? = '' OR owner_id = ?) AND (expires_at = 0 OR expires_at > ?)
		ORDER BY created_at DESC, id`, ownerID, ownerID, now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("list document shares: %w", err)
	}
	defer rows.Close()
	shares := []Share{}
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document share: %w", err)
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document shares: %w", err)
	}
	return shares, nil
}

func (store *Store) RevokeShare(ctx context.Context, ownerID, id string) error {
	var removed string
	err := store.db.QueryRowContext(ctx, `DELETE FROM document_shares
		WHERE owner_id = ? AND id = ? RETURNING id`, ownerID, id).Scan(&removed)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return fmt.Errorf("revoke document share: %w", err)
	}
	return nil
}

func (store *Store) CleanupShares(ctx context.Context, now time.Time) error {
	if _, err := store.db.ExecContext(ctx, `DELETE FROM document_shares
		WHERE expires_at != 0 AND expires_at <= ?`, now.UnixMilli()); err != nil {
		return fmt.Errorf("clean up expired document shares: %w", err)
	}
	return nil
}
