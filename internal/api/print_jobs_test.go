package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"labelling/internal/postgres"
)

func TestReadPrintJobRequest(t *testing.T) {
	const body = `{"printerId":1,"labelId":"label-123"}`
	cases := []struct {
		name string
		key  string
	}{
		{"one character", "a"},
		{"allowed punctuation", "Job_1:copy-2.3"},
		{"longest key", strings.Repeat("x", 128)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/print-jobs", strings.NewReader(body))
			r.Header.Set("X-Requested-By", "user-123")
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()

			got, ok := readPrintJobRequest(w, r)
			want := postgres.PrintJobRequest{
				RequestedBy:    "user-123",
				IdempotencyKey: tc.key,
				PrinterID:      1,
				LabelID:        "label-123",
			}
			if !ok || got != want {
				t.Fatalf("got %+v, ok=%t, want %+v", got, ok, want)
			}
		})
	}
}

func TestInvalidPrintJobHeaders(t *testing.T) {
	requester := []string{"alice"}
	key := []string{"job-42"}
	cases := []struct {
		name       string
		requesters []string
		keys       []string
		status     int
		code       string
	}{
		{"missing requester", nil, key, 500, "internal_error"},
		{"blank requester", []string{" "}, key, 500, "internal_error"},
		{"duplicate requester", []string{"alice", "bob"}, key, 500, "internal_error"},
		{"missing key", requester, nil, 400, "bad_request"},
		{"empty key", requester, []string{""}, 400, "bad_request"},
		{"duplicate key", requester, []string{"a", "a"}, 400, "bad_request"},
		{"invalid key character", requester, []string{"a/b"}, 400, "bad_request"},
		{"key too long", requester, []string{strings.Repeat("x", 129)}, 400, "bad_request"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/print-jobs", strings.NewReader(`{"printerId":1,"labelId":"label-123"}`))
			r.Header["X-Requested-By"] = tc.requesters
			r.Header["Idempotency-Key"] = tc.keys
			requirePrintJobRequestError(t, r, tc.status, tc.code)
		})
	}
}

func TestInvalidPrintJobBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"malformed JSON", `{"printerId":`},
		{"array", `[]`},
		{"missing printer", `{"labelId":"a"}`},
		{"missing label", `{"printerId":1}`},
		{"zero printer", `{"printerId":0,"labelId":"a"}`},
		{"fractional printer", `{"printerId":1.5,"labelId":"a"}`},
		{"printer ID too large", `{"printerId":9223372036854775808,"labelId":"a"}`},
		{"empty label", `{"printerId":1,"labelId":""}`},
		{"numeric label", `{"printerId":1,"labelId":42}`},
		{"null byte in label", `{"printerId":1,"labelId":"a\u0000"}`},
		{"wrong field casing", `{"PrinterId":1,"labelId":"a"}`},
		{"requester in body", `{"printerId":1,"labelId":"a","requestedBy":"bob"}`},
		{"second object", `{"printerId":1,"labelId":"a"} {}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/print-jobs", strings.NewReader(tc.body))
			r.Header.Set("X-Requested-By", "alice")
			r.Header.Set("Idempotency-Key", "job-42")
			requirePrintJobRequestError(t, r, http.StatusBadRequest, "bad_request")
		})
	}
}

func requirePrintJobRequestError(t *testing.T, r *http.Request, status int, code string) {
	t.Helper()

	w := httptest.NewRecorder()
	NewRouter(nil).ServeHTTP(w, r)
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected JSON %d, got status=%d, headers=%v", status, w.Code, w.Header())
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if len(body) != 2 || body["code"] != code || body["message"] == "" {
		t.Fatalf("unexpected error body: %v", body)
	}
}
