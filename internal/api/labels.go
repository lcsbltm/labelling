package api

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"labelling/internal/postgres"
)

type labelPage struct {
	limit   int
	afterID *string
}

func listLabels(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, ok := readLabelPage(w, r)
		if !ok {
			return
		}

		page, err := postgres.ListLabels(r.Context(), db, params.afterID, params.limit)
		if err != nil {
			log.Printf("list labels: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not list labels.")
			return
		}

		writeJSON(w, http.StatusOK, page)
	}
}

func readLabelPage(w http.ResponseWriter, r *http.Request) (labelPage, bool) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeBadRequest(w, "Invalid query string.")
		return labelPage{}, false
	}

	page := labelPage{limit: 50}

	values, present := query["limit"]
	if present {
		if len(values) != 1 {
			writeBadRequest(w, "Provide a single limit.")
			return labelPage{}, false
		}

		limit, err := strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			writeBadRequest(w, "Provide a limit between 1 and 100.")
			return labelPage{}, false
		}
		page.limit = limit
	}

	values, present = query["cursor"]
	if present {
		if len(values) != 1 || !validLabelID(values[0]) {
			writeBadRequest(w, "Provide a single nonempty label ID as the cursor.")
			return labelPage{}, false
		}
		page.afterID = &values[0]
	}
	return page, true
}

func validLabelID(id string) bool {
	return id != "" && utf8.ValidString(id) && !strings.ContainsRune(id, 0)
}
