package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestReadPrinterMetadataPatch(t *testing.T) {
	location := "Warehouse B"
	maxLength := strings.Repeat("é", 255)

	cases := []struct {
		name string
		body string
		want map[string]*string
	}{
		{"only location supplied", `{"location":"Warehouse B"}`, map[string]*string{"location": &location}},
		{"clear nickname", `{"nickname":null}`, map[string]*string{"nickname": nil}},
		{"set and clear together", `{"location":"Warehouse B","serialNumber":null}`, map[string]*string{"location": &location, "serialNumber": nil}},
		{"255 Unicode characters", `{"nickname":"` + maxLength + `"}`, map[string]*string{"nickname": &maxLength}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/v1/printers/1", strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			got, ok := readPrinterMetadataPatch(w, r)
			if !ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, ok=%t, want %+v", got, ok, tc.want)
			}
		})
	}
}

func TestInvalidPrinterMetadataPatchReturnsBadRequest(t *testing.T) {
	router := NewRouter(nil)

	cases := []struct {
		name string
		body string
	}{
		{"missing body", ""},
		{"empty object", `{}`},
		{"null body", `null`},
		{"array body", `[]`},
		{"malformed JSON", `{"nickname":`},
		{"second JSON value", `{"nickname":null} {}`},
		{"unknown field", `{"location":"Warehouse B","isActive":null}`},
		{"incorrect field casing", `{"Nickname":"Dispatch"}`},
		{"numeric value", `{"serialNumber":123}`},
		{"empty string", `{"nickname":""}`},
		{"256 Unicode characters", `{"location":"` + strings.Repeat("é", 256) + `"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/v1/printers/1", strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest || w.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("expected JSON 400, got status=%d, headers=%v", w.Code, w.Header())
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

func TestPatchInvalidPrinterIDReturnsBadRequest(t *testing.T) {
	router := NewRouter(nil)
	cases := []struct {
		name string
		id   string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"nonnumeric", "abc"},
		{"above maximum printer ID", "9223372036854775808"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/v1/printers/"+tc.id, strings.NewReader(`{"nickname":null}`))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want 400", w.Code)
			}
		})
	}
}
