package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Metadata struct {
	Nickname     *string `db:"nickname" json:"nickname"`
	Location     *string `db:"location" json:"location"`
	SerialNumber *string `db:"serial_number" json:"serialNumber"`
}

type Printer struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	IsActive bool   `db:"is_active" json:"isActive"`
	Metadata `json:"metadata"`
}

type Pagination struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"nextCursor"`
}

type PrinterPage struct {
	Items                []Printer  `json:"items"`
	LastSuccessfulSyncAt time.Time  `json:"lastSuccessfulSyncAt"`
	Pagination           Pagination `json:"pagination"`
}

func ListPrinters(ctx context.Context, db *pgxpool.Pool, afterID int64, limit int) (PrinterPage, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PrinterPage{}, err
	}
	defer tx.Rollback(ctx)

	syncRows, err := tx.Query(
		ctx,
		`
		SELECT last_successful_sync_at
		FROM inventory_sync_state
		WHERE resource_type = 'printers'
		`,
	)
	if err != nil {
		return PrinterPage{}, err
	}

	syncedAt, err := pgx.CollectExactlyOneRow(syncRows, pgx.RowTo[*time.Time])
	if err != nil {
		return PrinterPage{}, err
	}
	if syncedAt == nil {
		return PrinterPage{}, errors.New("printer inventory has never synced")
	}

	rows, err := tx.Query(
		ctx,
		`
		SELECT id, name, model, is_active, nickname, location, serial_number
		FROM printers
		WHERE is_active AND id > @after_id
		ORDER BY id ASC
		LIMIT @fetch_limit
		`,
		pgx.NamedArgs{
			"after_id":    afterID,
			"fetch_limit": limit + 1,
		},
	)
	if err != nil {
		return PrinterPage{}, err
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Printer])
	if err != nil {
		return PrinterPage{}, err
	}

	page := PrinterPage{
		Items:                items,
		LastSuccessfulSyncAt: *syncedAt,
		Pagination:           Pagination{Limit: limit},
	}

	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		cursor := strconv.FormatInt(page.Items[limit-1].ID, 10)
		page.Pagination.NextCursor = &cursor
	}

	err = tx.Commit(ctx)
	if err != nil {
		return PrinterPage{}, err
	}

	return page, nil
}
