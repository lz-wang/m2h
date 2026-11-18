package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lz-wang/m2h/internal/annotation"
)

// ErrAnnotationNotFound marks a missing annotation row.
var ErrAnnotationNotFound = errors.New("annotation not found")

// CreateAnnotation persists a new annotation, assigning its UUID and
// timestamps.
func (store *Store) CreateAnnotation(ctx context.Context, entry *annotation.Annotation) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	result, err := store.db.ExecContext(ctx, `
		INSERT INTO annotations (
			id, root_id, document_path, author_id, author_name,
			quote_exact, quote_prefix, quote_suffix,
			position_start, position_end, heading_id, tab_id, block_hint,
			document_hash, selector_version, comment, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, entry.RootID, entry.DocumentPath, entry.AuthorID, entry.AuthorName,
		entry.Selector.Quote.Exact, entry.Selector.Quote.Prefix, entry.Selector.Quote.Suffix,
		entry.Selector.Position.Start, entry.Selector.Position.End,
		nullString(entry.Selector.HeadingID), nullString(entry.Selector.TabID), nullString(entry.Selector.BlockHint),
		entry.DocumentHash, entry.Selector.Version, entry.Comment, now, now)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil || inserted != 1 {
		if err == nil {
			err = errors.New("annotation insert affected no rows")
		}
		return err
	}
	entry.ID = id
	entry.CreatedAt = now
	entry.UpdatedAt = now
	return nil
}

// ListAnnotations returns one document's annotations oldest-first.
func (store *Store) ListAnnotations(ctx context.Context, rootID, documentPath string) ([]annotation.Annotation, error) {
	rows, err := store.db.QueryContext(ctx, `
		SELECT id, root_id, document_path, author_id, author_name,
			quote_exact, quote_prefix, quote_suffix,
			position_start, position_end, heading_id, tab_id, block_hint,
			document_hash, selector_version, comment, created_at, updated_at
		FROM annotations
		WHERE root_id = ? AND document_path = ?
		ORDER BY created_at, rowid`, rootID, documentPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []annotation.Annotation
	for rows.Next() {
		entry, scanErr := scanAnnotation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetAnnotation fetches one annotation by id.
func (store *Store) GetAnnotation(ctx context.Context, id string) (annotation.Annotation, error) {
	row := store.db.QueryRowContext(ctx, `
		SELECT id, root_id, document_path, author_id, author_name,
			quote_exact, quote_prefix, quote_suffix,
			position_start, position_end, heading_id, tab_id, block_hint,
			document_hash, selector_version, comment, created_at, updated_at
		FROM annotations WHERE id = ?`, id)
	entry, err := scanAnnotation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return annotation.Annotation{}, ErrAnnotationNotFound
	}
	return entry, err
}

// DeleteAnnotation removes an annotation, but only when authorID owns it.
// The returned found flag distinguishes a missing row from a foreign one.
func (store *Store) DeleteAnnotation(ctx context.Context, id, authorID string) (found bool, err error) {
	entry, err := store.GetAnnotation(ctx, id)
	if err != nil {
		return false, err
	}
	if entry.AuthorID != authorID {
		return true, nil
	}
	_, err = store.db.ExecContext(ctx, `DELETE FROM annotations WHERE id = ?`, id)
	return true, err
}

// UpdateAnnotationSelector re-anchors an annotation to new selector data —
// the human-confirmed repair path after a document edit. The author
// condition rides the UPDATE itself, so authorization and the write are
// one database-level fact even if the entry changed between the API
// layer's check and this statement.
func (store *Store) UpdateAnnotationSelector(ctx context.Context, id, authorID string, selector annotation.Selector, documentHash string) error {
	result, err := store.db.ExecContext(ctx, `
		UPDATE annotations SET
			quote_exact = ?, quote_prefix = ?, quote_suffix = ?,
			position_start = ?, position_end = ?,
			heading_id = ?, tab_id = ?, block_hint = ?,
			document_hash = ?, selector_version = ?, updated_at = ?
		WHERE id = ? AND author_id = ?`,
		selector.Quote.Exact, selector.Quote.Prefix, selector.Quote.Suffix,
		selector.Position.Start, selector.Position.End,
		nullString(selector.HeadingID), nullString(selector.TabID), nullString(selector.BlockHint),
		documentHash, selector.Version, time.Now().UnixMilli(), id, authorID)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return ErrAnnotationNotFound
	}
	return nil
}

// UpdateAnnotationComment updates the author's comment.
func (store *Store) UpdateAnnotationComment(ctx context.Context, id, authorID, comment string) error {
	result, err := store.db.ExecContext(ctx, `UPDATE annotations SET comment = ?, updated_at = ? WHERE id = ? AND author_id = ?`, comment, time.Now().UnixMilli(), id, authorID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrAnnotationNotFound
	}
	return nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type annotationScanner interface {
	Scan(dest ...any) error
}

func scanAnnotation(scanner annotationScanner) (annotation.Annotation, error) {
	var entry annotation.Annotation
	var headingID, tabID, blockHint sql.NullString
	err := scanner.Scan(
		&entry.ID, &entry.RootID, &entry.DocumentPath, &entry.AuthorID, &entry.AuthorName,
		&entry.Selector.Quote.Exact, &entry.Selector.Quote.Prefix, &entry.Selector.Quote.Suffix,
		&entry.Selector.Position.Start, &entry.Selector.Position.End,
		&headingID, &tabID, &blockHint,
		&entry.DocumentHash, &entry.Selector.Version, &entry.Comment,
		&entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return annotation.Annotation{}, err
	}
	entry.Selector.HeadingID = headingID.String
	entry.Selector.TabID = tabID.String
	entry.Selector.BlockHint = blockHint.String
	return entry, nil
}
