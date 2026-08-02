package commands

import (
	"flag"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Golden-тесты вывода (SPEC §5.2): фикстурный ответ фейкового сервера →
// ожидаемый TTY/JSON-вывод команды, зафиксированный в файле
// ../../testdata/<name>.golden. Флаг -update осознанно перезаписывает
// golden-файлы текущим выводом (при изменении формата).
var update = flag.Bool("update", false, "update golden files in ../../testdata/")

// goldenPath возвращает путь к golden-файлу относительно каталога пакета
// (тесты запускаются из internal/commands/, поэтому testdata — на корне yt/,
// как в SPEC §2.2).
func goldenPath(name string) string {
	return filepath.Join("..", "..", "testdata", name+".golden")
}

// assertGolden сравнивает фактический вывод с golden-файлом; с -update
// перезаписывает файл фактическим выводом.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (запусти go test ./internal/commands -run %s -update)",
			path, err, t.Name())
	}
	if got != string(want) {
		t.Errorf("%s: вывод не совпадает\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// runGolden выполняет команду против фейкового сервера с изолированной
// конфигурацией и тестовым токеном. При srv == nil --base-url не передаётся
// (команды без API, например version).
func runGolden(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	if srv != nil {
		args = append(args, "--base-url", srv.URL)
	}
	return runRoot(t, NewRootCommand(), args...)
}

// normalizeBaseURL заменяет адрес httptest-сервера плейсхолдером: адрес
// уникален для каждого запуска, golden-файл должен оставаться стабильным.
func normalizeBaseURL(s, baseURL string) string {
	return strings.ReplaceAll(s, baseURL, "<base-url>")
}

// normalizeRuntime заменяет go/os/arch плейсхолдерами: значения зависят от
// платформы запуска, golden-файл должен оставаться переносимым.
func normalizeRuntime(s string) string {
	return strings.NewReplacer(
		runtime.Version(), "<goversion>",
		runtime.GOOS, "<os>",
		runtime.GOARCH, "<arch>",
	).Replace(s)
}

// --- issue list (SPEC §3.4) ---

func TestGoldenIssueListTTY(t *testing.T) {
	srv, _ := searchServer(t, searchIssuesBody)
	out, _, code := runGolden(t, srv, "issue", "list")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_list_tty", out)
}

func TestGoldenIssueListTTYEmpty(t *testing.T) {
	srv, _ := searchServer(t, `[]`)
	out, _, code := runGolden(t, srv, "issue", "list")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_list_tty_empty", out)
}

func TestGoldenIssueListJSON(t *testing.T) {
	srv, _ := searchServer(t, searchIssuesBody)
	out, _, code := runGolden(t, srv, "issue", "list", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_list_json", out)
}

func TestGoldenIssueListJSONEmpty(t *testing.T) {
	srv, _ := searchServer(t, `[]`)
	out, _, code := runGolden(t, srv, "issue", "list", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_list_json_empty", out)
}

// --- search (SPEC §3.5) ---

func TestGoldenSearchTTY(t *testing.T) {
	srv, _ := searchServer(t, searchIssuesBody)
	out, _, code := runGolden(t, srv, "search", "project: PRJ has: open")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "search_tty", out)
}

func TestGoldenSearchTTYEmpty(t *testing.T) {
	srv, _ := searchServer(t, `[]`)
	out, _, code := runGolden(t, srv, "search", "nonexistent")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "search_tty_empty", out)
}

func TestGoldenSearchJSON(t *testing.T) {
	srv, _ := searchServer(t, searchIssuesBody)
	out, _, code := runGolden(t, srv, "search", "project: PRJ", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "search_json", out)
}

// --- search suggest (SPEC §3.5) ---

func TestGoldenSearchSuggestTTY(t *testing.T) {
	srv, _, _ := suggestServer(t, searchSuggestBody)
	out, _, code := runGolden(t, srv, "search", "suggest", "has: ")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "search_suggest_tty", out)
}

func TestGoldenSearchSuggestJSON(t *testing.T) {
	srv, _, _ := suggestServer(t, searchSuggestBody)
	out, _, code := runGolden(t, srv, "search", "suggest", "has: ", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "search_suggest_json", out)
}

// --- issue view (SPEC §3.4) ---

func TestGoldenIssueViewTTY(t *testing.T) {
	srv, _ := issueViewServer(t, issueViewIssueBody, issueViewCommentsBody)
	out, _, code := runGolden(t, srv, "issue", "view", "PRJ-42", "-c")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_view_tty", out)
}

func TestGoldenIssueViewTTYMinimal(t *testing.T) {
	const minimal = `{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"","description":"","created":0,"updated":0,"project":null,"reporter":null,"customFields":null,"tags":null}`
	srv, _ := issueViewServer(t, minimal, `[]`)
	out, _, code := runGolden(t, srv, "issue", "view", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_view_tty_minimal", out)
}

// issueViewLongBody — задача с длинным многострочным описанием (занимает не
// один экран): pager в тестах отключён через PAGER=cat (SPEC §5.2).
const issueViewLongBody = `{
	"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"Fix login flow",
	"description":"A long description spanning many lines so the output does not fit a single terminal screen and a real pager would take over.\n\nParagraph one keeps going with a lot of words to push the content well past one screen of output.\n\nParagraph two continues the story and adds even more detail to the issue body.\n\nFinal paragraph wraps up the description.",
	"created":1782914400000,"updated":1783245600000,
	"project":{"$type":"Project","id":"0-0","shortName":"PRJ","name":"Demo"},
	"reporter":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"},
	"customFields":[{"$type":"EnumIssueCustomField","id":"4-1","name":"State","value":{"$type":"StateBundleElement","id":"5-1","name":"Open"}}],
	"tags":[{"$type":"Tag","id":"6-1","name":"backend"}]
}`

func TestGoldenIssueViewTTYLongDescription(t *testing.T) {
	t.Setenv("PAGER", "cat")
	srv, _ := issueViewServer(t, issueViewLongBody, `[]`)
	out, _, code := runGolden(t, srv, "issue", "view", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_view_tty_longdesc", out)
}

func TestGoldenIssueViewJSON(t *testing.T) {
	srv, _ := issueViewServer(t, issueViewIssueBody, `[]`)
	out, _, code := runGolden(t, srv, "issue", "view", "PRJ-42", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_view_json", out)
}

func TestGoldenIssueViewJSONComments(t *testing.T) {
	srv, _ := issueViewServer(t, issueViewIssueBody, issueViewCommentsBody)
	out, _, code := runGolden(t, srv, "issue", "view", "PRJ-42", "--json", "-c")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_view_json_comments", out)
}

// --- issue create (SPEC §3.4) ---

func TestGoldenIssueCreateTTY(t *testing.T) {
	srv, _, _ := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	out, _, code := runGolden(t, srv, "issue", "create", "-p", "PRJ", "-t", "Fix login flow", "-b", "Steps:\n1. Reproduce.")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_create_tty", out)
}

func TestGoldenIssueCreateJSON(t *testing.T) {
	srv, _, _ := issueCreateServer(t, issueCreateProjectsBody, issueCreateBody)
	out, _, code := runGolden(t, srv, "issue", "create", "-p", "PRJ", "-t", "Fix login flow", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_create_json", out)
}

// --- issue edit (SPEC §3.4) ---

func TestGoldenIssueEditTTY(t *testing.T) {
	srv, _, _ := issueEditServer(t, issueEditBody)
	out, _, code := runGolden(t, srv, "issue", "edit", "PRJ-42", "--title", "New summary", "--body", "New description")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_edit_tty", out)
}

func TestGoldenIssueEditJSON(t *testing.T) {
	srv, _, _ := issueEditServer(t, issueEditBody)
	out, _, code := runGolden(t, srv, "issue", "edit", "PRJ-42", "--title", "New summary", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_edit_json", out)
}

// --- issue close (SPEC §3.4) ---

func TestGoldenIssueCloseTTY(t *testing.T) {
	srv, _, _ := issueCloseServer(t, issueCloseBody)
	out, _, code := runGolden(t, srv, "issue", "close", "-y", "PRJ-42", "PRJ-43")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_close_tty", out)
}

func TestGoldenIssueCloseJSON(t *testing.T) {
	srv, _, _ := issueCloseServer(t, issueCloseBody)
	out, _, code := runGolden(t, srv, "issue", "close", "-y", "--json", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_close_json", out)
}

// --- issue delete (SPEC §3.4) ---

func TestGoldenIssueDeleteTTY(t *testing.T) {
	srv, _ := issueDeleteServer(t)
	out, _, code := runGolden(t, srv, "issue", "delete", "-y", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_delete_tty", out)
}

// --- issue comment list/create (SPEC §3.4) ---

func TestGoldenIssueCommentListTTY(t *testing.T) {
	srv, _, _ := commentServer(t, issueViewCommentsBody)
	out, _, code := runGolden(t, srv, "issue", "comment", "list", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_comment_list_tty", out)
}

func TestGoldenIssueCommentListTTYEmpty(t *testing.T) {
	srv, _, _ := commentServer(t, `[]`)
	out, _, code := runGolden(t, srv, "issue", "comment", "list", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_comment_list_tty_empty", out)
}

func TestGoldenIssueCommentListJSON(t *testing.T) {
	srv, _, _ := commentServer(t, issueViewCommentsBody)
	out, _, code := runGolden(t, srv, "issue", "comment", "list", "PRJ-42", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_comment_list_json", out)
}

func TestGoldenIssueCommentCreateTTY(t *testing.T) {
	srv, _, _ := commentServer(t, commentCreateBody)
	out, _, code := runGolden(t, srv, "issue", "comment", "create", "PRJ-42", "-m", "Fix login flow")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_comment_create_tty", out)
}

func TestGoldenIssueCommentCreateJSON(t *testing.T) {
	srv, _, _ := commentServer(t, commentCreateBody)
	out, _, code := runGolden(t, srv, "issue", "comment", "create", "PRJ-42", "-m", "Fix login flow", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "issue_comment_create_json", out)
}

// --- command / command assist (SPEC §3.6) ---

func TestGoldenCommandTTY(t *testing.T) {
	srv, _, _ := commandServer(t, commandBody)
	out, _, code := runGolden(t, srv, "command", "-y", "state: Fixed Priority: High", "PRJ-42", "PRJ-43")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "command_tty", out)
}

func TestGoldenCommandJSON(t *testing.T) {
	srv, _, _ := commandServer(t, commandBody)
	out, _, code := runGolden(t, srv, "command", "-y", "--json", "state: Fixed", "PRJ-42")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "command_json", out)
}

func TestGoldenCommandAssistTTY(t *testing.T) {
	srv, _, _ := commandAssistServer(t, commandAssistBody)
	out, _, code := runGolden(t, srv, "command", "assist", "state: ")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "command_assist_tty", out)
}

func TestGoldenCommandAssistJSON(t *testing.T) {
	srv, _, _ := commandAssistServer(t, commandAssistBody)
	out, _, code := runGolden(t, srv, "command", "assist", "state: ", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "command_assist_json", out)
}

// --- project list (SPEC §3.7) ---

func TestGoldenProjectListTTY(t *testing.T) {
	srv, _ := projectServer(t, projectListBody)
	out, _, code := runGolden(t, srv, "project", "list")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "project_list_tty", out)
}

func TestGoldenProjectListTTYEmpty(t *testing.T) {
	srv, _ := projectServer(t, `[]`)
	out, _, code := runGolden(t, srv, "project", "list")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "project_list_tty_empty", out)
}

func TestGoldenProjectListJSON(t *testing.T) {
	srv, _ := projectServer(t, projectListBody)
	out, _, code := runGolden(t, srv, "project", "list", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "project_list_json", out)
}

// --- tag list (SPEC §3.9) ---

func TestGoldenTagListTTY(t *testing.T) {
	srv, _ := tagServer(t, tagListBody)
	out, _, code := runGolden(t, srv, "tag", "list")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "tag_list_tty", out)
}

func TestGoldenTagListTTYEmpty(t *testing.T) {
	srv, _ := tagServer(t, `[]`)
	out, _, code := runGolden(t, srv, "tag", "list")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "tag_list_tty_empty", out)
}

func TestGoldenTagListJSON(t *testing.T) {
	srv, _ := tagServer(t, tagListBody)
	out, _, code := runGolden(t, srv, "tag", "list", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "tag_list_json", out)
}

// --- auth status / user whoami (SPEC §3.3, §3.8) ---

func TestGoldenAuthStatusTTY(t *testing.T) {
	srv, _ := userServer(t, "")
	out, _, code := runGolden(t, srv, "auth", "status")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "auth_status_tty", normalizeBaseURL(out, srv.URL))
}

func TestGoldenAuthStatusJSON(t *testing.T) {
	srv, _ := userServer(t, "")
	out, _, code := runGolden(t, srv, "auth", "status", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "auth_status_json", normalizeBaseURL(out, srv.URL))
}

func TestGoldenUserWhoamiTTY(t *testing.T) {
	srv, _ := userServer(t, "")
	out, _, code := runGolden(t, srv, "user", "whoami")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "user_whoami_tty", out)
}

func TestGoldenUserWhoamiJSON(t *testing.T) {
	srv, _ := userServer(t, "")
	out, _, code := runGolden(t, srv, "user", "whoami", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "user_whoami_json", out)
}

// --- version (SPEC §3.11) ---

func TestGoldenVersionTTY(t *testing.T) {
	setVersionForTest(t, "0.0.1-pre-alpha", "2036315", "2026-07-31T12:00:00Z")
	out, _, code := runGolden(t, nil, "version")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "version_tty", normalizeRuntime(out))
}

func TestGoldenVersionJSON(t *testing.T) {
	setVersionForTest(t, "0.0.1-pre-alpha", "2036315", "2026-07-31T12:00:00Z")
	out, _, code := runGolden(t, nil, "version", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; out=%q", code, exitOK, out)
	}
	assertGolden(t, "version_json", normalizeRuntime(out))
}
