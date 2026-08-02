package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amolofeev/youtrack-cli/internal/api"
	"github.com/spf13/cobra"
)

// testRoot возвращает root с дополнительной тестовой командой, RunE которой
// вызывает fn (после pipeline).
func testRoot(fn func(cmd *cobra.Command) error) *cobra.Command {
	root := NewRootCommand()
	root.AddCommand(&cobra.Command{
		Use:  "testpipeline",
		Args: argsValidator(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fn(cmd)
		},
	})
	return root
}

// runRoot выполняет root с аргументами, захватывая stdout/stderr и код выхода.
func runRoot(t *testing.T, root *cobra.Command, args ...string) (string, string, int) {
	t.Helper()
	var out, errw strings.Builder
	root.SetOut(&out)
	root.SetErr(&errw)
	root.SetArgs(args)
	code := run(root)
	return out.String(), errw.String(), code
}

// isolatedConfig изолирует чтение конфигурации на время теста и сбрасывает токен.
func isolatedConfig(t *testing.T) {
	t.Helper()
	t.Setenv("YT_CONFIG_HOME", t.TempDir())
	t.Setenv("YT_TOKEN", "")
	t.Setenv("YT_BASE_URL", "")
}

func TestExitCodeUnknownCommand(t *testing.T) {
	var out, errw strings.Builder
	code := RunArgs([]string{"bogus"}, &out, &errw)
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if want := `yt: unknown command "bogus" for "yt"`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestExitCodeUnknownFlag(t *testing.T) {
	var out, errw strings.Builder
	code := RunArgs([]string{"--bogus"}, &out, &errw)
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if want := "yt: unknown flag: --bogus"; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestExitCodeInvalidArgs(t *testing.T) {
	var out, errw strings.Builder
	code := RunArgs([]string{"version", "extra"}, &out, &errw)
	if code != exitUsage {
		t.Errorf("code = %d, want %d", code, exitUsage)
	}
	if want := `yt: unknown command "extra" for "yt version"`; !strings.Contains(errw.String(), want) {
		t.Errorf("stderr = %q, want contains %q", errw.String(), want)
	}
}

func TestExitCodeRuntimeError(t *testing.T) {
	isolatedConfig(t)
	_, stderr, code := runRoot(t, testRoot(func(*cobra.Command) error {
		return context.DeadlineExceeded
	}), "testpipeline")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := "yt: context deadline exceeded\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, exitOK},
		{"plain error", context.DeadlineExceeded, exitRuntime},
		{"usage", usageError(context.DeadlineExceeded), exitUsage},
		{"canceled", context.Canceled, exitCancel},
		{"api canceled", &api.Error{Type: api.ErrorCanceled, Err: context.Canceled}, exitCancel},
		{"api http", &api.Error{Type: api.ErrorHTTP, Code: 404, Title: "Nope"}, exitRuntime},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := exitCodeFor(c.err); got != c.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	if got, want := formatError(usageError(context.DeadlineExceeded)), "context deadline exceeded"; got != want {
		t.Errorf("formatError(usage) = %q, want %q", got, want)
	}
	apiErr := &api.Error{Type: api.ErrorHTTP, Code: 404, Title: "Issue NOPE not found"}
	if got, want := formatError(apiErr), "request failed: 404 Issue NOPE not found"; got != want {
		t.Errorf("formatError(api) = %q, want %q", got, want)
	}
}

func TestNoTokenProvided(t *testing.T) {
	isolatedConfig(t)
	root := testRoot(func(cmd *cobra.Command) error {
		_, err := requireClient(cmd)
		return err
	})
	_, stderr, code := runRoot(t, root, "testpipeline")
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := `yt: no token provided: run "yt auth login" or set YT_TOKEN` + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestPipelineConfigPriority(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("YT_BASE_URL", "http://env.example/api")
	t.Setenv("YT_TOKEN", "env-token")

	check := func(cmd *cobra.Command) error {
		cfg := configFromContext(cmd)
		if cfg == nil {
			return context.DeadlineExceeded
		}
		if cfg.BaseURL != "http://flag.example/api" {
			return &api.Error{Type: api.ErrorHTTP, Code: 500, Title: "baseURL = " + cfg.BaseURL}
		}
		if cfg.Token != "flag-token" {
			return &api.Error{Type: api.ErrorHTTP, Code: 500, Title: "token mismatch"}
		}
		return nil
	}

	// env должен уступать флагам (флаг > env > config > дефолт, §3.2).
	_, stderr, code := runRoot(t, testRoot(check), "testpipeline",
		"--base-url", "http://flag.example/api", "--token", "flag-token")
	if code != exitOK {
		t.Errorf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}

	// без флагов берётся env.
	checkEnv := func(cmd *cobra.Command) error {
		cfg := configFromContext(cmd)
		if cfg.BaseURL != "http://env.example/api" || cfg.Token != "env-token" {
			return &api.Error{Type: api.ErrorHTTP, Code: 500, Title: "env not applied"}
		}
		return nil
	}
	if _, stderr, code = runRoot(t, testRoot(checkEnv), "testpipeline"); code != exitOK {
		t.Errorf("env priority: code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
}

// pipelineServer — фейковый YouTrack, проверяющий Authorization-заголовок.
func pipelineServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			http.Error(w, `{"error":"unauthorized","error_description":"bad token"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"me","id":"2-1"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPipelineDebugLogging(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	srv := pipelineServer(t)

	root := testRoot(func(cmd *cobra.Command) error {
		client, err := requireClient(cmd)
		if err != nil {
			return err
		}
		var out map[string]any
		return client.Get(cmd.Context(), "/users/me", nil, &out)
	})

	// --verbose включает debug-лог в stderr (без токена и без scheme/host).
	_, stderr, code := runRoot(t, root, "testpipeline", "--base-url", srv.URL, "--verbose")
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	for _, want := range []string{"DBG GET /users/me status=200 dur=", "DBG"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want contains %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "secret-token") {
		t.Errorf("stderr leaked token: %q", stderr)
	}
	if strings.Contains(stderr, srv.URL) {
		t.Errorf("stderr contains full base URL: %q", stderr)
	}
	if !strings.Contains(stderr, "status=200") {
		t.Errorf("stderr = %q, want status=200", stderr)
	}
}

func TestPipelineNoLogByDefault(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	srv := pipelineServer(t)

	root := testRoot(func(cmd *cobra.Command) error {
		client, err := requireClient(cmd)
		if err != nil {
			return err
		}
		var out map[string]any
		return client.Get(cmd.Context(), "/users/me", nil, &out)
	})
	_, stderr, code := runRoot(t, root, "testpipeline", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty (лог выключен по умолчанию)", stderr)
	}
}

func TestPipelineLogLevelEnv(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	t.Setenv("YT_LOG_LEVEL", "debug")
	srv := pipelineServer(t)

	root := testRoot(func(cmd *cobra.Command) error {
		client, err := requireClient(cmd)
		if err != nil {
			return err
		}
		var out map[string]any
		return client.Get(cmd.Context(), "/users/me", nil, &out)
	})
	_, stderr, code := runRoot(t, root, "testpipeline", "--base-url", srv.URL)
	if code != exitOK {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitOK, stderr)
	}
	if !strings.Contains(stderr, "DBG GET /users/me status=200") {
		t.Errorf("YT_LOG_LEVEL=debug: stderr = %q, want DBG-строку", stderr)
	}
}

func TestPipelineAPIErrorFormat(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("YT_TOKEN", "secret-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Issue NOPE not found","error_description":"no such issue"}`))
	}))
	defer srv.Close()

	root := testRoot(func(cmd *cobra.Command) error {
		client, err := requireClient(cmd)
		if err != nil {
			return err
		}
		var out any
		return client.Get(cmd.Context(), "/issues/NOPE", nil, &out)
	})
	_, stderr, code := runRoot(t, root, "testpipeline", "--base-url", srv.URL)
	if code != exitRuntime {
		t.Errorf("code = %d, want %d", code, exitRuntime)
	}
	if want := "yt: request failed: 404 Issue NOPE not found: no such issue\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}
