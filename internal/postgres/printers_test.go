package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListPrintersIntegration(t *testing.T) {
	t.Run("sync determines whether an empty inventory can be served", testEmptyPrinterInventory)
	t.Run("pagination returns only active printers", testActivePrinterPages)
	t.Run("a saved cursor survives changes to its printer", testSavedPrinterCursor)
}

func testEmptyPrinterInventory(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	const limit = 2

	// Before the first sync, the inventory is unknown.
	_, err := ListPrinters(ctx, db, 0, limit)
	if err == nil {
		t.Fatal("expected an error before the first printer sync")
	}

	// Record a successful sync while the printer table is still empty.
	syncedAt := recordPrinterSync(t, db)

	empty := requirePrinterPage(t, db, 0, limit, syncedAt)
	if empty.Items == nil || len(empty.Items) != 0 || empty.Pagination.NextCursor != nil {
		t.Fatalf("expected an empty list and null cursor after sync, got %+v", empty)
	}
}

func testActivePrinterPages(t *testing.T) {
	db := newTestDB(t)
	syncedAt := recordPrinterSync(t, db)
	insertTestPrinters(t, db)
	const limit = 2

	// The first page ends at printer 3, with printer 5 on the next page.
	first := requirePrinterPage(t, db, 0, limit, syncedAt)
	if len(first.Items) != 2 || first.Items[0].ID != 1 || first.Items[1].ID != 3 {
		t.Fatalf("expected IDs [1, 3], got %+v", first.Items)
	}

	if first.Pagination.NextCursor == nil || *first.Pagination.NextCursor != "3" {
		t.Fatal("expected the first page to end with cursor 3")
	}

	// Check both populated metadata and SQL NULLs.
	printer := first.Items[0]
	m := printer.Metadata

	if !printer.IsActive || printer.Name != "p1" || printer.Model != "A" ||
		m.Nickname == nil || *m.Nickname != "Packing" || m.Location == nil || *m.Location != "West" ||
		m.SerialNumber == nil || *m.SerialNumber != "SN-1" {
		t.Fatalf("unexpected data for printer 1: %+v", printer)
	}

	if first.Items[1].Metadata != (Metadata{}) {
		t.Fatalf("expected all metadata on printer 3 to be null, got %+v", first.Items[1].Metadata)
	}

	last := requirePrinterPage(t, db, first.Items[1].ID, limit, syncedAt)
	if len(last.Items) != 1 || last.Items[0].ID != 5 || last.Pagination.NextCursor != nil {
		t.Fatalf("expected only printer 5 and a null cursor, got %+v", last)
	}
}

func testSavedPrinterCursor(t *testing.T) {
	cases := []struct {
		name   string
		change string
	}{
		{name: "active"},
		{name: "deactivated", change: `UPDATE printers SET is_active = false WHERE id = 3`},
		{name: "deleted", change: `DELETE FROM printers WHERE id = 3`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			syncedAt := recordPrinterSync(t, db)
			insertTestPrinters(t, db)

			// The client saved cursor "3" before the printer changed.
			const afterID = 3
			if tc.change != "" {
				execSQL(t, db, tc.change)
			}

			page := requirePrinterPage(t, db, afterID, 2, syncedAt)
			if len(page.Items) != 1 || page.Items[0].ID != 5 || page.Pagination.NextCursor != nil {
				t.Fatalf("expected only printer 5 and a null cursor, got %+v", page)
			}
		})
	}
}

func recordPrinterSync(t *testing.T, db *pgxpool.Pool) time.Time {
	t.Helper()

	syncedAt := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	execSQL(t, db,
		`
		UPDATE inventory_sync_state
		SET last_successful_sync_at = @synced_at
		WHERE resource_type = 'printers'
		`,
		pgx.NamedArgs{
			"synced_at": syncedAt,
		},
	)
	return syncedAt
}

func insertTestPrinters(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	// Place inactive printers between active IDs and after the last active ID.
	execSQL(t, db, `
		INSERT INTO printers (name, model, is_active, nickname, location, serial_number) VALUES
			('p1', 'A', true,  'Packing', 'West', 'SN-1'),
			('p2', 'A', false, NULL, NULL, NULL),
			('p3', 'B', true,  NULL, NULL, NULL),
			('p4', 'B', false, NULL, NULL, NULL),
			('p5', 'C', true,  NULL, NULL, NULL),
			('p6', 'C', false, NULL, NULL, NULL)`,
	)
}

func requirePrinterPage(t *testing.T, db *pgxpool.Pool, afterID int64, limit int, syncedAt time.Time) PrinterPage {
	t.Helper()

	page, err := ListPrinters(t.Context(), db, afterID, limit)
	if err != nil {
		t.Fatal(err)
	}

	if !page.LastSuccessfulSyncAt.Equal(syncedAt) || page.Pagination.Limit != limit {
		t.Fatalf("unexpected timestamp or limit: %+v", page)
	}

	return page
}
