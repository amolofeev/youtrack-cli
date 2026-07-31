package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
}

func TestPathUsesConfigHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envConfigHome, home)
	p, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	want := filepath.Join(home, "yt", "config.yml")
	if p != want {
		t.Errorf("Path() = %q, want %q", p, want)
	}
}

func TestPathUsesUserConfigDir(t *testing.T) {
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("UserConfigDir unavailable: %v", err)
	}
	t.Setenv(envConfigHome, "")
	p, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	want := filepath.Join(dir, "yt", "config.yml")
	if p != want {
		t.Errorf("Path() = %q, want %q", p, want)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Setenv(envConfigHome, t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
}

func TestSaveCreatesDirsAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envConfigHome, home)
	want := &Config{BaseURL: "http://ytsrv:8080/api", Token: "perm:secret"}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	p, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.BaseURL != want.BaseURL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, want.BaseURL)
	}
	if got.Token != want.Token {
		t.Errorf("Token = %q, want %q", got.Token, want.Token)
	}
}

func TestSaveSets0600(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envConfigHome, home)
	if err := Save(&Config{BaseURL: DefaultBaseURL, Token: "perm:t"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	p, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 600", perm)
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envConfigHome, home)
	first := &Config{BaseURL: "http://a:1", Token: "one"}
	second := &Config{BaseURL: "http://b:2", Token: "two"}
	if err := Save(first); err != nil {
		t.Fatalf("Save(first) error: %v", err)
	}
	if err := Save(second); err != nil {
		t.Fatalf("Save(second) error: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.BaseURL != second.BaseURL || got.Token != second.Token {
		t.Errorf("got %+v, want %+v", got, second)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envConfigHome, home)
	p, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("base_url: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("Load() with invalid YAML: want error, got nil")
	}
}

func TestResolvePrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envConfigHome, home)

	if err := Save(&Config{BaseURL: "http://file:1", Token: "file-token"}); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cases := []struct {
		name      string
		envBase   string
		envToken  string
		flagBase  string
		flagToken string
		wantBase  string
		wantToken string
	}{
		{
			name:      "config only",
			wantBase:  "http://file:1",
			wantToken: "file-token",
		},
		{
			name:      "env over config",
			envBase:   "http://env:2",
			envToken:  "env-token",
			wantBase:  "http://env:2",
			wantToken: "env-token",
		},
		{
			name:      "flag over env",
			envBase:   "http://env:2",
			envToken:  "env-token",
			flagBase:  "http://flag:3",
			flagToken: "flag-token",
			wantBase:  "http://flag:3",
			wantToken: "flag-token",
		},
		{
			name:      "flag over config without env",
			flagBase:  "http://flag:3",
			wantBase:  "http://flag:3",
			wantToken: "file-token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envBaseURL, tc.envBase)
			t.Setenv(envToken, tc.envToken)
			cfg, err := Resolve(tc.flagBase, tc.flagToken)
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			if cfg.BaseURL != tc.wantBase {
				t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, tc.wantBase)
			}
			if cfg.Token != tc.wantToken {
				t.Errorf("Token = %q, want %q", cfg.Token, tc.wantToken)
			}
		})
	}
}

func TestResolveMissingFileUsesDefaults(t *testing.T) {
	t.Setenv(envConfigHome, t.TempDir())
	cfg, err := Resolve("", "")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
}

func TestLogLevel(t *testing.T) {
	t.Setenv(envLogLevel, "")
	if got := LogLevel(); got != DefaultLogLevel {
		t.Errorf("LogLevel() = %q, want %q", got, DefaultLogLevel)
	}
	t.Setenv(envLogLevel, "debug")
	if got := LogLevel(); got != "debug" {
		t.Errorf("LogLevel() = %q, want debug", got)
	}
}

func TestHTTPTimeout(t *testing.T) {
	t.Setenv(envHTTPTimeout, "")
	d, err := HTTPTimeout()
	if err != nil {
		t.Fatalf("HTTPTimeout() error: %v", err)
	}
	if d != DefaultHTTPTimeout {
		t.Errorf("HTTPTimeout() = %v, want %v", d, DefaultHTTPTimeout)
	}

	t.Setenv(envHTTPTimeout, "5")
	d, err = HTTPTimeout()
	if err != nil {
		t.Fatalf("HTTPTimeout() error: %v", err)
	}
	if d != 5*time.Second {
		t.Errorf("HTTPTimeout() = %v, want 5s", d)
	}

	for _, bad := range []string{"abc", "0", "-3", "1.5"} {
		t.Setenv(envHTTPTimeout, bad)
		if _, err := HTTPTimeout(); err == nil {
			t.Errorf("HTTPTimeout() with %q: want error, got nil", bad)
		}
	}
}

func TestNoColor(t *testing.T) {
	t.Setenv(envNoColor, "")
	if NoColor() {
		t.Error("NoColor() = true, want false")
	}
	t.Setenv(envNoColor, "1")
	if !NoColor() {
		t.Error("NoColor() = false, want true")
	}
	t.Setenv(envNoColor, "true")
	if NoColor() {
		t.Error("NoColor() with \"true\" = true, want false")
	}
}
