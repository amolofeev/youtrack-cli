package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/amolofeev/prompt-and-pray/internal/api"
)

// tagListBody — фикстурный ответ GET /tags (поля FieldsTagList).
const tagListBody = `[
	{"$type":"Tag","id":"8-1","name":"backend","untagOnResolve":false},
	{"$type":"Tag","id":"8-2","name":"auth","untagOnResolve":true}
]`

// tagServer поднимает фейковый YouTrack, отвечающий на GET /tags, и
// фиксирует запросы.
func tagServer(t *testing.T, body string) (*httptest.Server, *[]string) {
	t.Helper()
	reqs := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, reqs
}

// runTagList выполняет yt tag list <args> против фейкового сервера.
func runTagList(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"tag", "list"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestTagRegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	sub, _, err := root.Find([]string{"tag"})
	if err != nil {
		t.Fatalf("find tag: %v", err)
	}
	if sub.GroupID != "server" {
		t.Errorf("tag.GroupID = %q, want %q", sub.GroupID, "server")
	}
}

func TestNewTagCmd(t *testing.T) {
	cmd := newTagCmd()
	if cmd.Use != "tag" {
		t.Errorf("Use = %q, want tag", cmd.Use)
	}
	var found bool
	for _, c := range cmd.Commands() {
		if c.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Error("expected \"list\" subcommand")
	}
}

func TestNewTagListCmd_Flags(t *testing.T) {
	cmd := newTagListCmd()

	query := cmd.Flags().Lookup("query")
	if query == nil || query.Shorthand != "q" || query.DefValue != "" {
		t.Errorf("flag query = %+v, want shorthand q, default empty", query)
	}
	limit := cmd.Flags().Lookup("limit")
	if limit == nil || limit.Shorthand != "l" || limit.DefValue != "50" {
		t.Errorf("flag limit = %+v, want shorthand l, default 50", limit)
	}
	if cmd.Flags().Lookup("skip") != nil {
		t.Error("flag skip should not exist for tag list (SPEC §3.9)")
	}
}

func TestTagList_ArgsUsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "tag", "list", "extra")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestTagList_LimitInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "tag", "list", "--limit", "0")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --limit must be at least 1\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestTagList_TTY(t *testing.T) {
	srv, reqs := tagServer(t, tagListBody)
	out, _, code := runTagList(t, srv)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	for _, want := range []string{"NAME", "UNTAG ON RESOLVE", "backend", "false", "auth", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want contains %q", out, want)
		}
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	got := (*reqs)[0]
	if !strings.HasPrefix(got, "GET /tags?") {
		t.Errorf("request = %q, want GET /tags", got)
	}
	q, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("$top") != "50" || q.Get("$skip") != "" || q.Get("fields") != api.FieldsTagList {
		t.Errorf("query params = %v, want top=50, no skip, fields=%s", q, api.FieldsTagList)
	}
}

func TestTagList_TTY_Query(t *testing.T) {
	srv, reqs := tagServer(t, tagListBody)
	out, _, code := runTagList(t, srv, "-q", "back")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if !strings.Contains(out, "backend") {
		t.Errorf("stdout = %q, want contains backend", out)
	}
	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	q, err := url.ParseQuery(strings.SplitN((*reqs)[0], "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("query") != "back" {
		t.Errorf("query = %q, want back", q.Get("query"))
	}
}

func TestTagList_TTY_Limit(t *testing.T) {
	srv, reqs := tagServer(t, tagListBody)
	_, _, code := runTagList(t, srv, "-l", "5")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	q, err := url.ParseQuery(strings.SplitN((*reqs)[0], "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("$top") != "5" {
		t.Errorf("query = %v, want top=5", q)
	}
}

func TestTagList_TTY_Empty(t *testing.T) {
	srv, _ := tagServer(t, `[]`)
	out, _, code := runTagList(t, srv)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "No tags found\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestTagList_JSON(t *testing.T) {
	srv, _ := tagServer(t, tagListBody)
	out, _, code := runTagList(t, srv, "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("len(tags) = %d, want 2", len(got))
	}
	if got[0]["name"] != "backend" || got[1]["name"] != "auth" {
		t.Errorf("tags = %v, want backend and auth", got)
	}
	if got[0]["untagOnResolve"] != false || got[1]["untagOnResolve"] != true {
		t.Errorf("untagOnResolve = %v, want false/true", got)
	}
}

func TestTagList_JSON_Empty(t *testing.T) {
	srv, _ := tagServer(t, `[]`)
	out, _, code := runTagList(t, srv, "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Errorf("tags = %v, want empty array", got)
	}
}

func TestTagList_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Internal error","error_description":"tags failed"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runTagList(t, ts)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 500 Internal error: tags failed\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestTagRow(t *testing.T) {
	tg := api.Tag{
		Name:           "backend",
		UntagOnResolve: boolPtr(false),
	}
	row := tagRow(tg)
	if len(row) != 2 {
		t.Fatalf("row cols = %d, want 2", len(row))
	}
	want := []string{"backend", "false"}
	for i := range want {
		if row[i] != want[i] {
			t.Errorf("tagRow[%d] = %q, want %q", i, row[i], want[i])
		}
	}
}

func TestTagRow_NilUntagOnResolve(t *testing.T) {
	tg := api.Tag{Name: "auth"}
	row := tagRow(tg)
	if row[1] != "false" {
		t.Errorf("untagOnResolve(nil) = %q, want false", row[1])
	}
}
