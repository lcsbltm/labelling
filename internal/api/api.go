package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(db *pgxpool.Pool) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/printers", listPrinters(db))
	mux.HandleFunc("PATCH /v1/printers/{printerId}", updatePrinterMetadata(db))
	mux.HandleFunc("GET /v1/labels", listLabels(db))
	mux.HandleFunc("PATCH /v1/labels/{labelId}", updateLabelMetadata(db))
	mux.HandleFunc("POST /v1/print-jobs", createPrintJob(db))

	return mux
}
