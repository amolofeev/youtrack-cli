package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/amolofeev/yt/internal/api"
)

// commentCreateBody — фикстурный ответ POST /issues/{id}/comments
// (FieldsIssueCommentCreate).
const commentCreateBody = `{"$type":"IssueComment","id":"7-1","text":"Fix login flow","created":1782914700000,"author":{"$type":"User","id":"1-1","login":"alex"}}`

// commentServer поднимает фейковый YouTrack, отвечающий на запросы
// /issues/{id}/comments, и фиксирует запросы и тела.
func commentServer(t *testing.T, body string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	reqs := &[]string{}
	bodies := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*reqs = append(*reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		*bodies = append(*bodies, string(data))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, reqs, bodies
}

// runIssueCommentList выполняет yt issue comment list <args> против фейкового
// сервера.
func runIssueCommentList(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "comment", "list"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

// runIssueCommentCreate выполняет yt issue comment create <args> против
// фейкового сервера.
func runIssueCommentCreate(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "comment", "create"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestNewIssueCommentCmd(t *testing.T) {
	cmd := newIssueCommentCmd()
	if cmd.Use != "comment" {
		t.Errorf("Use = %q, want comment", cmd.Use)
	}
	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "create"} {
		if !names[want] {
			t.Errorf("subcommand %q not found in issue comment", want)
		}
	}
}

func TestNewIssueCommentListCmd_Flags(t *testing.T) {
	cmd := newIssueCommentListCmd()
	limit := cmd.Flags().Lookup("limit")
	if limit == nil || limit.DefValue != "30" {
		t.Errorf("flag limit = %+v, want default 30", limit)
	}
	skip := cmd.Flags().Lookup("skip")
	if skip == nil || skip.DefValue != "0" {
		t.Errorf("flag skip = %+v, want default 0", skip)
	}
}

func TestNewIssueCommentCreateCmd_Flags(t *testing.T) {
	cmd := newIssueCommentCreateCmd()
	msg := cmd.Flags().Lookup("message")
	if msg == nil || msg.Shorthand != "m" {
		t.Errorf("flag message = %+v, want shorthand m", msg)
	}
	if ed := cmd.Flags().Lookup("editor"); ed == nil {
		t.Error("flag editor not found")
	}
}

func TestIssueCommentList_NoArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "list")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueCommentList_TooManyArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "list", "PRJ-1", "PRJ-2")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueCommentList_LimitInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "list", "PRJ-42", "--limit", "0")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --limit must be at least 1\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCommentList_SkipInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "list", "PRJ-42", "--skip", "-1")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --skip must be non-negative\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCommentCreate_MissingID_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "create", "-m", "hello")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueCommentCreate_MissingMessage_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "create", "PRJ-42")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --message is required (or use --editor)\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCommentCreate_MessageAndEditorMutuallyExclusive(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "comment", "create", "PRJ-42", "-m", "x", "--editor")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --message and --editor are mutually exclusive\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCommentList_TTY(t *testing.T) {
	srv, reqs, _ := commentServer(t, issueViewCommentsBody)
	out, _, code := runIssueCommentList(t, srv, "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	want := "alex · 2026-07-01 14:05\n" +
		"First comment text.\n" +
		strings.Repeat("─", 4) + "\n" +
		"ivan · 2026-07-03 09:00\n" +
		"Second comment text.\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	got := (*reqs)[0]
	if !strings.HasPrefix(got, "GET /issues/PRJ-42/comments?") {
		t.Errorf("request = %q, want GET /issues/PRJ-42/comments", got)
	}
	q, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("$top") != "30" || q.Get("$skip") != "" || q.Get("fields") != api.FieldsIssueComments {
		t.Errorf("query = %v, want top=30, no skip, fields=%s", q, api.FieldsIssueComments)
	}
}

func TestIssueCommentList_TTY_LimitSkip(t *testing.T) {
	srv, reqs, _ := commentServer(t, issueViewCommentsBody)
	_, _, code := runIssueCommentList(t, srv, "PRJ-42", "--limit", "5", "--skip", "2")
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

func TestIssueCommentList_TTY_Empty(t *testing.T) {
	srv, _, _ := commentServer(t, `[]`)
	out, _, code := runIssueCommentList(t, srv, "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "No comments for PRJ-42\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestIssueCommentList_JSON(t *testing.T) {
	srv, _, _ := commentServer(t, issueViewCommentsBody)
	out, _, code := runIssueCommentList(t, srv, "PRJ-42", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(got))
	}
	if got[0]["text"] != "First comment text." {
		t.Errorf("comments[0] = %v, want First comment text.", got[0])
	}
}

func TestIssueCommentList_JSON_Empty(t *testing.T) {
	srv, _, _ := commentServer(t, `[]`)
	out, _, code := runIssueCommentList(t, srv, "PRJ-42", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Errorf("comments = %v, want empty array", got)
	}
}

func TestIssueCommentCreate_TTY(t *testing.T) {
	srv, reqs, bodies := commentServer(t, commentCreateBody)
	out, _, code := runIssueCommentCreate(t, srv, "PRJ-42", "-m", "Fix login flow")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "✓ Added comment to PRJ-42\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	got := (*reqs)[0]
	if !strings.HasPrefix(got, "POST /issues/PRJ-42/comments?") {
		t.Errorf("request = %q, want POST /issues/PRJ-42/comments", got)
	}
	q, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("fields") != api.FieldsIssueCommentCreate {
		t.Errorf("fields = %q, want %q", q.Get("fields"), api.FieldsIssueCommentCreate)
	}

	if len(*bodies) != 1 {
		t.Fatalf("bodies = %v, want 1", *bodies)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil {
		t.Fatalf("POST body is not valid JSON: %v\n%s", err, (*bodies)[0])
	}
	if sent["text"] != "Fix login flow" {
		t.Errorf("body = %v, want text Fix login flow", sent)
	}
}

func TestIssueCommentCreate_JSON(t *testing.T) {
	srv, _, _ := commentServer(t, commentCreateBody)
	out, _, code := runIssueCommentCreate(t, srv, "PRJ-42", "-m", "Fix login flow", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if got["text"] != "Fix login flow" {
		t.Errorf("comment = %v, want text Fix login flow", got)
	}
}

func TestIssueCommentCreate_Editor(t *testing.T) {
	srv, _, bodies := commentServer(t, commentCreateBody)
	t.Setenv("EDITOR", fakeEditorScript(t, "Fix login flow\n"))
	_, _, code := runIssueCommentCreate(t, srv, "PRJ-42", "--editor")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil {
		t.Fatalf("POST body is not valid JSON: %v\n%s", err, (*bodies)[0])
	}
	if sent["text"] != "Fix login flow" {
		t.Errorf("body = %v, want text Fix login flow (from editor)", sent)
	}
}

func TestIssueCommentCreate_EditorEmptyText(t *testing.T) {
	srv, _, _ := commentServer(t, commentCreateBody)
	t.Setenv("EDITOR", fakeEditorScript(t, "   \n"))
	_, stderr, code := runIssueCommentCreate(t, srv, "PRJ-42", "--editor")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: no comment text provided\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCommentCreate_Error400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Bad request","error_description":"comment text is required"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueCommentCreate(t, ts, "PRJ-42", "-m", "x")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 400 Bad request: comment text is required\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCommentList_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Issue NOPE not found","error_description":"no such issue"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueCommentList(t, ts, "NOPE")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 404 Issue NOPE not found"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want contains %q", stderr, want)
	}
}
