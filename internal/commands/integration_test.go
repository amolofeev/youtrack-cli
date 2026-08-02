package commands

// Интеграционные тесты против живого сервера YouTrack (SPEC §5.4).
//
// Запуск: из каталога yt/ — `YT_INTEGRATION=1 make integration`
// (или `YT_INTEGRATION=1 go test ./...`). В CI не запускаются
// (.github/workflows/ci.yml гоняет `make test` без YT_INTEGRATION).
//
// Read-only тесты (auth status, whoami, list/view, search, suggest, assist)
// требуют только YT_INTEGRATION=1. Тесты, создающие/удаляющие реальные данные,
// дополнительно требуют YT_INTEGRATION_MUTATE=1 — это явное разрешение на
// create/delete (SPEC §5.4: такие тесты по умолчанию t.Skip). Каждая
// мутирующая проверка создаёт смоук-ишью с уникальным summary и удаляет её
// в t.Cleanup.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amolofeev/yt/internal/api"
	"github.com/amolofeev/yt/internal/config"
)

const (
	// envIntegration включает интеграционные тесты (SPEC §5.4).
	envIntegration = "YT_INTEGRATION"
	// envIntegrationMutate — явное разрешение на создание/удаление реальных
	// данных: без него тесты на create/delete пропускаются.
	envIntegrationMutate = "YT_INTEGRATION_MUTATE"
)

// integrationConfig возвращает адрес и токен живого сервера; без
// YT_INTEGRATION=1 или без токена тест пропускается. Base URL по умолчанию —
// дефолт конфигурации (localhost:8080/api).
func integrationConfig(t *testing.T) (baseURL, token string) {
	t.Helper()
	if os.Getenv(envIntegration) != "1" {
		t.Skipf("integration tests: set %s=1 to run against a live YouTrack", envIntegration)
	}
	baseURL = os.Getenv("YT_BASE_URL")
	if baseURL == "" {
		baseURL = config.DefaultBaseURL
	}
	token = os.Getenv("YT_TOKEN")
	if token == "" {
		t.Skip("integration tests: YT_TOKEN is required")
	}
	return baseURL, token
}

// requireMutate пропускает тест без явного разрешения на create/delete
// (SPEC §5.4). Вызывается в начале каждого мутирующего теста.
func requireMutate(t *testing.T) {
	t.Helper()
	if os.Getenv(envIntegrationMutate) != "1" {
		t.Skipf("mutating integration tests: set %s=1 to create/delete real data", envIntegrationMutate)
	}
}

// runIntegration выполняет команду против живого сервера с изолированной
// конфигурацией (baseURL/token из окружения, YT_CONFIG_HOME — временный).
func runIntegration(t *testing.T, baseURL, token string, args ...string) (string, string, int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("YT_BASE_URL", baseURL)
	t.Setenv("YT_TOKEN", token)
	return runRoot(t, NewRootCommand(), args...)
}

// requireExit падает, если код выхода команды не равен want.
func requireExit(t *testing.T, code, want int, out, stderr string) {
	t.Helper()
	if code != want {
		t.Fatalf("exit code = %d, want %d\nstdout=%q\nstderr=%q", code, want, out, stderr)
	}
}

// liveProject — не-архивированный проект сервера, используемый смоук-тестами
// как мишень create/command.
type liveProject struct {
	id        string
	shortName string
	name      string
}

// findLiveProject возвращает первый не-архивированный проект с shortName;
// если таких нет — тест пропускается.
func findLiveProject(t *testing.T, baseURL, token string) liveProject {
	t.Helper()
	out, stderr, code := runIntegration(t, baseURL, token, "project", "list", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var projects []api.Project
	if err := json.Unmarshal([]byte(out), &projects); err != nil {
		t.Fatalf("project list: stdout is not valid JSON: %v\n%s", err, out)
	}
	for _, p := range projects {
		if p.Archived != nil && *p.Archived {
			continue
		}
		if p.ShortName != "" {
			return liveProject{id: p.ID, shortName: p.ShortName, name: p.Name}
		}
	}
	t.Skip("no non-archived project with shortName on server")
	return liveProject{}
}

// createSmokeIssue создаёт задачу с уникальным summary (вызывается только из
// мутирующих тестов, уже прошедших requireMutate) и регистрирует её удаление
// в t.Cleanup. Возвращает idReadable.
func createSmokeIssue(t *testing.T, baseURL, token, project, tag string) string {
	t.Helper()
	summary := fmt.Sprintf("yt integration %s %s", tag, time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	out, stderr, code := runIntegration(t, baseURL, token, "issue", "create", "-p", project, "-t", summary, "--json")
	requireExit(t, code, exitOK, out, stderr)
	var it api.Issue
	if err := json.Unmarshal([]byte(out), &it); err != nil {
		t.Fatalf("issue create: stdout is not valid JSON: %v\n%s", err, out)
	}
	if it.IDReadable == "" {
		t.Fatalf("issue create: empty idReadable; stdout=%s", out)
	}
	id := it.IDReadable
	t.Cleanup(func() {
		out, stderr, code := runIntegration(t, baseURL, token, "issue", "delete", id, "-y")
		if code != exitOK {
			t.Logf("cleanup delete %s: exit=%d stdout=%q stderr=%q", id, code, out, stderr)
		}
	})
	return id
}

// existingIssueID возвращает idReadable произвольной существующей задачи для
// read-only тестов view/comment; если на сервере нет задач — тест пропускается.
func existingIssueID(t *testing.T, baseURL, token string) string {
	t.Helper()
	out, stderr, code := runIntegration(t, baseURL, token, "issue", "list", "-l", "1", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var issues []api.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("issue list: stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(issues) == 0 {
		t.Skip("no issues on server")
	}
	return issues[0].IDReadable
}

// resolveState определяет состояние разрешения (resolved) воркфлоу проекта по
// последней подсказке "state: " из command assist (на сервере 2025.3 — Done).
func resolveState(t *testing.T, baseURL, token string) string {
	t.Helper()
	out, stderr, code := runIntegration(t, baseURL, token, "command", "assist", "state: ", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var cl api.CommandList
	if err := json.Unmarshal([]byte(out), &cl); err != nil {
		t.Fatalf("command assist: stdout is not valid JSON: %v\n%s", err, out)
	}
	var states []string
	for _, s := range cl.Suggestions {
		if s.Description == "State" {
			states = append(states, s.Option)
		}
	}
	if len(states) == 0 {
		t.Skip("no state suggestions from command assist")
	}
	return states[len(states)-1]
}

// --- Read-only (только YT_INTEGRATION=1) ---

func TestIntegrationAuthStatus(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "auth", "status", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var st struct {
		BaseURL string `json:"baseUrl"`
		Login   string `json:"login"`
		Guest   bool   `json:"guest"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("auth status: stdout is not valid JSON: %v\n%s", err, out)
	}
	if st.Login == "" {
		t.Errorf("auth status: empty login; stdout=%s", out)
	}
	if st.BaseURL != baseURL {
		t.Errorf("auth status: baseUrl = %q, want %q", st.BaseURL, baseURL)
	}
}

func TestIntegrationWhoami(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "user", "whoami", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var u api.User
	if err := json.Unmarshal([]byte(out), &u); err != nil {
		t.Fatalf("user whoami: stdout is not valid JSON: %v\n%s", err, out)
	}
	if u.Login == "" {
		t.Errorf("user whoami: empty login; stdout=%s", out)
	}
}

func TestIntegrationIssueList(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "issue", "list", "-l", "5", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var issues []api.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("issue list: stdout is not valid JSON: %v\n%s", err, out)
	}
	for _, it := range issues {
		if it.IDReadable == "" {
			t.Errorf("issue list: issue without idReadable: %+v", it)
		}
	}
}

func TestIntegrationIssueView(t *testing.T) {
	baseURL, token := integrationConfig(t)
	id := existingIssueID(t, baseURL, token)
	out, stderr, code := runIntegration(t, baseURL, token, "issue", "view", id, "--json")
	requireExit(t, code, exitOK, out, stderr)
	var it api.Issue
	if err := json.Unmarshal([]byte(out), &it); err != nil {
		t.Fatalf("issue view: stdout is not valid JSON: %v\n%s", err, out)
	}
	if it.IDReadable != id {
		t.Errorf("issue view: idReadable = %q, want %q", it.IDReadable, id)
	}
}

func TestIntegrationCommentList(t *testing.T) {
	baseURL, token := integrationConfig(t)
	id := existingIssueID(t, baseURL, token)
	out, stderr, code := runIntegration(t, baseURL, token, "issue", "comment", "list", id, "--json")
	requireExit(t, code, exitOK, out, stderr)
	var comments []api.IssueComment
	if err := json.Unmarshal([]byte(out), &comments); err != nil {
		t.Fatalf("issue comment list: stdout is not valid JSON: %v\n%s", err, out)
	}
}

func TestIntegrationSearch(t *testing.T) {
	baseURL, token := integrationConfig(t)
	proj := findLiveProject(t, baseURL, token)
	out, stderr, code := runIntegration(t, baseURL, token, "search", "project: "+proj.shortName, "-l", "5", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var issues []api.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("search: stdout is not valid JSON: %v\n%s", err, out)
	}
	for _, it := range issues {
		if it.Project == nil || !strings.EqualFold(it.Project.ShortName, proj.shortName) {
			t.Errorf("search: issue %s in project %+v, want shortName %s", it.IDReadable, it.Project, proj.shortName)
		}
	}
}

func TestIntegrationSearchSuggest(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "search", "suggest", "has: ")
	requireExit(t, code, exitOK, out, stderr)
	if stderr != "" {
		t.Errorf("search suggest: stderr = %q, want empty", stderr)
	}
}

func TestIntegrationCommandAssist(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "command", "assist", "state: ")
	requireExit(t, code, exitOK, out, stderr)
	if stderr != "" {
		t.Errorf("command assist: stderr = %q, want empty", stderr)
	}
}

func TestIntegrationProjectList(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "project", "list", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var projects []api.Project
	if err := json.Unmarshal([]byte(out), &projects); err != nil {
		t.Fatalf("project list: stdout is not valid JSON: %v\n%s", err, out)
	}
	for _, p := range projects {
		if p.ShortName == "" || p.Name == "" {
			t.Errorf("project list: project without shortName/name: %+v", p)
		}
	}
}

func TestIntegrationTagList(t *testing.T) {
	baseURL, token := integrationConfig(t)
	out, stderr, code := runIntegration(t, baseURL, token, "tag", "list", "--json")
	requireExit(t, code, exitOK, out, stderr)
	var tags []api.Tag
	if err := json.Unmarshal([]byte(out), &tags); err != nil {
		t.Fatalf("tag list: stdout is not valid JSON: %v\n%s", err, out)
	}
}

// --- Mutating (только YT_INTEGRATION=1 + YT_INTEGRATION_MUTATE=1) ---

func TestIntegrationCreateProjectResolve(t *testing.T) {
	baseURL, token := integrationConfig(t)
	requireMutate(t)
	proj := findLiveProject(t, baseURL, token)

	// Резолвинг --project (Атом 4.3, SPEC §3.4): ring-id как есть, shortName
	// без учёта регистра, name — по GET /admin/projects.
	cases := []struct {
		name    string
		project string
	}{
		{"shortName", proj.shortName},
		{"shortName lowercase", strings.ToLower(proj.shortName)},
		{"name", proj.name},
		{"ring-id", proj.id},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.project == "" {
				t.Skip("project value is empty")
			}
			id := createSmokeIssue(t, baseURL, token, tc.project, "resolve")
			out, stderr, code := runIntegration(t, baseURL, token, "issue", "view", id, "--json")
			requireExit(t, code, exitOK, out, stderr)
			var it api.Issue
			if err := json.Unmarshal([]byte(out), &it); err != nil {
				t.Fatalf("issue view: stdout is not valid JSON: %v\n%s", err, out)
			}
			if it.Project == nil || !strings.EqualFold(it.Project.ShortName, proj.shortName) {
				t.Errorf("issue %s project = %+v, want shortName %s", id, it.Project, proj.shortName)
			}
		})
	}
}

func TestIntegrationEdit(t *testing.T) {
	baseURL, token := integrationConfig(t)
	requireMutate(t)
	proj := findLiveProject(t, baseURL, token)
	id := createSmokeIssue(t, baseURL, token, proj.shortName, "edit")

	newTitle := "yt integration edited " + time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	out, stderr, code := runIntegration(t, baseURL, token, "issue", "edit", id, "--title", newTitle, "--body", "edited body")
	requireExit(t, code, exitOK, out, stderr)

	viewOut, viewStderr, viewCode := runIntegration(t, baseURL, token, "issue", "view", id, "--json")
	requireExit(t, viewCode, exitOK, viewOut, viewStderr)
	var it api.Issue
	if err := json.Unmarshal([]byte(viewOut), &it); err != nil {
		t.Fatalf("issue view: stdout is not valid JSON: %v\n%s", err, viewOut)
	}
	if it.Summary != newTitle || it.Description != "edited body" {
		t.Errorf("issue %s after edit: summary=%q description=%q, want %q / %q", id, it.Summary, it.Description, newTitle, "edited body")
	}
}

func TestIntegrationCommandBatchApply(t *testing.T) {
	baseURL, token := integrationConfig(t)
	requireMutate(t)
	proj := findLiveProject(t, baseURL, token)
	state := resolveState(t, baseURL, token)
	a := createSmokeIssue(t, baseURL, token, proj.shortName, "batch-a")
	b := createSmokeIssue(t, baseURL, token, proj.shortName, "batch-b")

	// Оба id применяются одним запросом (POST /commands).
	out, stderr, code := runIntegration(t, baseURL, token, "command", "-y", "--json", "state: "+state, a, b)
	requireExit(t, code, exitOK, out, stderr)
	var issues []api.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("command: stdout is not valid JSON: %v\n%s", err, out)
	}
	if len(issues) != 2 {
		t.Errorf("command: %d issues in response, want 2\n%s", len(issues), out)
	}
	for _, it := range issues {
		if it.Resolved == nil {
			t.Errorf("command: issue %s not resolved after batch", it.IDReadable)
		}
	}
}

func TestIntegrationCommandAtomicity(t *testing.T) {
	baseURL, token := integrationConfig(t)
	requireMutate(t)
	proj := findLiveProject(t, baseURL, token)
	state := resolveState(t, baseURL, token)
	a := createSmokeIssue(t, baseURL, token, proj.shortName, "atomicity")

	// Команда с несуществующей задачей в batch: сервер отклоняет ВЕСЬ запрос
	// (HTTP 400) — валидная задача не меняется (атомарность /commands,
	// SPEC §3.4: «изменения не применяются», фиксируется интеграционным тестом).
	nonexistent := fmt.Sprintf("%s-999999", proj.shortName)
	out, stderr, code := runIntegration(t, baseURL, token, "command", "-y", "state: "+state, a, nonexistent)
	if code == exitOK {
		t.Fatalf("command with nonexistent issue: exit = 0, want error; stdout=%q", out)
	}
	if !strings.Contains(stderr, "400") {
		t.Errorf("stderr = %q, want HTTP 400", stderr)
	}

	viewOut, viewStderr, viewCode := runIntegration(t, baseURL, token, "issue", "view", a, "--json")
	requireExit(t, viewCode, exitOK, viewOut, viewStderr)
	var it api.Issue
	if err := json.Unmarshal([]byte(viewOut), &it); err != nil {
		t.Fatalf("issue view: stdout is not valid JSON: %v\n%s", err, viewOut)
	}
	if it.Resolved != nil {
		t.Errorf("issue %s resolved = %v, want nil (batch не применён частично)", a, *it.Resolved)
	}
}

func TestIntegrationCommentCreate(t *testing.T) {
	baseURL, token := integrationConfig(t)
	requireMutate(t)
	proj := findLiveProject(t, baseURL, token)
	id := createSmokeIssue(t, baseURL, token, proj.shortName, "comment")
	text := "integration comment " + time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	out, stderr, code := runIntegration(t, baseURL, token, "issue", "comment", "create", id, "-m", text)
	requireExit(t, code, exitOK, out, stderr)

	listOut, listStderr, listCode := runIntegration(t, baseURL, token, "issue", "comment", "list", id, "--json")
	requireExit(t, listCode, exitOK, listOut, listStderr)
	var comments []api.IssueComment
	if err := json.Unmarshal([]byte(listOut), &comments); err != nil {
		t.Fatalf("issue comment list: stdout is not valid JSON: %v\n%s", err, listOut)
	}
	for _, c := range comments {
		if c.Text == text {
			return
		}
	}
	t.Errorf("comment %q not found in issue %s comments: %s", text, id, listOut)
}

func TestIntegrationDelete(t *testing.T) {
	baseURL, token := integrationConfig(t)
	requireMutate(t)
	proj := findLiveProject(t, baseURL, token)
	id := createSmokeIssue(t, baseURL, token, proj.shortName, "delete")

	out, stderr, code := runIntegration(t, baseURL, token, "issue", "delete", id, "-y")
	requireExit(t, code, exitOK, out, stderr)
	if !strings.Contains(out, id) {
		t.Errorf("delete: stdout = %q, want contains %q", out, id)
	}

	// Задача удалена: view возвращает 404, exit 1.
	_, viewStderr, viewCode := runIntegration(t, baseURL, token, "issue", "view", id, "--json")
	if viewCode != exitRuntime {
		t.Errorf("view deleted issue: exit = %d, want %d", viewCode, exitRuntime)
	}
	if !strings.Contains(viewStderr, "404") {
		t.Errorf("view deleted issue: stderr = %q, want 404", viewStderr)
	}
}
