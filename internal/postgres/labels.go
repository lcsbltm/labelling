package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LabelMetadata struct {
	Nickname  *string `db:"nickname" json:"nickname"`
	IsPrimary bool    `db:"is_primary" json:"isPrimary"`
}

type Label struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	IsActive      bool   `db:"is_active" json:"isActive"`
	LabelMetadata `json:"metadata"`
}

type LabelPage struct {
	Items                []Label    `json:"items"`
	LastSuccessfulSyncAt time.Time  `json:"lastSuccessfulSyncAt"`
	Pagination           Pagination `json:"pagination"`
}

func ListLabels(ctx context.Context, db *pgxpool.Pool, afterID *string, limit int) (LabelPage, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return LabelPage{}, err
	}
	defer tx.Rollback(ctx)

	syncRows, err := tx.Query(
		ctx,
		`
		SELECT last_successful_sync_at
		FROM inventory_sync_state
		WHERE resource_type = 'labels'
		`,
	)
	if err != nil {
		return LabelPage{}, err
	}

	syncedAt, err := pgx.CollectExactlyOneRow(syncRows, pgx.RowTo[*time.Time])
	if err != nil {
		return LabelPage{}, err
	}
	if syncedAt == nil {
		return LabelPage{}, errors.New("label inventory has never synced")
	}

	rows, err := tx.Query(
		ctx,
		`
		SELECT id, name, version, is_active, nickname, is_primary
		FROM labels
		WHERE is_active AND (@after_id::text IS NULL OR id > @after_id)
		ORDER BY id ASC
		LIMIT @fetch_limit
		`,
		pgx.NamedArgs{
			"after_id":    afterID,
			"fetch_limit": limit + 1,
		},
	)
	if err != nil {
		return LabelPage{}, err
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Label])
	if err != nil {
		return LabelPage{}, err
	}

	page := LabelPage{
		Items:                items,
		LastSuccessfulSyncAt: *syncedAt,
		Pagination:           Pagination{Limit: limit},
	}

	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		cursor := page.Items[limit-1].ID
		page.Pagination.NextCursor = &cursor
	}

	err = tx.Commit(ctx)
	if err != nil {
		return LabelPage{}, err
	}

	return page, nil
}
