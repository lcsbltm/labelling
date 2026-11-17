package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func UpdatePrinterMetadata(ctx context.Context, db *pgxpool.Pool, id int64, fields map[string]*string) (Printer, error) {
	nickname, hasNickname := fields["nickname"]
	location, hasLocation := fields["location"]
	serialNumber, hasSerialNumber := fields["serialNumber"]

	rows, err := db.Query(
		ctx,
		`
		UPDATE printers
		SET nickname = CASE WHEN @has_nickname THEN @nickname ELSE nickname END,
		    location = CASE WHEN @has_location THEN @location ELSE location END,
		    serial_number = CASE WHEN @has_serial_number THEN @serial_number ELSE serial_number END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = @printer_id AND is_active
		RETURNING id, name, model, is_active, nickname, location, serial_number
		`,
		pgx.NamedArgs{
			"printer_id":        id,
			"has_nickname":      hasNickname,
			"nickname":          nickname,
			"has_location":      hasLocation,
			"location":          location,
			"has_serial_number": hasSerialNumber,
			"serial_number":     serialNumber,
		},
	)
	if err != nil {
		return Printer{}, err
	}

	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Printer])
}
