package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListLabelsIntegration(t *testing.T) {
	t.Run("only the label sync establishes inventory", testEmptyLabelInventory)
	t.Run("pages follow text IDs across inactive gaps", testActiveLabelPages)
	t.Run("a saved cursor survives changes to its label", testSavedLabelCursor)
}

func testEmptyLabelInventory(t *testing.T) {
	db := newTestDB(t)
	recordPrinterSync(t, db)

	_, err := ListLabels(t.Context(), db, nil, 2)
	if err == nil {
		t.Fatal("expected an error before the first label sync")
	}

	syncedAt := recordLabelSync(t, db)
	page := requireLabelPage(t, db, nil, 2, syncedAt)
	if page.Items == nil || len(page.Items) != 0 || page.Pagination.NextCursor != nil {
		t.Fatalf("expected an empty synced label page, got %+v", page)
	}
}

func testActiveLabelPages(t *testing.T) {
	db := newTestDB(t)
	syncedAt := recordLabelSync(t, db)
	insertTestLabels(t, db)

	first := requireLabelPage(t, db, nil, 2, syncedAt)
	if len(first.Items) != 2 || first.Items[0].ID != "label-1" || first.Items[1].ID != "label-10" {
		t.Fatalf("unexpected first page: %+v", first.Items)
	}
	cursor := first.Pagination.NextCursor
	if cursor == nil || *cursor != "label-10" {
		t.Fatal("expected cursor label-10")
	}

	last := requireLabelPage(t, db, cursor, 2, syncedAt)
	if len(last.Items) != 1 || last.Items[0].ID != "label-2" || last.Pagination.NextCursor != nil {
		t.Fatalf("expected only label-2 and a null cursor, got %+v", last)
	}
	if last.Items[0].Nickname != nil || last.Items[0].IsPrimary {
		t.Fatalf("unexpected metadata: %+v", last.Items[0].LabelMetadata)
	}
}

func testSavedLabelCursor(t *testing.T) {
	cases := []struct {
		name   string
		change string
	}{
		{"deactivated", `UPDATE labels SET is_active = false WHERE id = 'label-10'`},
		{"deleted", `DELETE FROM labels WHERE id = 'label-10'`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			syncedAt := recordLabelSync(t, db)
			insertTestLabels(t, db)
			first := requireLabelPage(t, db, nil, 2, syncedAt)
			execSQL(t, db, tc.change)

			last := requireLabelPage(t, db, first.Pagination.NextCursor, 2, syncedAt)
			if len(last.Items) != 1 || last.Items[0].ID != "label-2" || last.Pagination.NextCursor != nil {
				t.Fatalf("expected only label-2 and a null cursor, got %+v", last)
			}
		})
	}
}

func recordLabelSync(t *testing.T, db *pgxpool.Pool) time.Time {
	t.Helper()

	syncedAt := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	execSQL(t, db, `UPDATE inventory_sync_state SET last_successful_sync_at = @synced_at WHERE resource_type = 'labels'`, pgx.NamedArgs{"synced_at": syncedAt})
	return syncedAt
}

func insertTestLabels(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	execSQL(t, db, `
		INSERT INTO labels (id, name, version, is_active, nickname, is_primary) VALUES
			('label-1', 'Shipping', '3', true, 'Standard', true),
			('label-10', 'Returns', '2', true, 'Desk', false),
			('label-11', 'Old', '1', false, NULL, false),
			('label-2', 'Large', '1', true, NULL, false),
			('label-9', 'Legacy', '1', false, NULL, false)`)
}

func requireLabelPage(t *testing.T, db *pgxpool.Pool, cursor *string, limit int, syncedAt time.Time) LabelPage {
	t.Helper()

	page, err := ListLabels(t.Context(), db, cursor, limit)
	if err != nil {
		t.Fatal(err)
	}
	if !page.LastSuccessfulSyncAt.Equal(syncedAt) || page.Pagination.Limit != limit {
		t.Fatalf("unexpected timestamp or limit: %+v", page)
	}
	return page
}
