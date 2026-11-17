package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestUpdatePrinterMetadata(t *testing.T) {
	db := newTestDB(t)
	insertTestPrinters(t, db)

	location := "Warehouse B"
	fields := map[string]*string{"location": &location, "nickname": nil}
	printer, err := UpdatePrinterMetadata(t.Context(), db, 1, fields)
	if err != nil {
		t.Fatal(err)
	}

	m := printer.Metadata
	if m.Location == nil || *m.Location != location || m.Nickname != nil || m.SerialNumber == nil || *m.SerialNumber != "SN-1" {
		t.Fatalf("expected updated location, cleared nickname, and original serial number; got %+v", printer)
	}

	cases := []struct {
		name string
		id   int64
	}{
		{"inactive printer", 2},
		{"missing printer", 99},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UpdatePrinterMetadata(t.Context(), db, tc.id, fields)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("expected ErrNoRows, got %v", err)
			}
		})
	}
}
