package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"labelling/internal/postgres"
)

func TestReadLabelMetadataPatch(t *testing.T) {
	primary := true
	secondary := false
	maxLength := strings.Repeat("é", 255)

	cases := []struct {
		name string
		body string
		want postgres.LabelMetadataPatch
	}{
		{"set nickname", `{"nickname":"` + maxLength + `"}`, postgres.LabelMetadataPatch{HasNickname: true, Nickname: &maxLength}},
		{"clear nickname", `{"nickname":null}`, postgres.LabelMetadataPatch{HasNickname: true}},
		{"set independent flag", `{"isPrimary":true}`, postgres.LabelMetadataPatch{IsPrimary: &primary}},
		{"clear flag and nickname", `{"isPrimary":false,"nickname":null}`, postgres.LabelMetadataPatch{HasNickname: true, IsPrimary: &secondary}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/v1/labels/1", strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			got, ok := readLabelMetadataPatch(w, r)
			if !ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, ok=%t, want %+v", got, ok, tc.want)
			}
		})
	}
}

func TestInvalidLabelMetadataPatchReturnsBadRequest(t *testing.T) {
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
		{"unknown field", `{"nickname":"Shipping","isActive":null}`},
		{"incorrect field casing", `{"Nickname":"Dispatch"}`},
		{"numeric nickname", `{"nickname":123}`},
		{"null flag", `{"isPrimary":null}`},
		{"string flag", `{"isPrimary":"false"}`},
		{"empty string", `{"nickname":""}`},
		{"256 Unicode characters", `{"nickname":"` + strings.Repeat("é", 256) + `"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/v1/labels/1", strings.NewReader(tc.body))
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

func TestPatchInvalidLabelIDReturnsBadRequest(t *testing.T) {
	router := NewRouter(nil)
	cases := []struct {
		name string
		id   string
	}{
		{"NUL in ID", "label%00"},
		{"invalid UTF-8 in ID", "%FF"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/v1/labels/"+tc.id, strings.NewReader(`{"nickname":null}`))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want 400", w.Code)
			}
		})
	}
}
