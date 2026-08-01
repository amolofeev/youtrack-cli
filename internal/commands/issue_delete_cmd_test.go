package commands

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// issueDeleteServer поднимает фейковый YouTrack: DELETE /issues/{id} → 200 без
// тела, фиксируя запросы. Остальные пути — 404.
func issueDeleteServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	reqs := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reqs = append(*reqs, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/issues/") {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, reqs
}

// runIssueDelete выполняет yt issue delete <args> против фейкового сервера
// (с -y, чтобы не ждать подтверждения).
func runIssueDelete(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "delete", "-y"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestNewIssueDeleteCmd(t *testing.T) {
	cmd := newIssueDeleteCmd()
	if cmd.Use != "delete <id>" {
		t.Errorf("Use = %q, want delete <id>", cmd.Use)
	}
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("flag yes not found")
	}
	if f.Shorthand != "y" {
		t.Errorf("flag yes shorthand = %q, want y", f.Shorthand)
	}

	parent := newIssueCmd()
	var found bool
	for _, c := range parent.Commands() {
		if c.Name() == "delete" {
			found = true
		}
	}
	if !found {
		t.Error("expected \"delete\" subcommand of issue")
	}
}

func TestIssueDelete_TTY(t *testing.T) {
	srv, reqs := issueDeleteServer(t)
	out, _, code := runIssueDelete(t, srv, "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "✓ Deleted issue PRJ-42\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if len(*reqs) != 1 || (*reqs)[0] != "DELETE /issues/PRJ-42" {
		t.Errorf("requests = %v, want [DELETE /issues/PRJ-42]", *reqs)
	}
}

func TestIssueDelete_RingID(t *testing.T) {
	srv, reqs := issueDeleteServer(t)
	_, _, code := runIssueDelete(t, srv, "2-1")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	if len(*reqs) != 1 || (*reqs)[0] != "DELETE /issues/2-1" {
		t.Errorf("requests = %v, want [DELETE /issues/2-1]", *reqs)
	}
}

func TestIssueDelete_NoID(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "delete")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueDelete_ConfirmYes(t *testing.T) {
	srv, reqs := issueDeleteServer(t)
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader("y\n"),
		"issue", "delete", "PRJ-42", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if !strings.Contains(stderr, "! Warning: this will permanently delete PRJ-42. Continue? [y/N] ") {
		t.Errorf("stderr = %q, want confirmation prompt", stderr)
	}
	if len(*reqs) != 1 {
		t.Errorf("requests = %v, want 1 DELETE after confirm", *reqs)
	}
}

func TestIssueDelete_ConfirmNo(t *testing.T) {
	srv, reqs := issueDeleteServer(t)
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader("n\n"),
		"issue", "delete", "PRJ-42", "--base-url", srv.URL)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if !strings.Contains(stderr, "yt: Aborted\n") {
		t.Errorf("stderr = %q, want yt: Aborted", stderr)
	}
	if len(*reqs) != 0 {
		t.Errorf("requests = %v, want none after abort", *reqs)
	}
}

func TestIssueDelete_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Issue PRJ-999 not found","error_description":"no such issue"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueDelete(t, ts, "PRJ-999")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 404 Issue PRJ-999 not found"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want contains %q", stderr, want)
	}
}
