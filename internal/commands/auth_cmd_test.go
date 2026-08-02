package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/amolofeev/youtrack-cli/internal/config"
	"github.com/spf13/cobra"
)

const userFixture = `{"$type":"User","id":"1-1","login":"alex","fullName":"Alex","email":"alex@example.com","guest":false,"avatarUrl":"https://example.com/a.png"}`

func userServer(t *testing.T, authToken string) (*httptest.Server, *string) {
	t.Helper()
	var gotAuth string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/me" {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		mu.Lock()
		gotAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(userFixture))
	}))
	t.Cleanup(srv.Close)
	if authToken != "" {
		return srv, &gotAuth
	}
	return srv, nil
}

func runRootIn(t *testing.T, root *cobra.Command, in io.Reader, args ...string) (string, string, int) {
	t.Helper()
	var out, errw strings.Builder
	root.SetOut(&out)
	root.SetErr(&errw)
	root.SetIn(in)
	root.SetArgs(args)
	code := run(root)
	return out.String(), errw.String(), code
}

func TestLoginWithToken(t *testing.T) {
	isolatedConfig(t)
	srv, gotAuth := userServer(t, "perm:login-token")

	stdout, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader(""),
		"auth", "login", "--with-token", "perm:login-token", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if stdout == "" {
		t.Error("stdout is empty, want ✓ Authenticated")
	}
	if !strings.Contains(stdout, "✓ Authenticated as alex (Alex)") {
		t.Errorf("stdout = %q, want ✓ Authenticated as alex (Alex)", stdout)
	}
	if *gotAuth != "Bearer perm:login-token" {
		t.Errorf("Authorization = %q, want Bearer perm:login-token", *gotAuth)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.Token != "perm:login-token" {
		t.Errorf("saved token = %q, want perm:login-token", cfg.Token)
	}
	if cfg.BaseURL != srv.URL {
		t.Errorf("saved baseURL = %q, want %q", cfg.BaseURL, srv.URL)
	}
}

func TestLoginTokenFromStdin(t *testing.T) {
	isolatedConfig(t)
	srv, _ := userServer(t, "")

	stdout, stderr, code := runRootIn(t, NewRootCommand(),
		strings.NewReader("perm:stdin-token\n"),
		"auth", "login", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if !strings.Contains(stdout, "✓ Authenticated as alex (Alex)") {
		t.Errorf("stdout = %q, want ✓ Authenticated", stdout)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.Token != "perm:stdin-token" {
		t.Errorf("saved token = %q, want perm:stdin-token", cfg.Token)
	}
}

func TestLoginUnauthorizedDoesNotSave(t *testing.T) {
	isolatedConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Authentication required","error_description":"No token"}`))
	}))
	defer srv.Close()

	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "login", "--with-token", "bad", "--base-url", srv.URL}, &out, &errw)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := `yt: not logged in or token is invalid, run "yt auth login"`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if _, err := config.Path(); err != nil {
		t.Fatalf("config.Path() error: %v", err)
	}
	path, _ := config.Path()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("config file %s exists, want not saved on auth error", path)
	}
}

func TestLoginTokenRequired(t *testing.T) {
	isolatedConfig(t)
	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "login"}, &out, &errw)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := "yt: token is required"; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
}

func TestLoginInvalidArgs(t *testing.T) {
	isolatedConfig(t)
	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "login", "extra"}, &out, &errw)
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if want := `yt: unknown command "extra" for "yt auth login"`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
}

func TestAuthBarePrintsHelp(t *testing.T) {
	stdout, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader(""), "auth")
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "login") || !strings.Contains(stdout, "logout") || !strings.Contains(stdout, "status") {
		t.Errorf("stdout = %q, want help with auth subcommands", stdout)
	}
}

func TestAuthUnknownSubcommand(t *testing.T) {
	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "bogus"}, &out, &errw)
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if want := `yt: unknown command "bogus" for "yt auth"`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
}

func TestLogout(t *testing.T) {
	isolatedConfig(t)
	if err := config.Save(&config.Config{BaseURL: "http://localhost:8080/api", Token: "perm:x"}); err != nil {
		t.Fatalf("config.Save() error: %v", err)
	}

	out, errw, code := runRootIn(t, NewRootCommand(), strings.NewReader(""), "auth", "logout")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, errw)
	}
	if !strings.Contains(out, "✓ Logged out") {
		t.Errorf("stdout = %q, want ✓ Logged out", out)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("token after logout = %q, want empty", cfg.Token)
	}
	if cfg.BaseURL != "http://localhost:8080/api" {
		t.Errorf("baseURL after logout = %q, want preserved", cfg.BaseURL)
	}
}

func TestLogoutNotLoggedIn(t *testing.T) {
	isolatedConfig(t)
	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "logout"}, &out, &errw)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := "yt: not logged in"; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestStatusTTY(t *testing.T) {
	isolatedConfig(t)
	srv, _ := userServer(t, "")

	stdout, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader(""),
		"auth", "status", "--base-url", srv.URL, "--token", "perm:x")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	for _, want := range []string{
		"Server:   " + srv.URL,
		"Login:    alex",
		"Name:     Alex",
		"Email:    alex@example.com",
		"Guest:    false",
		"✓ Authenticated",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want contains %q", stdout, want)
		}
	}
}

func TestStatusJSON(t *testing.T) {
	isolatedConfig(t)
	srv, _ := userServer(t, "")

	stdout, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader(""),
		"auth", "status", "--base-url", srv.URL, "--token", "perm:x", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	var got struct {
		BaseURL  string `json:"baseUrl"`
		Login    string `json:"login"`
		FullName string `json:"fullName"`
		Email    string `json:"email"`
		Guest    bool   `json:"guest"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if got.BaseURL != srv.URL || got.Login != "alex" || got.FullName != "Alex" || got.Email != "alex@example.com" || got.Guest {
		t.Errorf("json = %+v, want baseUrl=%s alex", got, srv.URL)
	}
}

func TestStatus401(t *testing.T) {
	isolatedConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
	}))
	defer srv.Close()

	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "status", "--base-url", srv.URL, "--token", "bad"}, &out, &errw)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := "yt: ✗ not logged in"; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestStatusNoToken(t *testing.T) {
	isolatedConfig(t)
	var out, errw strings.Builder
	code := RunArgs([]string{"auth", "status"}, &out, &errw)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := `yt: no token provided: run "yt auth login" or set YT_TOKEN`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
}

func TestWhoamiTTY(t *testing.T) {
	isolatedConfig(t)
	srv, _ := userServer(t, "")

	stdout, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader(""),
		"user", "whoami", "--base-url", srv.URL, "--token", "perm:x")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	for _, want := range []string{
		"Login:    alex",
		"Name:     Alex",
		"Email:    alex@example.com",
		"Guest:    false",
		"ID:       1-1",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want contains %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "avatarUrl") {
		t.Errorf("stdout = %q, TTY не должен содержать avatarUrl", stdout)
	}
}

func TestWhoamiJSON(t *testing.T) {
	isolatedConfig(t)
	srv, _ := userServer(t, "")

	stdout, stderr, code := runRootIn(t, NewRootCommand(), strings.NewReader(""),
		"user", "whoami", "--base-url", srv.URL, "--token", "perm:x", "--json")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	var got struct {
		Type     string `json:"$type"`
		ID       string `json:"id"`
		Login    string `json:"login"`
		FullName string `json:"fullName"`
		Email    string `json:"email"`
		Guest    bool   `json:"guest"`
		Avatar   string `json:"avatarUrl"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if got.Type != "User" || got.ID != "1-1" || got.Login != "alex" || got.Avatar != "https://example.com/a.png" {
		t.Errorf("json = %+v, want raw user object", got)
	}
}

func TestWhoami401(t *testing.T) {
	isolatedConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
	}))
	defer srv.Close()

	var out, errw strings.Builder
	code := RunArgs([]string{"user", "whoami", "--base-url", srv.URL, "--token", "bad"}, &out, &errw)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := `yt: not logged in or token is invalid, run "yt auth login"`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
}
