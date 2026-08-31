package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultPort = 8787

type Config struct {
	Server                 ServerConfig `json:"server"`
	Paths                  PathsConfig  `json:"paths"`
	Quota                  QuotaConfig  `json:"quota"`
	RefreshIntervalSeconds int          `json:"refreshIntervalSeconds"`
}

type ServerConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AutoStart bool   `json:"autoStart"`
}

type PathsConfig struct {
	ClaudeLogs        string `json:"claudeLogs"`
	ClaudeCredentials string `json:"claudeCredentials"`
	CodexLogs         string `json:"codexLogs"`
}

type QuotaConfig struct {
	ClaudeOAuth         bool `json:"claudeOAuth"`
	CodexCLI            bool `json:"codexCLI"`
	PollIntervalSeconds int  `json:"pollIntervalSeconds"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: defaultPort, AutoStart: true},
		Paths: PathsConfig{
			ClaudeLogs:        "~/.claude/projects",
			ClaudeCredentials: "~/.claude/.credentials.json",
			CodexLogs:         "~/.codex",
		},
		Quota:                  QuotaConfig{ClaudeOAuth: true, CodexCLI: true, PollIntervalSeconds: 300},
		RefreshIntervalSeconds: 10,
	}
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "agentsusage", "config.json"), nil
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := Save(path, cfg); err != nil {
			return Config{}, err
		}
		return ResolvePaths(cfg)
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}
	return ResolvePaths(cfg)
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Host) == "" {
		return errors.New("server.host must not be empty")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}
	if c.RefreshIntervalSeconds < 1 || c.RefreshIntervalSeconds > 3600 {
		return errors.New("refreshIntervalSeconds must be between 1 and 3600")
	}
	if strings.TrimSpace(c.Paths.ClaudeLogs) == "" {
		return errors.New("paths.claudeLogs must not be empty")
	}
	if strings.TrimSpace(c.Paths.CodexLogs) == "" {
		return errors.New("paths.codexLogs must not be empty")
	}
	if c.Quota.ClaudeOAuth && strings.TrimSpace(c.Paths.ClaudeCredentials) == "" {
		return errors.New("paths.claudeCredentials must not be empty when quota.claudeOAuth is enabled")
	}
	if c.Quota.PollIntervalSeconds < 30 || c.Quota.PollIntervalSeconds > 3600 {
		return errors.New("quota.pollIntervalSeconds must be between 30 and 3600")
	}
	return nil
}

func ResolvePaths(cfg Config) (Config, error) {
	var err error
	cfg.Paths.ClaudeLogs, err = ExpandPath(cfg.Paths.ClaudeLogs)
	if err != nil {
		return Config{}, fmt.Errorf("resolve Claude logs path: %w", err)
	}
	cfg.Paths.CodexLogs, err = ExpandPath(cfg.Paths.CodexLogs)
	if err != nil {
		return Config{}, fmt.Errorf("resolve Codex logs path: %w", err)
	}
	if strings.TrimSpace(cfg.Paths.ClaudeCredentials) != "" {
		cfg.Paths.ClaudeCredentials, err = ExpandPath(cfg.Paths.ClaudeCredentials)
		if err != nil {
			return Config{}, fmt.Errorf("resolve Claude credentials path: %w", err)
		}
	}
	return cfg, nil
}

func ExpandPath(value string) (string, error) {
	value = os.ExpandEnv(strings.TrimSpace(value))
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, value[2:])
		}
	}
	abs, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", err
	}
	return abs, nil
}

func (s ServerConfig) Address() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func (s ServerConfig) DashboardURL() string {
	host := strings.TrimSpace(s.Host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(s.Port))}).String()
}
