package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCommandAssist(t *testing.T) {
	var reqBody string
	var gotMethod, gotPath, gotRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		reqBody = string(data)
		gotMethod, gotPath, gotRaw = r.Method, r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"$type":"CommandList","query":"state: ","suggestions":[{"$type":"Suggestion","option":"Fixed","description":"Fixed state","prefix":" ","suffix":" ","group":""}]}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	res, err := c.CommandAssist(context.Background(), "state: ", FieldsCommandAssist)
	if err != nil {
		t.Fatalf("CommandAssist() error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/commands/assist" {
		t.Errorf("request = %s %s, want POST /commands/assist", gotMethod, gotPath)
	}
	if gotRaw != "fields="+FieldsCommandAssist {
		t.Errorf("raw query = %q, want fields=%s", gotRaw, FieldsCommandAssist)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(reqBody), &sent); err != nil {
		t.Fatalf("request body is not valid JSON: %v\n%s", err, reqBody)
	}
	if sent["query"] != "state: " {
		t.Errorf("body = %v, want query %q", sent, "state: ")
	}
	if sent["caret"] != float64(len("state: ")) {
		t.Errorf("body caret = %v, want %d", sent["caret"], len("state: "))
	}
	if res.Type != "CommandList" || res.Query != "state: " {
		t.Errorf("result = %+v, want CommandList for query state: ", res)
	}
	if len(res.Suggestions) != 1 {
		t.Fatalf("len(suggestions) = %d, want 1", len(res.Suggestions))
	}
	s := res.Suggestions[0]
	if s.Type != "Suggestion" || s.Option != "Fixed" || s.Description != "Fixed state" || s.Suffix != " " {
		t.Errorf("suggestion = %+v, want Fixed / Fixed state", s)
	}
}

func TestCommandAssistCaretUTF8(t *testing.T) {
	var reqBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		reqBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"state: "}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	query := "состояние: "
	if _, err := c.CommandAssist(context.Background(), query, ""); err != nil {
		t.Fatalf("CommandAssist() error: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(reqBody), &sent); err != nil {
		t.Fatalf("request body is not valid JSON: %v\n%s", err, reqBody)
	}
	if sent["caret"] != float64(len([]rune(query))) {
		t.Errorf("body caret = %v, want %d (runes)", sent["caret"], len([]rune(query)))
	}
}

func TestCommandAssistEmptyFields(t *testing.T) {
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
	if _, err := c.CommandAssist(context.Background(), "x", ""); err != nil {
		t.Fatalf("CommandAssist() error: %v", err)
	}
	if gotRaw != "" {
		t.Errorf("raw query = %q, want empty", gotRaw)
	}
}
