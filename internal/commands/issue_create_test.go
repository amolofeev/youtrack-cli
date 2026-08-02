package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amolofeev/youtrack-cli/internal/api"
)

// issueCreateProjectsBody — фикстурный ответ GET /admin/projects (FieldsProjectResolve).
const issueCreateProjectsBody = `[
	{"$type":"Project","id":"0-0","name":"Demo","shortName":"PRJ"},
	{"$type":"Project","id":"0-1","name":"Alpha","shortName":"ALP"}
]`

// issueCreateBody — фикстурный ответ POST /issues (FieldsIssueCreate).
const issueCreateBody = `{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"Fix login flow","project":{"$type":"Project","id":"0-0","shortName":"PRJ"}}`

// issueCreateServer поднимает фейковый YouTrack: GET /admin/projects и
// POST /issues, фиксируя запросы и тела.
func issueCreateServer(t *testing.T, projectsBody, createdBody string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	reqs := &[]string{}
	bodies := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*reqs = append(*reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		*bodies = append(*bodies, string(data))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/admin/projects":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(projectsBody))
		case r.Method == http.MethodPost && r.URL.Path == "/issues":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(createdBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, reqs, bodies
}

// runIssueCreate выполняет yt issue create <args> против фейкового сервера.
func runIssueCreate(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "create"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

// createQuery разбирает query-строку из захваченного запроса.
func createQuery(t *testing.T, req string) url.Values {
	t.Helper()
	i := strings.Index(req, "?")
	if i < 0 {
		t.Fatalf("request without query: %q", req)
	}
	q, err := url.ParseQuery(req[i+1:])
	if err != nil {
		t.Fatalf("ParseQuery(%q) error: %v", req[i+1:], err)
	}
	return q
}

// decodeSentBody разбирает последнее захваченное тело запроса (POST /issues).
func decodeSentBody(t *testing.T, bodies *[]string) map[string]any {
	t.Helper()
	if len(*bodies) == 0 {
		t.Fatal("no request bodies captured")
	}
	var body map[string]any
	raw := (*bodies)[len(*bodies)-1]
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("POST body is not valid JSON: %v\n%s", err, raw)
	}
	return body
}

func TestNewIssueCreateCmd_Flags(t *testing.T) {
	cmd := newIssueCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("Use = %q, want create", cmd.Use)
	}
	checks := map[string]string{"project": "p", "title": "t", "body": "b", "editor": ""}
	for name, sh := range checks {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag %q not found", name)
			continue
		}
		if f.Shorthand != sh {
			t.Errorf("flag %q shorthand = %q, want %q", name, f.Shorthand, sh)
		}
	}
}

func TestIssueCreate_MissingProject(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "create", "-t", "T")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --project is required\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCreate_MissingTitle(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "create", "-p", "PRJ")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --title is required (or use --editor)\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCreate_BodyAndEditorMutuallyExclusive(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "create", "-p", "PRJ", "-t", "T", "-b", "B", "--editor")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --body and --editor are mutually exclusive\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCreate_TTY(t *testing.T) {
	srv, reqs, bodies := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	out, _, code := runIssueCreate(t, srv, "-p", "prj", "-t", "Fix login flow", "-b", "Steps")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "✓ Created issue PRJ-42: Fix login flow\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}

	if len(*reqs) != 2 {
		t.Fatalf("requests = %v, want 2 (resolve + create)", *reqs)
	}
	if !strings.HasPrefix((*reqs)[0], "GET /admin/projects?") {
		t.Errorf("resolve request = %q, want GET /admin/projects", (*reqs)[0])
	}
	q := createQuery(t, (*reqs)[0])
	if q.Get("$top") != "200" {
		t.Errorf("resolve $top = %q, want 200", q.Get("$top"))
	}
	if q.Get("fields") != api.FieldsProjectResolve {
		t.Errorf("resolve fields = %q, want %q", q.Get("fields"), api.FieldsProjectResolve)
	}

	if !strings.HasPrefix((*reqs)[1], "POST /issues?") {
		t.Errorf("create request = %q, want POST /issues", (*reqs)[1])
	}
	if q := createQuery(t, (*reqs)[1]); q.Get("fields") != api.FieldsIssueCreate {
		t.Errorf("create fields = %q, want %q", q.Get("fields"), api.FieldsIssueCreate)
	}

	body := decodeSentBody(t, bodies)
	proj, ok := body["project"].(map[string]any)
	if !ok || proj["id"] != "0-0" {
		t.Errorf("project = %v, want {id: 0-0} (shortName prj resolved)", body["project"])
	}
	if body["summary"] != "Fix login flow" || body["description"] != "Steps" {
		t.Errorf("body = %v, want summary and description", body)
	}
}

func TestIssueCreate_JSON(t *testing.T) {
	srv, _, _ := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	out, _, code := runIssueCreate(t, srv, "-p", "PRJ", "-t", "Fix login flow", "--json")
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
}

func TestIssueCreate_RingIDSkipsResolve(t *testing.T) {
	srv, reqs, bodies := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	_, _, code := runIssueCreate(t, srv, "-p", "0-0", "-t", "T")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	if len(*reqs) != 1 || !strings.HasPrefix((*reqs)[0], "POST /issues") {
		t.Errorf("requests = %v, want only POST /issues", *reqs)
	}
	body := decodeSentBody(t, bodies)
	proj, ok := body["project"].(map[string]any)
	if !ok || proj["id"] != "0-0" {
		t.Errorf("project = %v, want {id: 0-0} as-is", body["project"])
	}
}

func TestIssueCreate_ProjectNotFound(t *testing.T) {
	srv, _, _ := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	_, stderr, code := runIssueCreate(t, srv, "-p", "NOPE", "-t", "T")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: project NOPE not found\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCreate_Error400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/admin/projects":
			_, _ = w.Write([]byte(issueCreateProjectsBody))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Bad request","error_description":"summary is required"}`))
		}
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueCreate(t, ts, "-p", "PRJ", "-t", "T")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 400 Bad request: summary is required\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestResolveProject(t *testing.T) {
	c := resolveTestClient(t, issueCreateProjectsBody)

	// ring-id используется как есть, без обращения к API.
	if got, err := resolveProject(context.Background(), c, "0-0"); err != nil || got != "0-0" {
		t.Errorf("ring-id: got %q, err %v; want 0-0", got, err)
	}
	// shortName без учёта регистра.
	if got, err := resolveProject(context.Background(), c, "prj"); err != nil || got != "0-0" {
		t.Errorf("shortName: got %q, err %v; want 0-0", got, err)
	}
	// name.
	if got, err := resolveProject(context.Background(), c, "Alpha"); err != nil || got != "0-1" {
		t.Errorf("name: got %q, err %v; want 0-1", got, err)
	}
	// не найдено.
	if _, err := resolveProject(context.Background(), c, "NOPE"); err == nil || !strings.Contains(err.Error(), "project NOPE not found") {
		t.Errorf("not found: err = %v, want project NOPE not found", err)
	}
}

// resolveTestClient создаёт клиент с фейковым GET /admin/projects.
func resolveTestClient(t *testing.T, body string) *api.Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	c, err := api.New(ts.URL, "perm:test")
	if err != nil {
		t.Fatalf("api.New error: %v", err)
	}
	return c
}

// fakeEditorScript создаёт исполняемый скрипт-редактор, записывающий в файл
// (argv[1]) фиксированное содержимое content.
func fakeEditorScript(t *testing.T, content string) string {
	t.Helper()
	script := "#!/bin/sh\ncat > \"$1\" <<'YTEOF'\n" + content + "\nYTEOF\n"
	path := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	return path
}

func TestIssueCreate_Editor(t *testing.T) {
	srv, _, bodies := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	t.Setenv("EDITOR", fakeEditorScript(t, "Summary: Fix login flow\n\nDescription:\nSteps:\n1. Reproduce."))
	_, _, code := runIssueCreate(t, srv, "-p", "PRJ", "--editor")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	body := decodeSentBody(t, bodies)
	if body["summary"] != "Fix login flow" {
		t.Errorf("summary = %v, want Fix login flow", body["summary"])
	}
	if body["description"] != "Steps:\n1. Reproduce." {
		t.Errorf("description = %q, want Steps:\\n1. Reproduce.", body["description"])
	}
}

func TestIssueCreate_EditorNoSummary(t *testing.T) {
	srv, _, _ := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	t.Setenv("EDITOR", fakeEditorScript(t, "Summary: \n\nDescription:\n"))
	_, stderr, code := runIssueCreate(t, srv, "-p", "PRJ", "--editor")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: no summary provided\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueCreate_EditorFallsBackToVi(t *testing.T) {
	srv, _, bodies := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	dir := t.TempDir()
	script := "#!/bin/sh\ncat > \"$1\" <<'YTEOF'\nSummary: Via vi\n\nDescription:\nbody\nYTEOF\n"
	if err := os.WriteFile(filepath.Join(dir, "vi"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake vi: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("EDITOR", "")
	_, _, code := runIssueCreate(t, srv, "-p", "PRJ", "--editor")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	body := decodeSentBody(t, bodies)
	if body["summary"] != "Via vi" {
		t.Errorf("summary = %v, want Via vi (fallback to vi)", body["summary"])
	}
}

func TestEditorTemplate(t *testing.T) {
	if got := editorTemplate(""); got != "Summary: \n\nDescription:\n" {
		t.Errorf("editorTemplate(\"\") = %q, want %q", got, "Summary: \n\nDescription:\n")
	}
	if got := editorTemplate("Fix login flow"); got != "Summary: Fix login flow\n\nDescription:\n" {
		t.Errorf("editorTemplate(title) = %q, want prefilled Summary", got)
	}
}

func TestParseEditorContent(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantSummary string
		wantDesc    string
	}{
		{
			"full",
			"Summary: Fix login flow\n\nDescription:\nSteps:\n1. Reproduce.",
			"Fix login flow",
			"Steps:\n1. Reproduce.",
		},
		{"empty description", "Summary: T\n\nDescription:\n", "T", ""},
		{"empty summary", "Summary: \n\nDescription:\n", "", ""},
		{"trailing newline", "Summary: T\n\nDescription:\nx\n", "T", "x"},
		{"crlf", "Summary: T\r\n\r\nDescription:\r\nbody\r\n", "T", "body"},
		{"no description label", "Summary: T\njust a summary", "T", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, desc := parseEditorContent(tc.content)
			if summary != tc.wantSummary || desc != tc.wantDesc {
				t.Errorf("parseEditorContent = (%q, %q), want (%q, %q)", summary, desc, tc.wantSummary, tc.wantDesc)
			}
		})
	}
}
