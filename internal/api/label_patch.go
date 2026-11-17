package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"labelling/internal/postgres"
)

func updateLabelMetadata(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("labelId")
		if !validLabelID(id) {
			writeBadRequest(w, "Provide a valid label ID.")
			return
		}

		fields, ok := readLabelMetadataPatch(w, r)
		if !ok {
			return
		}

		label, err := postgres.UpdateLabelMetadata(r.Context(), db, id, fields)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "Label not found.")
			return
		}
		if err != nil {
			log.Printf("update label %q: %v", id, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not update label.")
			return
		}

		writeJSON(w, http.StatusOK, label)
	}
}

func readLabelMetadataPatch(w http.ResponseWriter, r *http.Request) (postgres.LabelMetadataPatch, bool) {
	var patch postgres.LabelMetadataPatch
	var fields map[string]json.RawMessage

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&fields)
	if err != nil || len(fields) == 0 {
		writeBadRequest(w, "Provide a JSON object with at least one metadata field.")
		return patch, false
	}

	err = decoder.Decode(new(json.RawMessage))
	if err != io.EOF {
		writeBadRequest(w, "Provide a single JSON object.")
		return patch, false
	}

	for name, raw := range fields {
		switch name {
		case "nickname":
			patch.HasNickname = true
			err = json.Unmarshal(raw, &patch.Nickname)
			if err != nil {
				writeBadRequest(w, "Provide a string or null for nickname.")
				return patch, false
			}
			if patch.Nickname != nil && (utf8.RuneCountInString(*patch.Nickname) > 255 || *patch.Nickname == "" || strings.ContainsRune(*patch.Nickname, 0)) {
				writeBadRequest(w, "Provide a nickname of 1 to 255 characters without a null byte.")
				return patch, false
			}
		case "isPrimary":
			err = json.Unmarshal(raw, &patch.IsPrimary)
			if err != nil || patch.IsPrimary == nil {
				writeBadRequest(w, "Provide true or false for isPrimary.")
				return patch, false
			}
		default:
			writeBadRequest(w, "Only nickname and isPrimary can be updated.")
			return patch, false
		}
	}
	return patch, true
}
