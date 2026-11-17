package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LabelMetadataPatch struct {
	HasNickname bool
	Nickname    *string
	IsPrimary   *bool
}

func UpdateLabelMetadata(ctx context.Context, db *pgxpool.Pool, id string, patch LabelMetadataPatch) (Label, error) {
	rows, err := db.Query(
		ctx,
		`
		UPDATE labels
		SET nickname = CASE WHEN @has_nickname THEN @nickname ELSE nickname END,
		    is_primary = COALESCE(@is_primary, is_primary),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = @label_id AND is_active
		RETURNING id, name, version, is_active, nickname, is_primary
		`,
		pgx.NamedArgs{
			"label_id":     id,
			"has_nickname": patch.HasNickname,
			"nickname":     patch.Nickname,
			"is_primary":   patch.IsPrimary,
		},
	)
	if err != nil {
		return Label{}, err
	}

	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Label])
}
