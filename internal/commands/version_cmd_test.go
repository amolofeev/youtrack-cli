package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/amolofeev/yt/internal/version"
)

// setVersionForTest подменяет значения сборки на время теста и восстанавливает
// их после. Tests не параллелятся, поэтому глобальные переменные безопасны.
func setVersionForTest(t *testing.T, v, c, b string) {
	t.Helper()
	old := struct{ v, c, b string }{version.Version, version.Commit, version.Built}
	version.Version, version.Commit, version.Built = v, c, b
	t.Cleanup(func() {
		version.Version, version.Commit, version.Built = old.v, old.c, old.b
	})
}

// runCmd выполняет корневую команду с заданными аргументами и возвращает
// захваченные stdout/stderr.
func runCmd(args ...string) (string, string, error) {
	root := NewRootCommand()
	var out, errw bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errw)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errw.String(), err
}

func TestVersionTTY(t *testing.T) {
	setVersionForTest(t, "0.0.1-pre-alpha", "2036315", "2026-07-31T12:00:00Z")

	stdout, stderr, err := runCmd("version")
	if err != nil {
		t.Fatalf("version: unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("version: stderr = %q, want empty", stderr)
	}
	want := fmt.Sprintf("yt version 0.0.1-pre-alpha\n"+
		"commit: 2036315\n"+
		"built:  2026-07-31T12:00:00Z\n"+
		"go:     %s\n"+
		"os:     %s\n"+
		"arch:   %s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if stdout != want {
		t.Errorf("version TTY output mismatch\n got: %q\nwant: %q", stdout, want)
	}
}

func TestVersionJSON(t *testing.T) {
	setVersionForTest(t, "0.0.1-pre-alpha", "2036315", "2026-07-31T12:00:00Z")

	stdout, stderr, err := runCmd("version", "--json")
	if err != nil {
		t.Fatalf("version --json: unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("version --json: stderr = %q, want empty", stderr)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("version --json: invalid JSON %q: %v", stdout, err)
	}
	want := map[string]string{
		"version": "0.0.1-pre-alpha",
		"commit":  "2036315",
		"built":   "2026-07-31T12:00:00Z",
		"go":      runtime.Version(),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("version --json: field %q = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("version --json: got %d fields, want %d", len(got), len(want))
	}
}

func TestVersionFlag(t *testing.T) {
	setVersionForTest(t, "0.0.1-pre-alpha", "2036315", "2026-07-31T12:00:00Z")

	stdout, stderr, err := runCmd("--version")
	if err != nil {
		t.Fatalf("--version: unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("--version: stderr = %q, want empty", stderr)
	}
	if want := "yt version 0.0.1-pre-alpha\n"; stdout != want {
		t.Errorf("--version: stdout = %q, want %q", stdout, want)
	}
}

func TestVersionRejectsArgs(t *testing.T) {
	_, _, err := runCmd("version", "extra")
	if err == nil {
		t.Error("version with positional args: expected error, got nil")
	}
}
