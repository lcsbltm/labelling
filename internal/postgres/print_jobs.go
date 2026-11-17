package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used for a different print job")

type PrintJobRequest struct {
	RequestedBy    string
	IdempotencyKey string
	PrinterID      int64
	LabelID        string
}

type PrintJob struct {
	ID                   int64     `json:"id"`
	PrinterID            int64     `db:"printer_id" json:"printerId"`
	LabelID              string    `db:"label_id" json:"labelId"`
	ObservedLabelVersion string    `db:"observed_label_version" json:"observedLabelVersion"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt            time.Time `db:"updated_at" json:"updatedAt"`
	LastErrorCode        *string   `db:"last_error_code" json:"-"`
	LastErrorMessage     *string   `db:"last_error_message" json:"-"`
}

func CreatePrintJob(ctx context.Context, db *pgxpool.Pool, request PrintJobRequest) (PrintJob, bool, error) {
	job, err := existingPrintJob(ctx, db, request)
	if !errors.Is(err, pgx.ErrNoRows) {
		return job, false, err
	}

	rows, err := db.Query(
		ctx,
		`
		INSERT INTO print_jobs (
			printer_id, label_id, printer_name, observed_label_version, requested_by, idempotency_key
		)
		SELECT printers.id, labels.id, printers.name, labels.version, @requested_by, @idempotency_key
		FROM printers
		CROSS JOIN labels
		WHERE printers.id = @printer_id AND printers.is_active
		  AND labels.id = @label_id AND labels.is_active
		FOR SHARE OF printers, labels
		ON CONFLICT (requested_by, idempotency_key) DO NOTHING
		RETURNING id, printer_id, label_id, observed_label_version, status,
		          created_at, updated_at, last_error_code, last_error_message
		`,
		pgx.NamedArgs{
			"requested_by":    request.RequestedBy,
			"idempotency_key": request.IdempotencyKey,
			"printer_id":      request.PrinterID,
			"label_id":        request.LabelID,
		},
	)
	if err != nil {
		return PrintJob{}, false, err
	}

	job, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[PrintJob])
	if err == nil {
		return job, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return PrintJob{}, false, err
	}

	job, err = existingPrintJob(ctx, db, request)
	return job, false, err
}

func existingPrintJob(ctx context.Context, db *pgxpool.Pool, request PrintJobRequest) (PrintJob, error) {
	rows, err := db.Query(
		ctx,
		`
		SELECT id, printer_id, label_id, observed_label_version, status,
		       created_at, updated_at, last_error_code, last_error_message
		FROM print_jobs
		WHERE requested_by = @requested_by AND idempotency_key = @idempotency_key
		`,
		pgx.NamedArgs{
			"requested_by":    request.RequestedBy,
			"idempotency_key": request.IdempotencyKey,
		},
	)
	if err != nil {
		return PrintJob{}, err
	}

	job, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[PrintJob])
	if err != nil {
		return PrintJob{}, err
	}
	if job.PrinterID != request.PrinterID || job.LabelID != request.LabelID {
		return PrintJob{}, ErrIdempotencyConflict
	}

	return job, nil
}
