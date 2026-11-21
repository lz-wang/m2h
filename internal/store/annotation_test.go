package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lz-wang/m2h/internal/annotation"
)

func openAnnotated(t *testing.T) (*Store, string) {
	t.Helper()
	dataStore := openMigrated(t)
	rootID, err := dataStore.EnsureDocumentRoot(context.Background(), t.TempDir(), RootKindDirectory)
	if err != nil {
		t.Fatalf("EnsureDocumentRoot failed: %v", err)
	}
	return dataStore, rootID
}

func sampleAnnotation(rootID, comment string) annotation.Annotation {
	return annotation.Annotation{
		RootID:       rootID,
		DocumentPath: "design.md",
		AuthorID:     "local:alice",
		AuthorName:   "alice",
		Selector: annotation.Selector{
			Quote:     annotation.QuoteSelector{Exact: "使用 SQLite", Prefix: "建议", Suffix: "作为存储"},
			Position:  annotation.PositionSelector{Start: 120, End: 129},
			HeadingID: "数据存储",
			Version:   1,
		},
		Comment:      comment,
		DocumentHash: "sha256:abc",
	}
}

func TestAnnotationRoundTrip(t *testing.T) {
	dataStore, rootID := openAnnotated(t)
	ctx := context.Background()

	entry := sampleAnnotation(rootID, "需要补充备份方案")
	entry.Color = annotation.ColorAmber
	if err := dataStore.CreateAnnotation(ctx, &entry); err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}
	if entry.ID == "" || entry.CreatedAt == 0 || entry.UpdatedAt == 0 {
		t.Fatalf("create did not fill id/timestamps: %+v", entry)
	}

	loaded, err := dataStore.GetAnnotation(ctx, entry.ID)
	if err != nil {
		t.Fatalf("GetAnnotation failed: %v", err)
	}
	if loaded.Comment != "需要补充备份方案" ||
		loaded.Selector.Quote.Exact != "使用 SQLite" ||
		loaded.Selector.Quote.Prefix != "建议" ||
		loaded.Selector.Quote.Suffix != "作为存储" ||
		loaded.Selector.Position.Start != 120 || loaded.Selector.Position.End != 129 ||
		loaded.Selector.HeadingID != "数据存储" ||
		loaded.AuthorID != "local:alice" || loaded.AuthorName != "alice" ||
		loaded.DocumentHash != "sha256:abc" || loaded.Color != annotation.ColorAmber {
		t.Fatalf("round trip lost data: %+v", loaded)
	}

	listed, err := dataStore.ListAnnotations(ctx, rootID, "design.md")
	if err != nil || len(listed) != 1 || listed[0].ID != entry.ID {
		t.Fatalf("ListAnnotations = %v (err %v)", listed, err)
	}
	// Another document's annotations stay invisible.
	other, err := dataStore.ListAnnotations(ctx, rootID, "other.md")
	if err != nil || len(other) != 0 {
		t.Fatalf("other document listed = %v (err %v)", other, err)
	}
}

func TestAnnotationColorDefaultsOnV4Upgrade(t *testing.T) {
	ctx := context.Background()
	dataStore, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dataStore.Close()
	if _, err := dataStore.Version(ctx); err != nil {
		t.Fatal(err)
	}
	for _, step := range migrations[:4] {
		if err := dataStore.apply(ctx, step); err != nil {
			t.Fatal(err)
		}
	}
	rootID, err := dataStore.EnsureDocumentRoot(ctx, t.TempDir(), RootKindDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dataStore.db.ExecContext(ctx, `
		INSERT INTO annotations (
			id, root_id, document_path, author_id, author_name, quote_exact,
			position_start, position_end, document_hash, comment, created_at, updated_at
		) VALUES ('old', ?, 'design.md', 'local:alice', 'alice', 'x', 0, 1, 'sha256:abc', 'old comment', 1, 2)
	`, rootID); err != nil {
		t.Fatal(err)
	}
	if err := dataStore.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	entry, err := dataStore.GetAnnotation(ctx, "old")
	if err != nil || entry.Color != annotation.ColorBlue || entry.Comment != "old comment" || entry.UpdatedAt != 2 {
		t.Fatalf("migrated annotation = %+v, %v", entry, err)
	}
}

func TestUpdateAnnotationComment(t *testing.T) {
	dataStore, rootID := openAnnotated(t)
	ctx := context.Background()
	entry := sampleAnnotation(rootID, "原评论")
	entry.Color = annotation.ColorAmber
	if err := dataStore.CreateAnnotation(ctx, &entry); err != nil {
		t.Fatal(err)
	}
	green := annotation.ColorGreen
	if err := dataStore.UpdateAnnotationComment(ctx, entry.ID, "local:bob", "无权修改", &green); !errors.Is(err, ErrAnnotationNotFound) {
		t.Fatalf("foreign edit: %v", err)
	}
	if err := dataStore.UpdateAnnotationComment(ctx, entry.ID, entry.AuthorID, "更新评论", nil); err != nil {
		t.Fatal(err)
	}
	updated, err := dataStore.GetAnnotation(ctx, entry.ID)
	if err != nil || updated.Comment != "更新评论" || updated.Color != annotation.ColorAmber {
		t.Fatalf("updated = %+v, %v", updated, err)
	}
	if err := dataStore.UpdateAnnotationComment(ctx, "missing", entry.AuthorID, "x", &green); !errors.Is(err, ErrAnnotationNotFound) {
		t.Fatalf("missing edit: %v", err)
	}
}

func TestAnnotationListOrdersOldestFirst(t *testing.T) {
	dataStore, rootID := openAnnotated(t)
	ctx := context.Background()
	ids := []string{}
	for index := range 3 {
		entry := sampleAnnotation(rootID, strings.Repeat("c", index+1))
		if err := dataStore.CreateAnnotation(ctx, &entry); err != nil {
			t.Fatalf("CreateAnnotation failed: %v", err)
		}
		ids = append(ids, entry.ID)
	}
	listed, err := dataStore.ListAnnotations(ctx, rootID, "design.md")
	if err != nil {
		t.Fatalf("ListAnnotations failed: %v", err)
	}
	if len(listed) != 3 || listed[0].ID != ids[0] || listed[2].ID != ids[2] {
		t.Fatalf("order = %v, want %v", listedIDs(listed), ids)
	}
}

func TestDeleteAnnotationAuthorGate(t *testing.T) {
	dataStore, rootID := openAnnotated(t)
	ctx := context.Background()
	entry := sampleAnnotation(rootID, "mine")
	if err := dataStore.CreateAnnotation(ctx, &entry); err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}

	if _, err := dataStore.DeleteAnnotation(ctx, entry.ID, "local:bob"); err != nil {
		t.Fatalf("foreign delete errored: %v", err)
	}
	still, err := dataStore.GetAnnotation(ctx, entry.ID)
	if err != nil || still.ID != entry.ID {
		t.Fatalf("foreign delete removed the row: %+v %v", still, err)
	}

	if _, err := dataStore.DeleteAnnotation(ctx, entry.ID, "local:alice"); err != nil {
		t.Fatalf("author delete failed: %v", err)
	}
	if _, err := dataStore.GetAnnotation(ctx, entry.ID); !errors.Is(err, ErrAnnotationNotFound) {
		t.Fatalf("author delete left the row: %v", err)
	}

	if _, err := dataStore.DeleteAnnotation(ctx, "missing", "local:alice"); !errors.Is(err, ErrAnnotationNotFound) {
		t.Fatalf("missing annotation = %v, want ErrAnnotationNotFound", err)
	}
}

func TestUpdateAnnotationSelectorAuthorGate(t *testing.T) {
	dataStore, rootID := openAnnotated(t)
	ctx := context.Background()
	entry := sampleAnnotation(rootID, "mine")
	if err := dataStore.CreateAnnotation(ctx, &entry); err != nil {
		t.Fatalf("CreateAnnotation failed: %v", err)
	}
	replacement := annotation.Selector{
		Quote:    annotation.QuoteSelector{Exact: "改用 SQLite", Prefix: "", Suffix: ""},
		Position: annotation.PositionSelector{Start: 5, End: 11},
		Version:  1,
	}
	// A foreign author's UPDATE touches nothing — authorization rides
	// the statement itself.
	if err := dataStore.UpdateAnnotationSelector(ctx, entry.ID, "local:mallory", replacement, "sha256:evil"); err == nil {
		t.Fatalf("foreign update accepted")
	}
	untouched, _ := dataStore.GetAnnotation(ctx, entry.ID)
	if untouched.Selector.Quote.Exact != entry.Selector.Quote.Exact {
		t.Fatalf("foreign update changed the row: %+v", untouched)
	}
	if err := dataStore.UpdateAnnotationSelector(ctx, entry.ID, entry.AuthorID, replacement, "sha256:new"); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	updated, _ := dataStore.GetAnnotation(ctx, entry.ID)
	if updated.Selector.Quote.Exact != "改用 SQLite" ||
		updated.Selector.Position.Start != 5 || updated.DocumentHash != "sha256:new" {
		t.Fatalf("update did not persist: %+v", updated)
	}
	// Millisecond resolution can tie creation and update on a fast machine;
	// what must never happen is the timestamp going backwards.
	if updated.UpdatedAt < updated.CreatedAt {
		t.Fatalf("updatedAt went backwards: %d < %d", updated.UpdatedAt, updated.CreatedAt)
	}
}

func listedIDs(entries []annotation.Annotation) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}
