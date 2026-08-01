package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchSuggest(t *testing.T) {
	var reqBody string
	var gotMethod, gotPath, gotRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		reqBody = string(data)
		gotMethod, gotPath, gotRaw = r.Method, r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"$type":"SearchSuggestions","query":"has: ","suggestions":[{"$type":"Suggestion","option":"star","description":"by star","prefix":"","suffix":" ","group":"Commands"}]}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	sugg, err := c.SearchSuggest(context.Background(), "has: ", FieldsSearchSuggest)
	if err != nil {
		t.Fatalf("SearchSuggest() error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/search/assist" {
		t.Errorf("request = %s %s, want POST /search/assist", gotMethod, gotPath)
	}
	if gotRaw != "fields="+FieldsSearchSuggest {
		t.Errorf("raw query = %q, want fields=%s", gotRaw, FieldsSearchSuggest)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(reqBody), &sent); err != nil {
		t.Fatalf("request body is not valid JSON: %v\n%s", err, reqBody)
	}
	if sent["query"] != "has: " {
		t.Errorf("body = %v, want query %q", sent, "has: ")
	}
	if sugg.Type != "SearchSuggestions" || sugg.Query != "has: " {
		t.Errorf("suggestions = %+v, want SearchSuggestions for query has: ", sugg)
	}
	if len(sugg.Suggestions) != 1 {
		t.Fatalf("len(suggestions) = %d, want 1", len(sugg.Suggestions))
	}
	s := sugg.Suggestions[0]
	if s.Type != "Suggestion" || s.Option != "star" || s.Description != "by star" || s.Suffix != " " || s.Group != "Commands" {
		t.Errorf("suggestion = %+v, want star / by star / Commands", s)
	}
}

func TestSearchSuggestEmptyFields(t *testing.T) {
	var gotRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		gotRaw = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"x"}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if _, err := c.SearchSuggest(context.Background(), "x", ""); err != nil {
		t.Fatalf("SearchSuggest() error: %v", err)
	}
	if gotRaw != "" {
		t.Errorf("raw query = %q, want empty", gotRaw)
	}
}
