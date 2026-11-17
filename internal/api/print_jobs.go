package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"labelling/internal/postgres"
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type jobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type printJobResponse struct {
	postgres.PrintJob
	LastError *jobError `json:"lastError"`
}

func createPrintJob(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := readPrintJobRequest(w, r)
		if !ok {
			return
		}

		job, created, err := postgres.CreatePrintJob(r.Context(), db, request)
		if errors.Is(err, postgres.ErrIdempotencyConflict) {
			writeError(w, http.StatusConflict, "idempotency_conflict", "This key was already used for a different print job.")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "Active printer or label not found.")
			return
		}
		if err != nil {
			log.Printf("create print job: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not create print job.")
			return
		}

		response := printJobResponse{PrintJob: job}
		if job.LastErrorCode != nil && job.LastErrorMessage != nil {
			response.LastError = &jobError{Code: *job.LastErrorCode, Message: *job.LastErrorMessage}
		}

		status := http.StatusOK
		if created {
			status = http.StatusAccepted
		}
		writeJSON(w, status, response)
	}
}

func readPrintJobRequest(w http.ResponseWriter, r *http.Request) (postgres.PrintJobRequest, bool) {
	var request postgres.PrintJobRequest

	requesters := r.Header.Values("X-Requested-By")
	if len(requesters) != 1 || strings.TrimSpace(requesters[0]) == "" || !utf8.ValidString(requesters[0]) {
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not identify the requester.")
		return request, false
	}

	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !idempotencyKeyPattern.MatchString(keys[0]) {
		writeBadRequest(w, "Provide one Idempotency-Key of 1 to 128 letters, digits, dots, underscores, colons, or hyphens.")
		return request, false
	}

	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&fields)
	if err != nil || len(fields) != 2 || fields["printerId"] == nil || fields["labelId"] == nil {
		writeBadRequest(w, "Provide a JSON object with only printerId and labelId.")
		return request, false
	}

	err = decoder.Decode(new(json.RawMessage))
	if err != io.EOF {
		writeBadRequest(w, "Provide a single JSON object.")
		return request, false
	}

	err = json.Unmarshal(fields["printerId"], &request.PrinterID)
	if err != nil || request.PrinterID < 1 {
		writeBadRequest(w, "Provide a positive integer for printerId.")
		return request, false
	}

	err = json.Unmarshal(fields["labelId"], &request.LabelID)
	if err != nil || !validLabelID(request.LabelID) {
		writeBadRequest(w, "Provide a valid label ID.")
		return request, false
	}

	request.RequestedBy = requesters[0]
	request.IdempotencyKey = keys[0]
	return request, true
}
