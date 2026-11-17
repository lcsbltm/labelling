package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestReadLabelPage(t *testing.T) {
	id := "label/7 + blue"

	cases := []struct {
		name  string
		query string
		want  labelPage
	}{
		{"omitted parameters use defaults", "", labelPage{limit: 50}},
		{"minimum limit", "limit=1", labelPage{limit: 1}},
		{"limit accepts a plus sign", "limit=%2B2", labelPage{limit: 2}},
		{"cursor preserves decoded label ID", "cursor=label%2F7+%2B+blue", labelPage{limit: 50, afterID: &id}},
		{"explicit limit and cursor", "limit=2&cursor=label%2F7+%2B+blue", labelPage{limit: 2, afterID: &id}},
		{"maximum limit", "limit=100", labelPage{limit: 100}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/labels?"+tc.query, nil)
			w := httptest.NewRecorder()

			got, ok := readLabelPage(w, r)
			if !ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, ok=%t, want %+v", got, ok, tc.want)
			}
		})
	}
}

func TestInvalidLabelPageReturnsBadRequest(t *testing.T) {
	router := NewRouter(nil)

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
		{"NUL in cursor", "cursor=abc%00"},
		{"invalid UTF-8 in cursor", "cursor=%FF"},
		{"repeated cursor", "cursor=1&cursor=2"},
		{"invalid URL encoding", "cursor=%zz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/labels?"+tc.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || w.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("expected a JSON 400, got status=%d, headers=%v", w.Code, w.Header())
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
