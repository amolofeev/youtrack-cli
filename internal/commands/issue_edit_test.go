package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/amolofeev/youtrack-cli/internal/api"
)

// issueEditBody — фикстурный ответ POST /issues/{id} (FieldsIssueEdit).
const issueEditBody = `{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"New summary","description":"New description"}`

// issueEditServer поднимает фейковый YouTrack: POST /issues/PRJ-42, фиксируя
// запросы и тела.
func issueEditServer(t *testing.T, body string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	reqs := &[]string{}
	bodies := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*reqs = append(*reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		*bodies = append(*bodies, string(data))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/issues/PRJ-42" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, reqs, bodies
}

// runIssueEdit выполняет yt issue edit PRJ-42 <args> против фейкового сервера.
func runIssueEdit(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "edit", "PRJ-42"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

// editQuery разбирает query-строку из захваченного запроса.
func editQuery(t *testing.T, req string) url.Values {
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

func TestNewIssueEditCmd_Flags(t *testing.T) {
	cmd := newIssueEditCmd()
	if cmd.Use != "edit <id>" {
		t.Errorf("Use = %q, want edit <id>", cmd.Use)
	}
	for _, name := range []string{"title", "body"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag %q not found", name)
			continue
		}
		if f.Shorthand != "" {
			t.Errorf("flag %q shorthand = %q, want empty", name, f.Shorthand)
		}
	}
}

func TestIssueEdit_NoFlags(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "edit", "PRJ-42")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: at least one of --title, --body is required\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueEdit_TTY(t *testing.T) {
	srv, reqs, bodies := issueEditServer(t, issueEditBody)
	out, _, code := runIssueEdit(t, srv, "--title", "New summary", "--body", "New description")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "✓ Updated issue PRJ-42\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1 (POST /issues/PRJ-42)", *reqs)
	}
	req := (*reqs)[0]
	if !strings.HasPrefix(req, "POST /issues/PRJ-42?") {
		t.Errorf("request = %q, want POST /issues/PRJ-42", req)
	}
	if q := editQuery(t, req); q.Get("fields") != api.FieldsIssueEdit {
		t.Errorf("fields = %q, want %q", q.Get("fields"), api.FieldsIssueEdit)
	}

	body := decodeSentBody(t, bodies)
	if body["summary"] != "New summary" || body["description"] != "New description" {
		t.Errorf("body = %v, want summary and description", body)
	}
}

func TestIssueEdit_TitleOnly(t *testing.T) {
	srv, reqs, bodies := issueEditServer(t, issueEditBody)
	_, _, code := runIssueEdit(t, srv, "--title", "New summary")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	body := decodeSentBody(t, bodies)
	if body["summary"] != "New summary" {
		t.Errorf("summary = %v, want New summary", body["summary"])
	}
	if _, ok := body["description"]; ok {
		t.Errorf("description present in body, want omitted (partial update): %v", body)
	}
}

func TestIssueEdit_BodyOnly(t *testing.T) {
	srv, reqs, bodies := issueEditServer(t, issueEditBody)
	_, _, code := runIssueEdit(t, srv, "--body", "New description")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	body := decodeSentBody(t, bodies)
	if body["description"] != "New description" {
		t.Errorf("description = %v, want New description", body["description"])
	}
	if _, ok := body["summary"]; ok {
		t.Errorf("summary present in body, want omitted (partial update): %v", body)
	}
}

func TestIssueEdit_JSON(t *testing.T) {
	srv, _, _ := issueEditServer(t, issueEditBody)
	out, _, code := runIssueEdit(t, srv, "--title", "New summary", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if got["idReadable"] != "PRJ-42" || got["summary"] != "New summary" || got["description"] != "New description" {
		t.Errorf("issue = %v, want PRJ-42 / New summary / New description", got)
	}
}

func TestIssueEdit_Error400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Bad request","error_description":"summary cannot be empty"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueEdit(t, ts, "--title", "New summary")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 400 Bad request: summary cannot be empty\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}
