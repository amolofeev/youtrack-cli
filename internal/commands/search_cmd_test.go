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

// searchIssuesBody — фикстурный ответ GET /issues (поля FieldsIssueList).
const searchIssuesBody = `[
	{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"Fix login flow","created":1782864000000,"updated":1783296000000,"project":{"$type":"Project","id":"0-0","shortName":"PRJ","name":"Demo"},"reporter":{"$type":"User","id":"1-1","login":"alice","fullName":"Alice"},"customFields":[{"$type":"EnumIssueCustomField","id":"4-1","name":"State","value":{"$type":"StateBundleElement","id":"5-1","name":"Done"}}]},
	{"$type":"Issue","id":"2-2","idReadable":"PRJ-43","summary":"Add dark mode","created":1782914400000,"updated":1783069200000,"project":{"$type":"Project","id":"0-0","shortName":"PRJ","name":"Demo"},"reporter":{"$type":"User","id":"1-1","login":"bob","fullName":"Bob"},"customFields":[{"$type":"EnumIssueCustomField","id":"4-1","name":"State","value":{"$type":"StateBundleElement","id":"5-1","name":"Open"}}]}
]`

// searchServer поднимает фейковый YouTrack, отвечающий на GET /issues, и
// фиксирует запросы.
func searchServer(t *testing.T, body string) (*httptest.Server, *[]string) {
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

// runSearch выполняет yt search <args> против фейкового сервера.
func runSearch(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"search"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestSearchRegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	sub, _, err := root.Find([]string{"search"})
	if err != nil {
		t.Fatalf("find search: %v", err)
	}
	if sub.GroupID != "issues" {
		t.Errorf("search.GroupID = %q, want %q", sub.GroupID, "issues")
	}
}

func TestNewSearchCmd_Flags(t *testing.T) {
	cmd := newSearchCmd()

	limit := cmd.Flags().Lookup("limit")
	if limit == nil || limit.Shorthand != "l" || limit.DefValue != "30" {
		t.Errorf("flag limit = %+v, want shorthand l, default 30", limit)
	}
	skip := cmd.Flags().Lookup("skip")
	if skip == nil || skip.DefValue != "0" {
		t.Errorf("flag skip = %+v, want default 0", skip)
	}
}

func TestSearch_NoArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "search")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestSearch_TooManyArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "search", "query one", "query two")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestSearch_LimitInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "search", "x", "--limit", "0")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --limit must be between 1 and 100\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestSearch_SkipInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "search", "x", "--skip", "-1")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --skip must be non-negative\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestSearch_TTY(t *testing.T) {
	srv, reqs := searchServer(t, searchIssuesBody)
	out, _, code := runSearch(t, srv, "project: PRJ has: open")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	for _, want := range []string{"ID", "STATE", "PRJ-42", "Fix login flow", "Done", "alice", "PRJ-43", "Add dark mode"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want contains %q", out, want)
		}
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	got := (*reqs)[0]
	if !strings.HasPrefix(got, "GET /issues?") {
		t.Errorf("request = %q, want GET /issues", got)
	}
	q, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("query") != "project: PRJ has: open" {
		t.Errorf("query = %q, want raw query as-is", q.Get("query"))
	}
	if q.Get("$top") != "30" || q.Get("$skip") != "" || q.Get("fields") != api.FieldsIssueList {
		t.Errorf("query params = %v, want top=30, no skip, fields=%s", q, api.FieldsIssueList)
	}
}

func TestSearch_TTY_LimitSkip(t *testing.T) {
	srv, reqs := searchServer(t, searchIssuesBody)
	_, _, code := runSearch(t, srv, "x", "-l", "5", "--skip", "2")
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
	if q.Get("$top") != "5" || q.Get("$skip") != "2" {
		t.Errorf("query = %v, want top=5, skip=2", q)
	}
}

func TestSearch_TTY_Empty(t *testing.T) {
	srv, _ := searchServer(t, `[]`)
	out, _, code := runSearch(t, srv, "nonexistent query")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "No issues found for query \"nonexistent query\"\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestSearch_JSON(t *testing.T) {
	srv, _ := searchServer(t, searchIssuesBody)
	out, _, code := runSearch(t, srv, "project: PRJ", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("len(issues) = %d, want 2", len(got))
	}
	if got[0]["idReadable"] != "PRJ-42" || got[1]["idReadable"] != "PRJ-43" {
		t.Errorf("issues = %v, want PRJ-42 and PRJ-43", got)
	}
}

func TestSearch_JSON_Empty(t *testing.T) {
	srv, _ := searchServer(t, `[]`)
	out, _, code := runSearch(t, srv, "x", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Errorf("issues = %v, want empty array", got)
	}
}

func TestSearch_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Internal error","error_description":"search failed"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runSearch(t, ts, "x")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 500 Internal error: search failed\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}
