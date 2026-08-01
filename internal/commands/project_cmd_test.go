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

// projectListBody — фикстурный ответ GET /admin/projects (поля FieldsProjectList).
const projectListBody = `[
	{"$type":"Project","id":"0-0","shortName":"PRJ","name":"Project One","archived":false,"leader":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"}},
	{"$type":"Project","id":"0-1","shortName":"DEMO","name":"Demo project","archived":true,"leader":{"$type":"User","id":"1-2","login":"bob","fullName":"Bob"}}
]`

// projectServer поднимает фейковый YouTrack, отвечающий на GET /admin/projects,
// и фиксирует запросы.
func projectServer(t *testing.T, body string) (*httptest.Server, *[]string) {
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

// runProjectList выполняет yt project list <args> против фейкового сервера.
func runProjectList(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"project", "list"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestProjectRegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	sub, _, err := root.Find([]string{"project"})
	if err != nil {
		t.Fatalf("find project: %v", err)
	}
	if sub.GroupID != "server" {
		t.Errorf("project.GroupID = %q, want %q", sub.GroupID, "server")
	}
}

func TestNewProjectCmd(t *testing.T) {
	cmd := newProjectCmd()
	if cmd.Use != "project" {
		t.Errorf("Use = %q, want project", cmd.Use)
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

func TestNewProjectListCmd_Flags(t *testing.T) {
	cmd := newProjectListCmd()

	limit := cmd.Flags().Lookup("limit")
	if limit == nil || limit.Shorthand != "l" || limit.DefValue != "50" {
		t.Errorf("flag limit = %+v, want shorthand l, default 50", limit)
	}
	skip := cmd.Flags().Lookup("skip")
	if skip == nil || skip.DefValue != "0" {
		t.Errorf("flag skip = %+v, want default 0", skip)
	}
}

func TestProjectList_ArgsUsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "project", "list", "extra")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestProjectList_LimitInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "project", "list", "--limit", "0")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --limit must be at least 1\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestProjectList_SkipInvalid(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "project", "list", "--skip", "-1")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: --skip must be non-negative\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestProjectList_TTY(t *testing.T) {
	srv, reqs := projectServer(t, projectListBody)
	out, _, code := runProjectList(t, srv)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}

	for _, want := range []string{"SHORTNAME", "NAME", "ARCHIVED", "LEADER", "PRJ", "Project One", "false", "alex", "DEMO", "Demo project", "true", "bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want contains %q", out, want)
		}
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	got := (*reqs)[0]
	if !strings.HasPrefix(got, "GET /admin/projects?") {
		t.Errorf("request = %q, want GET /admin/projects", got)
	}
	q, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("$top") != "50" || q.Get("$skip") != "" || q.Get("fields") != api.FieldsProjectList {
		t.Errorf("query params = %v, want top=50, no skip, fields=%s", q, api.FieldsProjectList)
	}
}

func TestProjectList_TTY_LimitSkip(t *testing.T) {
	srv, reqs := projectServer(t, projectListBody)
	_, _, code := runProjectList(t, srv, "-l", "5", "--skip", "2")
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

func TestProjectList_TTY_Empty(t *testing.T) {
	srv, _ := projectServer(t, `[]`)
	out, _, code := runProjectList(t, srv)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "No projects found\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestProjectList_JSON(t *testing.T) {
	srv, _ := projectServer(t, projectListBody)
	out, _, code := runProjectList(t, srv, "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(got))
	}
	if got[0]["shortName"] != "PRJ" || got[1]["shortName"] != "DEMO" {
		t.Errorf("projects = %v, want PRJ and DEMO", got)
	}
	leader, ok := got[0]["leader"].(map[string]any)
	if !ok || leader["login"] != "alex" {
		t.Errorf("leader = %v, want login alex", got[0]["leader"])
	}
}

func TestProjectList_JSON_Empty(t *testing.T) {
	srv, _ := projectServer(t, `[]`)
	out, _, code := runProjectList(t, srv, "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Errorf("projects = %v, want empty array", got)
	}
}

func TestProjectList_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Internal error","error_description":"projects failed"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runProjectList(t, ts)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 500 Internal error: projects failed\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestProjectRow(t *testing.T) {
	pr := api.Project{
		ShortName: "PRJ",
		Name:      "Project One",
		Archived:  boolPtr(false),
		Leader:    &api.User{Login: "alex"},
	}
	row := projectRow(pr)
	if len(row) != 4 {
		t.Fatalf("row cols = %d, want 4", len(row))
	}
	want := []string{"PRJ", "Project One", "false", "alex"}
	for i := range want {
		if row[i] != want[i] {
			t.Errorf("projectRow[%d] = %q, want %q", i, row[i], want[i])
		}
	}
}

func TestProjectRow_NilLeader(t *testing.T) {
	pr := api.Project{ShortName: "PRJ"}
	row := projectRow(pr)
	if row[3] != "" {
		t.Errorf("leader(nil) = %q, want empty", row[3])
	}
}

func boolPtr(b bool) *bool { return &b }
