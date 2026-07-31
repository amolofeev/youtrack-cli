package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL     = "http://localhost:8080/api"
	DefaultLogLevel    = "error"
	DefaultHTTPTimeout = 30 * time.Second
)

const (
	envBaseURL     = "YT_BASE_URL"
	envToken       = "YT_TOKEN"
	envConfigHome  = "YT_CONFIG_HOME"
	envLogLevel    = "YT_LOG_LEVEL"
	envHTTPTimeout = "YT_HTTP_TIMEOUT"
	envNoColor     = "YT_NO_COLOR"
)

type Config struct {
	BaseURL string `yaml:"base_url"`
	Token   string `yaml:"token"`
}

func Default() *Config {
	return &Config{BaseURL: DefaultBaseURL}
}

func Path() (string, error) {
	base, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "yt", "config.yml"), nil
}

func configDir() (string, error) {
	if dir := os.Getenv(envConfigHome); dir != "" {
		return dir, nil
	}
	return os.UserConfigDir()
}

func Load() (*Config, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func Resolve(flagBaseURL, flagToken string) (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if v := os.Getenv(envBaseURL); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv(envToken); v != "" {
		cfg.Token = v
	}
	if flagBaseURL != "" {
		cfg.BaseURL = flagBaseURL
	}
	if flagToken != "" {
		cfg.Token = flagToken
	}
	return cfg, nil
}

func LogLevel() string {
	if v := os.Getenv(envLogLevel); v != "" {
		return v
	}
	return DefaultLogLevel
}

func HTTPTimeout() (time.Duration, error) {
	v := os.Getenv(envHTTPTimeout)
	if v == "" {
		return DefaultHTTPTimeout, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", envHTTPTimeout, v, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be positive", envHTTPTimeout, v)
	}
	return time.Duration(n) * time.Second, nil
}

func NoColor() bool {
	return os.Getenv(envNoColor) == "1"
}
