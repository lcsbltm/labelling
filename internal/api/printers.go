package api

import (
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"labelling/internal/postgres"
)

type printerPage struct {
	limit   int
	afterID int64
}

func listPrinters(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, ok := readPrinterPage(w, r)
		if !ok {
			return
		}

		page, err := postgres.ListPrinters(r.Context(), db, params.afterID, params.limit)
		if err != nil {
			log.Printf("list printers: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not list printers.")
			return
		}

		writeJSON(w, http.StatusOK, page)
	}
}

func readPrinterPage(w http.ResponseWriter, r *http.Request) (printerPage, bool) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeBadRequest(w, "Invalid query string.")
		return printerPage{}, false
	}

	page := printerPage{limit: 50}

	values, present := query["limit"]
	if present {
		if len(values) != 1 {
			writeBadRequest(w, "Provide a single limit.")
			return printerPage{}, false
		}

		limit, err := strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			writeBadRequest(w, "Provide a limit between 1 and 100.")
			return printerPage{}, false
		}
		page.limit = limit
	}

	values, present = query["cursor"]
	if present {
		if len(values) != 1 {
			writeBadRequest(w, "Provide a single cursor.")
			return printerPage{}, false
		}

		id, err := strconv.ParseUint(values[0], 10, 63)
		if err != nil || id == 0 {
			writeBadRequest(w, "Provide a positive decimal printer ID as the cursor.")
			return printerPage{}, false
		}
		page.afterID = int64(id)
	}
	return page, true
}
