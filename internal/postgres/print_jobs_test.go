package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreatePrintJobIntegration(t *testing.T) {
	t.Run("new jobs start pending", testPendingPrintJob)
	t.Run("same key belongs to each requester independently", testPrintJobRequester)
	t.Run("conflicts and unavailable inventory", testRejectedPrintJobs)
	t.Run("retry returns current job and original inventory snapshot", testSavedPrintJob)
	t.Run("concurrent retries create one job", testConcurrentPrintJobRetries)
}

func testPendingPrintJob(t *testing.T) {
	db, request := newPrintJobFixture(t)

	first, created, err := CreatePrintJob(t.Context(), db, request)
	if err != nil {
		t.Fatal(err)
	}
	if !created || first.ID == 0 || first.PrinterID != 1 || first.LabelID != "label-1" ||
		first.ObservedLabelVersion != "3" || first.Status != "pending" ||
		first.CreatedAt.IsZero() || !first.UpdatedAt.Equal(first.CreatedAt) {
		t.Fatalf("unexpected new job: %+v (created %t)", first, created)
	}
	if first.LastErrorCode != nil || first.LastErrorMessage != nil {
		t.Fatal("expected both error fields to be nil for a new job")
	}
}

func testPrintJobRequester(t *testing.T) {
	db, request := newPrintJobFixture(t)
	first := requireNewPrintJob(t, db, request)

	other := request
	other.RequestedBy = "bob"
	job, created, err := CreatePrintJob(t.Context(), db, other)
	if err != nil || !created || job.ID == first.ID {
		t.Fatalf("expected a separate job for Bob, got %+v, created %t, err %v", job, created, err)
	}
}

func testRejectedPrintJobs(t *testing.T) {
	db, request := newPrintJobFixture(t)
	requireNewPrintJob(t, db, request)

	cases := []struct {
		name      string
		key       string
		printerID int64
		labelID   string
		wantErr   error
	}{
		{"same key with another printer", request.IdempotencyKey, 3, request.LabelID, ErrIdempotencyConflict},
		{"same key with another label", request.IdempotencyKey, 1, "label-10", ErrIdempotencyConflict},
		{"inactive printer", "new-key", 2, request.LabelID, pgx.ErrNoRows},
		{"missing printer", "new-key", 99, request.LabelID, pgx.ErrNoRows},
		{"inactive label", "new-key", 1, "label-11", pgx.ErrNoRows},
		{"missing label", "new-key", 1, "missing", pgx.ErrNoRows},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := PrintJobRequest{
				RequestedBy:    request.RequestedBy,
				IdempotencyKey: tc.key,
				PrinterID:      tc.printerID,
				LabelID:        tc.labelID,
			}

			_, created, err := CreatePrintJob(t.Context(), db, input)
			if created || !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got created %t, err %v", tc.wantErr, created, err)
			}
		})
	}
}

func testSavedPrintJob(t *testing.T) {
	db, request := newPrintJobFixture(t)
	first := requireNewPrintJob(t, db, request)

	execSQL(t, db, `UPDATE printers SET is_active = false, name = 'renamed' WHERE id = 1`)
	execSQL(t, db, `UPDATE labels SET version = '4' WHERE id = 'label-1'`)
	execSQL(t, db,
		`
		UPDATE print_jobs
		SET status = 'failed', last_error_code = 'offline', last_error_message = 'Printer offline',
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = @job_id
		`,
		pgx.NamedArgs{"job_id": first.ID},
	)

	job, created, err := CreatePrintJob(t.Context(), db, request)
	if err != nil {
		t.Fatal(err)
	}
	if created || job.ID != first.ID || job.Status != "failed" || job.ObservedLabelVersion != "3" {
		t.Fatalf("unexpected retry: %+v (created %t)", job, created)
	}
	if job.LastErrorCode == nil || *job.LastErrorCode != "offline" {
		t.Fatal("expected stored error code 'offline'")
	}
	if job.LastErrorMessage == nil || *job.LastErrorMessage != "Printer offline" {
		t.Fatal("expected stored error message 'Printer offline'")
	}

	var printerName string
	err = db.QueryRow(
		t.Context(),
		`
		SELECT printer_name
		FROM print_jobs
		WHERE id = @job_id
		`,
		pgx.NamedArgs{"job_id": job.ID},
	).Scan(&printerName)
	if err != nil || printerName != "p1" {
		t.Fatalf("expected original printer name p1, got %q, err %v", printerName, err)
	}
}

func testConcurrentPrintJobRetries(t *testing.T) {
	db, request := newPrintJobFixture(t)

	type result struct {
		job     PrintJob
		created bool
		err     error
	}
	const attempts = 8
	start := make(chan struct{})
	results := make(chan result, attempts)
	for range attempts {
		go func() {
			<-start
			job, created, err := CreatePrintJob(t.Context(), db, request)
			results <- result{job, created, err}
		}()
	}
	close(start)

	var jobID int64
	createdCount := 0
	for range attempts {
		r := <-results
		if r.err != nil {
			t.Error(r.err)
			continue
		}
		if jobID == 0 {
			jobID = r.job.ID
		}
		if r.job.ID != jobID {
			t.Errorf("expected job %d, got %d", jobID, r.job.ID)
		}
		if r.created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("expected one creation, got %d", createdCount)
	}
}

func newPrintJobFixture(t *testing.T) (*pgxpool.Pool, PrintJobRequest) {
	t.Helper()

	db := newTestDB(t)
	insertTestPrinters(t, db)
	insertTestLabels(t, db)

	request := PrintJobRequest{
		RequestedBy:    "alice",
		IdempotencyKey: "job-42",
		PrinterID:      1,
		LabelID:        "label-1",
	}

	return db, request
}

func requireNewPrintJob(t *testing.T, db *pgxpool.Pool, request PrintJobRequest) PrintJob {
	t.Helper()

	job, created, err := CreatePrintJob(t.Context(), db, request)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected a new print job")
	}
	return job
}
