package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadPrinterPage(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  printerPage
	}{
		{"omitted parameters use defaults", "", printerPage{limit: 50}},
		{"minimum limit", "limit=1", printerPage{limit: 1}},
		{"limit accepts a plus sign", "limit=%2B2", printerPage{limit: 2}},
		{"cursor alone uses default limit", "cursor=7", printerPage{limit: 50, afterID: 7}},
		{"explicit limit and cursor", "limit=2&cursor=7", printerPage{limit: 2, afterID: 7}},
		{"maximum limit and cursor", "limit=100&cursor=9223372036854775807", printerPage{limit: 100, afterID: math.MaxInt64}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/printers?"+tc.query, nil)
			w := httptest.NewRecorder()

			got, ok := readPrinterPage(w, r)
			if !ok || got != tc.want {
				t.Fatalf("got %+v, ok=%t, want %+v", got, ok, tc.want)
			}
		})
	}
}

func TestInvalidPrinterPageReturnsBadRequest(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"empty limit", "limit="},
		{"zero limit", "limit=0"},
		{"limit above maximum", "limit=101"},
		{"negative limit", "limit=-1"},
		{"fractional limit", "limit=1.5"},
		{"repeated limit", "limit=2&limit=3"},
		{"empty cursor", "cursor="},
		{"zero cursor", "cursor=0"},
		{"negative cursor", "cursor=-1"},
		{"cursor cannot have a plus sign", "cursor=%2B1"},
		{"nonnumeric cursor", "cursor=abc"},
		{"cursor exceeds maximum printer ID", "cursor=9223372036854775808"},
		{"repeated cursor", "cursor=1&cursor=2"},
		{"invalid URL encoding", "cursor=%zz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/printers?"+tc.query, nil)
			w := httptest.NewRecorder()

			_, ok := readPrinterPage(w, r)
			if ok || w.Code != http.StatusBadRequest || w.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("expected a JSON 400, got ok=%t, status=%d, headers=%v", ok, w.Code, w.Header())
			}

			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 2 || body["code"] != "bad_request" || body["message"] == "" {
				t.Fatalf("unexpected error body: %v", body)
			}
		})
	}
}
