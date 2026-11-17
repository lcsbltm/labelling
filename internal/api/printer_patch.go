package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"labelling/internal/postgres"
)

func updatePrinterMetadata(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(r.PathValue("printerId"), 10, 63)
		if err != nil || id == 0 {
			writeBadRequest(w, "Provide a positive decimal printer ID.")
			return
		}

		fields, ok := readPrinterMetadataPatch(w, r)
		if !ok {
			return
		}

		printer, err := postgres.UpdatePrinterMetadata(r.Context(), db, int64(id), fields)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "Printer not found.")
			return
		}
		if err != nil {
			log.Printf("update printer %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not update printer.")
			return
		}

		writeJSON(w, http.StatusOK, printer)
	}
}

func readPrinterMetadataPatch(w http.ResponseWriter, r *http.Request) (map[string]*string, bool) {
	var fields map[string]*string

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&fields)
	if err != nil {
		writeBadRequest(w, "Provide a JSON object with string or null metadata values.")
		return nil, false
	}

	err = decoder.Decode(new(json.RawMessage))
	if err != io.EOF {
		writeBadRequest(w, "Provide a single JSON object.")
		return nil, false
	}

	if len(fields) == 0 {
		writeBadRequest(w, "Provide at least one metadata field.")
		return nil, false
	}

	for name, value := range fields {
		if name != "nickname" && name != "location" && name != "serialNumber" {
			writeBadRequest(w, "Only nickname, location, and serialNumber can be updated.")
			return nil, false
		}

		if value != nil && (*value == "" || utf8.RuneCountInString(*value) > 255) {
			writeBadRequest(w, name+" must be null or contain 1 to 255 characters.")
			return nil, false
		}
	}
	return fields, true
}
