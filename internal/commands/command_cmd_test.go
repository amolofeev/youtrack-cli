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

// commandBody — фикстурный ответ POST /commands (FieldsCommandIssues): две
// задачи, к которым применена команда.
const commandBody = `{
	"$type":"CommandList",
	"issues":[
		{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"Fix login flow","resolved":1783296000000,"project":{"$type":"Project","id":"0-0","shortName":"PRJ"}},
		{"$type":"Issue","id":"2-2","idReadable":"PRJ-43","summary":"Write TZ for yt CLI","resolved":1783296000000,"project":{"$type":"Project","id":"0-0","shortName":"PRJ"}}
	]
}`

// commandServer поднимает фейковый YouTrack: POST /commands, фиксируя запросы
// и тела.
func commandServer(t *testing.T, body string) (*httptest.Server, *[]string, *[]string) {
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

// runCommand выполняет yt command <args> против фейкового сервера (с -y, чтобы
// не ждать подтверждения).
func runCommand(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"command", "-y"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

// commandQuery разбирает query-строку из захваченного запроса.
func commandQuery(t *testing.T, req string) url.Values {
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

func TestNewCommandCmd_Flags(t *testing.T) {
	cmd := newCommandCmd()
	if cmd.Use != "command <commands> <id>..." {
		t.Errorf("Use = %q, want command <commands> <id>...", cmd.Use)
	}
	checks := map[string]string{"message": "m", "yes": "y", "run-as": ""}
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

func TestCommand_NoArgs(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "command")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestCommand_OnlyQuery(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "command", "state: Fixed")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestCommand_InvalidID(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "command", "state: Fixed", "bogus")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
	if want := "yt: cannot parse issue id: bogus\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestCommand_TTY(t *testing.T) {
	srv, reqs, bodies := commandServer(t, commandBody)
	out, _, code := runCommand(t, srv, "state: Fixed Priority: High", "PRJ-42", "PRJ-43")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "✓ PRJ-42: state → Fixed, Priority → High\n✓ PRJ-43: state → Fixed, Priority → High\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1 (POST /commands)", *reqs)
	}
	req := (*reqs)[0]
	if !strings.HasPrefix(req, "POST /commands?") {
		t.Errorf("request = %q, want POST /commands", req)
	}
	if q := commandQuery(t, req); q.Get("fields") != api.FieldsCommandIssues {
		t.Errorf("fields = %q, want %q", q.Get("fields"), api.FieldsCommandIssues)
	}

	body := decodeSentBody(t, bodies)
	if body["query"] != "state: Fixed Priority: High" {
		t.Errorf("query = %v, want state: Fixed Priority: High", body["query"])
	}
	if _, ok := body["comment"]; ok {
		t.Errorf("comment present with empty value: %v", body)
	}
	if _, ok := body["runAs"]; ok {
		t.Errorf("runAs present with empty value: %v", body)
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

func TestCommand_RingID(t *testing.T) {
	srv, _, bodies := commandServer(t, commandBody)
	_, _, code := runCommand(t, srv, "state: Fixed", "2-1")
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

func TestCommand_MessageAndRunAs(t *testing.T) {
	srv, _, bodies := commandServer(t, commandBody)
	_, _, code := runCommand(t, srv, "-m", "Triaged", "--run-as", "alex", "state: Fixed", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d", code, exitOK)
	}
	body := decodeSentBody(t, bodies)
	if body["comment"] != "Triaged" {
		t.Errorf("comment = %v, want Triaged", body["comment"])
	}
	if body["runAs"] != "alex" {
		t.Errorf("runAs = %v, want alex", body["runAs"])
	}
}

func TestCommand_JSON(t *testing.T) {
	srv, _, _ := commandServer(t, commandBody)
	out, _, code := runCommand(t, srv, "--json", "state: Fixed", "PRJ-42")
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

func TestCommand_Error400(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Bad request","error_description":"State expected: Fixed"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runCommand(t, ts, "state: Fixed", "PRJ-42")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 400 Bad request: State expected: Fixed\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestCommand_ConfirmYes(t *testing.T) {
	srv, _, _ := commandServer(t, commandBody)
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader("y\n"),
		"command", "state: Fixed", "PRJ-42", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if !strings.Contains(stderr, `! This will apply command "state: Fixed" to 1 issue(s). Continue? [y/N] `) {
		t.Errorf("stderr = %q, want confirmation prompt", stderr)
	}
}

func TestCommand_ConfirmNo(t *testing.T) {
	srv, reqs, _ := commandServer(t, commandBody)
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	_, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader("n\n"),
		"command", "state: Fixed", "PRJ-42", "--base-url", srv.URL)
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

func TestFormatCommandQuery(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"state: Fixed Priority: High", "state → Fixed, Priority → High"},
		{"state: Fixed", "state → Fixed"},
		{"state:", "state"},
		{"tag: urgent, critical", "tag → urgent, critical"},
		{"responsible: alex b", "responsible → alex b"},
		{"Fixed", "Fixed"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := formatCommandQuery(tc.query); got != tc.want {
			t.Errorf("formatCommandQuery(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

// commandAssistBody — фикстурный ответ POST /commands/assist (поля
// FieldsCommandAssist): три подсказки для частичной команды «state: ».
const commandAssistBody = `{"$type":"CommandList","query":"state: ","suggestions":[
	{"$type":"Suggestion","option":"state","description":"Set the state","suffix":": "},
	{"$type":"Suggestion","option":"Fixed","description":"Fixed state","prefix":" ","suffix":" "},
	{"$type":"Suggestion","option":"Done","description":"Done state","prefix":" ","suffix":" "}
]}`

// commandAssistServer поднимает фейковый YouTrack: POST /commands/assist,
// фиксируя запросы и тела.
func commandAssistServer(t *testing.T, body string) (*httptest.Server, *[]string, *[]string) {
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

// runCommandAssist выполняет yt command assist <args> против фейкового сервера.
func runCommandAssist(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	full := append([]string{"command", "assist"}, args...)
	full = append(full, "--base-url", srv.URL)
	return runRoot(t, NewRootCommand(), full...)
}

func TestNewCommandAssistCmd_Use(t *testing.T) {
	cmd := newCommandAssistCmd()
	if cmd.Use != "assist <commands>" {
		t.Errorf("Use = %q, want assist <commands>", cmd.Use)
	}
}

func TestCommandAssistRegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	command, _, err := root.Find([]string{"command"})
	if err != nil {
		t.Fatalf("find command: %v", err)
	}
	assist, _, err := command.Find([]string{"assist"})
	if err != nil {
		t.Fatalf("find command assist: %v", err)
	}
	if assist.Name() != "assist" {
		t.Errorf("assist.Name() = %q, want assist", assist.Name())
	}
}

func TestCommandAssist_NoArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "command", "assist")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestCommandAssist_TooManyArgs_UsageError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, NewRootCommand(), "command", "assist", "state: ", "tag: ")
	if code != exitUsage {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitUsage, stderr)
	}
}

func TestCommandAssist_TTY(t *testing.T) {
	srv, reqs, bodies := commandAssistServer(t, commandAssistBody)
	out, _, code := runCommandAssist(t, srv, "state: ")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	want := "OK: state — Set the state\n" +
		"OK: Fixed — Fixed state\n" +
		"OK: Done — Done state\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}

	if len(*reqs) != 1 {
		t.Fatalf("requests = %v, want 1", *reqs)
	}
	got := (*reqs)[0]
	if !strings.HasPrefix(got, "POST /commands/assist?") {
		t.Errorf("request = %q, want POST /commands/assist", got)
	}
	if q := commandQuery(t, got); q.Get("fields") != api.FieldsCommandAssist {
		t.Errorf("fields = %q, want %q", q.Get("fields"), api.FieldsCommandAssist)
	}

	if len(*bodies) != 1 {
		t.Fatalf("bodies = %v, want 1", *bodies)
	}
	body := decodeSentBody(t, bodies)
	if body["query"] != "state: " {
		t.Errorf("body query = %v, want state: ", body["query"])
	}
	if body["caret"] != float64(len("state: ")) {
		t.Errorf("body caret = %v, want %d", body["caret"], len("state: "))
	}
}

func TestCommandAssist_TTY_Empty(t *testing.T) {
	srv, _, _ := commandAssistServer(t, `{"$type":"CommandList","query":"state: Fixed"}`)
	out, _, code := runCommandAssist(t, srv, "state: Fixed")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	if want := "No suggestions for \"state: Fixed\"\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestCommandAssist_JSON(t *testing.T) {
	srv, _, _ := commandAssistServer(t, commandAssistBody)
	out, _, code := runCommandAssist(t, srv, "state: ", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if got["query"] != "state: " {
		t.Errorf("query = %v, want state: ", got["query"])
	}
	sugs, ok := got["suggestions"].([]any)
	if !ok || len(sugs) != 3 {
		t.Fatalf("suggestions = %v, want array of 3", got["suggestions"])
	}
	first := sugs[0].(map[string]any)
	if first["option"] != "state" || first["description"] != "Set the state" {
		t.Errorf("suggestions[0] = %v, want state / Set the state", first)
	}
}

func TestCommandAssist_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Bad request","error_description":"cannot parse command"}`))
	}))
	t.Cleanup(ts.Close)

	_, stderr, code := runCommandAssist(t, ts, "bogus command")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr)
	}
	if want := "yt: request failed: 400 Bad request: cannot parse command\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}
