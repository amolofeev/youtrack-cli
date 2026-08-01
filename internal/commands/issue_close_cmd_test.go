package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/amolofeev/prompt-and-pray/internal/api"
)

// issueCloseBody — фикстурный ответ POST /commands (FieldsCommandIssues):
// две задачи, переведённые в resolved.
const issueCloseBody = `{
	"$type":"CommandList",
	"issues":[
		{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"Fix login flow","resolved":1783296000000,"project":{"$type":"Project","id":"0-0","shortName":"PRJ"}},
		{"$type":"Issue","id":"2-2","idReadable":"PRJ-43","summary":"Write TZ for yt CLI","resolved":1783296000000,"project":{"$type":"Project","id":"0-0","shortName":"PRJ"}}
	]
}`

// issueCloseServer поднимает фейковый YouTrack: POST /commands, фиксируя
// запросы и тела.
func issueCloseServer(t *testing.T, body string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	reqs := &[]string{}
	bodies := &[]string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*reqs = append(*reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		*bodies = append(*bodies, string(data))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/commands" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, reqs, bodies
}

// runIssueClose выполняет yt issue close <args> против фейкового сервера
// (с -y, чтобы не ждать подтверждения).
func runIssueClose(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"issue", "close", "-y"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

// closeQuery разбирает query-строку из захваченного запроса.
func closeQuery(t *testing.T, req string) url.Values {
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

func TestNewIssueCloseCmd_Flags(t *testing.T) {
	cmd := newIssueCloseCmd()
	if cmd.Use != "close <id>..." {
		t.Errorf("Use = %q, want close <id>...", cmd.Use)
	}
	checks := map[string]string{"state": "s", "message": "m", "yes": "y"}
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
	if f := cmd.Flags().Lookup("state"); f.DefValue != closeDefaultState {
		t.Errorf("--state default = %q, want %q", f.DefValue, closeDefaultState)
	}
}

func TestIssueClose_NoIDs(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "close")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestIssueClose_InvalidID(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "issue", "close", "-y", "bogus")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: cannot parse issue id: bogus\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueClose_TTY(t *testing.T) {
	srv, reqs, bodies := issueCloseServer(t, issueCloseBody)
	out, _, code := runIssueClose(t, srv, "PRJ-42", "PRJ-43")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "✓ PRJ-42 → Fixed\n✓ PRJ-43 → Fixed\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1 (POST /commands)", *reqs)
	}
	req := (*reqs)[0]
	if !strings.HasPrefix(req, "POST /commands?") {
		t.Errorf("request = %q, want POST /commands", req)
	}
	if q := closeQuery(t, req); q.Get("fields") != api.FieldsCommandIssues {
		t.Errorf("fields = %q, want %q", q.Get("fields"), api.FieldsCommandIssues)
	}

	body := decodeSentBody(t, bodies)
	if body["query"] != "state: Fixed" {
		t.Errorf("query = %v, want state: Fixed", body["query"])
	}
	if _, ok := body["comment"]; ok {
		t.Errorf("comment present with empty value: %v", body)
	}
	issues, ok := body["issues"].([]any)
	if !ok || len(issues) != 2 {
		t.Fatalf("issues = %v, want 2 refs", body["issues"])
	}
	first, ok := issues[0].(map[string]any)
	if !ok || first["idReadable"] != "PRJ-42" {
		t.Errorf("issue[0] = %v, want {idReadable: PRJ-42}", issues[0])
	}
}

func TestIssueClose_RingID(t *testing.T) {
	srv, _, bodies := issueCloseServer(t, issueCloseBody)
	_, _, code := runIssueClose(t, srv, "2-1")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	body := decodeSentBody(t, bodies)
	issues, ok := body["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("issues = %v, want 1 ref", body["issues"])
	}
	ref, ok := issues[0].(map[string]any)
	if !ok || ref["id"] != "2-1" {
		t.Errorf("issue[0] = %v, want {id: 2-1}", issues[0])
	}
	if _, hasReadable := ref["idReadable"]; hasReadable {
		t.Errorf("issue[0] has idReadable, want only id: %v", ref)
	}
}

func TestIssueClose_StateAndMessage(t *testing.T) {
	srv, _, bodies := issueCloseServer(t, issueCloseBody)
	_, _, code := runIssueClose(t, srv, "-s", "Verified", "-m", "Resolved by yt", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	body := decodeSentBody(t, bodies)
	if body["query"] != "state: Verified" {
		t.Errorf("query = %v, want state: Verified", body["query"])
	}
	if body["comment"] != "Resolved by yt" {
		t.Errorf("comment = %v, want Resolved by yt", body["comment"])
	}
}

func TestIssueClose_JSON(t *testing.T) {
	srv, _, _ := issueCloseServer(t, issueCloseBody)
	out, _, code := runIssueClose(t, srv, "--json", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 || got[0]["idReadable"] != "PRJ-42" || got[1]["idReadable"] != "PRJ-43" {
		t.Errorf("json = %v, want [PRJ-42, PRJ-43]", got)
	}
}

func TestIssueClose_Error400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Bad request","error_description":"You are not allowed to apply command"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runIssueClose(t, ts, "PRJ-42")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 400 Bad request: You are not allowed to apply command\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestIssueClose_ConfirmYes(t *testing.T) {
	srv, _, _ := issueCloseServer(t, issueCloseBody)
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader("y\n"),
		"issue", "close", "PRJ-42", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if !strings.Contains(stderr, `! This will close 1 issue(s) via command "state: Fixed". Continue? [y/N] `) {
		t.Errorf("stderr = %q, want confirmation prompt", stderr)
	}
}

func TestIssueClose_ConfirmNo(t *testing.T) {
	srv, reqs, _ := issueCloseServer(t, issueCloseBody)
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader("n\n"),
		"issue", "close", "PRJ-42", "--base-url", srv.URL)
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

func TestParseIssueRef(t *testing.T) {
	cases := []struct {
		name         string
		value        string
		wantID       string
		wantReadable string
		wantErr      bool
	}{
		{"ring id", "2-1", "2-1", "", false},
		{"ring id zeros", "0-0", "0-0", "", false},
		{"idReadable", "PRJ-42", "", "PRJ-42", false},
		{"idReadable lowercase", "prj-42", "", "prj-42", false},
		{"idReadable digits in code", "B2B-3", "", "B2B-3", false},
		{"no separator", "PRJ42", "", "", true},
		{"no number", "PRJ", "", "", true},
		{"letters only", "abc", "", "", true},
		{"starts with dash", "-42", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := parseIssueRef(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseIssueRef(%q) = %+v, want error", tc.value, ref)
				}
				if !strings.Contains(err.Error(), "cannot parse issue id: "+tc.value) {
					t.Errorf("error = %q, want contains cannot parse issue id", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIssueRef(%q) error: %v", tc.value, err)
			}
			if ref.ID != tc.wantID || ref.IDReadable != tc.wantReadable {
				t.Errorf("parseIssueRef(%q) = %+v, want id=%q idReadable=%q", tc.value, ref, tc.wantID, tc.wantReadable)
			}
		})
	}
}
