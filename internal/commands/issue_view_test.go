package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/amolofeev/yt/internal/api"
)

// issueViewIssueBody — фикстурный ответ GET /issues/PRJ-42 (поля FieldsIssueView).
const issueViewIssueBody = `{
	"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"Fix login flow",
	"description":"Steps:\n1. Reproduce.\n2. Fix.",
	"created":1782914400000,"updated":1783245600000,"resolved":null,
	"project":{"$type":"Project","id":"0-0","shortName":"PRJ","name":"Demo"},
	"reporter":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex","email":"alex@example.com"},
	"updater":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"},
	"customFields":[{"$type":"EnumIssueCustomField","id":"4-1","name":"State","value":{"$type":"StateBundleElement","id":"5-1","name":"Open"}}],
	"tags":[{"$type":"Tag","id":"6-1","name":"backend"},{"$type":"Tag","id":"6-2","name":"auth"}],
	"commentsCount":2
}`

// issueViewCommentsBody — фикстурный ответ GET /issues/PRJ-42/comments.
const issueViewCommentsBody = `[
	{"$type":"IssueComment","id":"7-1","text":"First comment text.","created":1782914700000,"author":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"}},
	{"$type":"IssueComment","id":"7-2","text":"Second comment text.","created":1783069200000,"author":{"$type":"User","id":"1-1","login":"ivan","fullName":"Ivan"}}
]`

// issueViewServer поднимает фейковый YouTrack, отвечающий на GET /issues/{id}
// и /issues/{id}/comments, и фиксирует запросы.
func issueViewServer(t *testing.T, issueBody, commentsBody string) (*httptest.Server, *[]string) {
	t.Helper()
	reqs := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/comments"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(commentsBody))
		case strings.Contains(r.URL.Path, "/issues/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, reqs
}

// runIssueView выполняет yt issue view <args> против фейкового сервера.
func runIssueView(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "view"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestNewIssueViewCmd_Flags(t *testing.T) {
	cmd := newIssueViewCmd()

	c := cmd.Flags().Lookup("comments")
	if c == nil || c.Shorthand != "c" {
		t.Errorf("flag comments = %+v, want shorthand c", c)
	}

	cl := cmd.Flags().Lookup("comments-limit")
	if cl == nil || cl.Shorthand != "C" {
		t.Errorf("flag comments-limit = %+v, want shorthand C", cl)
	}
	if cl.DefValue != "20" {
		t.Errorf("comments-limit default = %q, want 20", cl.DefValue)
	}
}

func TestIssueView_NoArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "view")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueView_TooManyArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "view", "PRJ-1", "PRJ-2")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueView_CommentsLimitInvalid(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "view", "PRJ-42", "-c", "-C", "0")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --comments-limit must be at least 1"; stderr != want+"\n" {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueView_TTY_WithComments(t *testing.T) {
	srv, reqs := issueViewServer(t, issueViewIssueBody, issueViewCommentsBody)
	out, _, code := runIssueView(t, srv, "PRJ-42", "-c", "-C", "5")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	sep := strings.Repeat("─", 64)
	want := "PRJ-42  Fix login flow\n" +
		sep + "\n" +
		"STATE: Open  PROJECT: PRJ  REPORTER: alex\n" +
		"CREATED: 2026-07-01 14:00  UPDATED: 2026-07-05 10:00\n" +
		"Tags: backend, auth\n" +
		sep + "\n" +
		"Steps:\n1. Reproduce.\n2. Fix.\n" +
		"\n" +
		"Comments (2):\n" +
		strings.Repeat("─", 11) + "\n" +
		"alex · 2026-07-01 14:05\n" +
		"First comment text.\n" +
		"\n" +
		"ivan · 2026-07-03 09:00\n" +
		"Second comment text.\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}

	if len(*reqs) != 2 {
		t.Fatalf("requests = %v, want 2 (issue + comments)", *reqs)
	}
	if got := (*reqs)[0]; !strings.HasPrefix(got, "GET /issues/PRJ-42?") {
		t.Errorf("issue request = %q, want GET /issues/PRJ-42", got)
	}
	if q, err := url.ParseQuery(strings.SplitN((*reqs)[0], "?", 2)[1]); err != nil {
		t.Fatalf("parse issue query: %v", err)
	} else if q.Get("fields") != api.FieldsIssueView {
		t.Errorf("issue fields = %q, want %q", q.Get("fields"), api.FieldsIssueView)
	}

	got := (*reqs)[1]
	if !strings.HasPrefix(got, "GET /issues/PRJ-42/comments?") {
		t.Errorf("comments request = %q, want GET /issues/PRJ-42/comments", got)
	}
	if q, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1]); err != nil {
		t.Fatalf("parse comments query: %v", err)
	} else {
		if q.Get("$top") != "5" {
			t.Errorf("comments $top = %q, want 5 (--comments-limit)", q.Get("$top"))
		}
		if q.Get("fields") != api.FieldsIssueComments {
			t.Errorf("comments fields = %q, want %q", q.Get("fields"), api.FieldsIssueComments)
		}
	}
}

func TestIssueView_TTY_Minimal(t *testing.T) {
	const minimal = `{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"","description":"","created":0,"updated":0,"project":null,"reporter":null,"customFields":null,"tags":null}`
	srv, _ := issueViewServer(t, minimal, `[]`)
	out, _, code := runIssueView(t, srv, "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	want := "PRJ-42\n" + strings.Repeat("─", 64) + "\nNo description\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
}

func TestIssueView_JSON_NoComments(t *testing.T) {
	srv, _ := issueViewServer(t, issueViewIssueBody, `[]`)
	out, _, code := runIssueView(t, srv, "PRJ-42", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if got["idReadable"] != "PRJ-42" || got["summary"] != "Fix login flow" {
		t.Errorf("issue = %v, want PRJ-42 / Fix login flow", got)
	}
	if _, ok := got["comments"]; ok {
		t.Errorf("comments key present without --comments: %v", got)
	}
}

func TestIssueView_JSON_WithComments(t *testing.T) {
	srv, _ := issueViewServer(t, issueViewIssueBody, issueViewCommentsBody)
	out, _, code := runIssueView(t, srv, "PRJ-42", "--json", "-c")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	cm, ok := got["comments"].([]any)
	if !ok {
		t.Fatalf("comments = %#v, want array", got["comments"])
	}
	if len(cm) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(cm))
	}
	first, ok := cm[0].(map[string]any)
	if !ok || first["text"] != "First comment text." {
		t.Errorf("comments[0] = %v, want text %q", cm[0], "First comment text.")
	}
}

func TestIssueView_JSON_CommentsEmptyList(t *testing.T) {
	srv, _ := issueViewServer(t, issueViewIssueBody, `[]`)
	out, _, code := runIssueView(t, srv, "PRJ-42", "--json", "-c")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	cm, ok := got["comments"].([]any)
	if !ok || len(cm) != 0 {
		t.Errorf("comments = %#v, want empty array", got["comments"])
	}
}

func TestIssueView_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Issue NOPE not found","error_description":"no such issue"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueView(t, ts, "NOPE")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 404 Issue NOPE not found"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want contains %q", stderr, want)
	}
	if stderr[0] != 'y' {
		t.Errorf("stderr = %q, want starts with yt:", stderr)
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{1782914400000, "2026-07-01 14:00"},
		{1783069200000, "2026-07-03 09:00"},
		{0, ""},
	}
	for _, tt := range tests {
		if got := formatDateTime(tt.in); got != tt.want {
			t.Errorf("formatDateTime(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIssueViewMeta1(t *testing.T) {
	i := api.Issue{
		Reporter: &api.User{Login: "alex"},
		Project:  &api.Project{ShortName: "PRJ"},
		CustomFields: []api.IssueCustomField{{
			Name:  "State",
			Value: json.RawMessage(`{"$type":"StateBundleElement","name":"Open"}`),
		}},
	}
	if got, want := issueViewMeta1(i), "STATE: Open  PROJECT: PRJ  REPORTER: alex"; got != want {
		t.Errorf("issueViewMeta1 = %q, want %q", got, want)
	}

	empty := api.Issue{}
	if got := issueViewMeta1(empty); got != "" {
		t.Errorf("issueViewMeta1(empty) = %q, want empty", got)
	}
}

func TestIssueViewMeta2(t *testing.T) {
	i := api.Issue{Created: 1782914400000, Updated: 1783245600000}
	if got, want := issueViewMeta2(i), "CREATED: 2026-07-01 14:00  UPDATED: 2026-07-05 10:00"; got != want {
		t.Errorf("issueViewMeta2 = %q, want %q", got, want)
	}
	if got := issueViewMeta2(api.Issue{}); got != "" {
		t.Errorf("issueViewMeta2(empty) = %q, want empty", got)
	}
}

func TestIssueViewTags(t *testing.T) {
	i := api.Issue{Tags: []api.Tag{{Name: "backend"}, {Name: "auth"}, {}}}
	if got, want := issueViewTags(i), "Tags: backend, auth"; got != want {
		t.Errorf("issueViewTags = %q, want %q", got, want)
	}
	if got := issueViewTags(api.Issue{}); got != "" {
		t.Errorf("issueViewTags(empty) = %q, want empty", got)
	}
}

func TestCommentAuthor(t *testing.T) {
	tests := []struct {
		name string
		c    api.IssueComment
		want string
	}{
		{"login", api.IssueComment{Author: &api.User{Login: "alex", FullName: "Alex"}}, "alex"},
		{"fullName only", api.IssueComment{Author: &api.User{FullName: "Alex"}}, "Alex"},
		{"nil author", api.IssueComment{}, "unknown"},
	}
	for _, tt := range tests {
		if got := commentAuthor(tt.c); got != tt.want {
			t.Errorf("commentAuthor(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
