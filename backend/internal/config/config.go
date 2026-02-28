package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Server    ServerConfig    `json:"server"`
	Execution ExecutionConfig `json:"execution"`
	Experts   []ExpertConfig  `json:"experts"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type ExecutionConfig struct {
	MaxConcurrency int `json:"max_concurrency"`
	KillGraceMs    int `json:"kill_grace_ms"`
}

type ExpertConfig struct {
	ID        string            `json:"id"`
	Label     string            `json:"label"`
	RunMode   string            `json:"run_mode"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	TimeoutMs int               `json:"timeout_ms"`
	SDK       *ExpertSDKConfig  `json:"sdk,omitempty"`
}

type ExpertSDKConfig struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	BaseURL         string   `json:"base_url,omitempty"`
	Instructions    string   `json:"instructions,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	OutputSchema    string   `json:"output_schema,omitempty"`
}

// Default 功能：返回一份可直接运行的默认配置（localhost-only）。
// 参数/返回：无入参；返回默认 Config。
// 失败场景：无。
// 副作用：无。
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 7777,
		},
		Execution: ExecutionConfig{
			MaxConcurrency: 6,
			KillGraceMs:    1500,
		},
		Experts: []ExpertConfig{
			{
				ID:      "master",
				Label:   "Master Planner (Anthropic SDK)",
				RunMode: "sdk",
				SDK: &ExpertSDKConfig{
					Provider:        "anthropic",
					Model:           "claude-3-7-sonnet-latest",
					MaxOutputTokens: 4096,
					OutputSchema:    "dag_v1",
				},
				Env: map[string]string{
					"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
				},
				TimeoutMs: 30 * 60 * 1000,
			},
			{
				ID:      "bash",
				Label:   "Bash",
				RunMode: "oneshot",
				Command: "bash",
				Args:    []string{"-lc", "{{prompt}}"},
				Env:     map[string]string{},
				// 30min: MVP 默认超时时间（后续由 scheduler/execution 实际 enforce）。
				TimeoutMs: 30 * 60 * 1000,
			},
			{
				ID:      "codex",
				Label:   "Codex (OpenAI SDK)",
				RunMode: "sdk",
				SDK: &ExpertSDKConfig{
					Provider:        "openai",
					Model:           "gpt-5-codex",
					MaxOutputTokens: 8192,
				},
				Env: map[string]string{
					"OPENAI_API_KEY": "${OPENAI_API_KEY}",
				},
				TimeoutMs: 30 * 60 * 1000,
			},
			{
				ID:      "claudecode",
				Label:   "ClaudeCode (Anthropic SDK)",
				RunMode: "sdk",
				SDK: &ExpertSDKConfig{
					Provider:        "anthropic",
					Model:           "claude-3-7-sonnet-latest",
					MaxOutputTokens: 4096,
				},
				Env: map[string]string{
					"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}",
				},
				TimeoutMs: 30 * 60 * 1000,
			},
		},
	}
}

// Addr 功能：拼接 server 的监听地址（host:port）。
// 参数/返回：返回用于 http.Server.Addr 的字符串。
// 失败场景：无。
// 副作用：无。
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Path 功能：解析配置文件路径（XDG 优先）。
// 参数/返回：返回 `~/.config/vibe-tree/config.json`（或 $XDG_CONFIG_HOME）下的路径。
// 失败场景：无法解析用户 home 目录时返回 error。
// 副作用：读取环境变量与 home 目录信息。
func Path() (string, error) {
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		xdgConfigHome = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfigHome, "vibe-tree", "config.json"), nil
}

// Load 功能：读取 config.json 并合并环境变量覆盖，产出最终运行配置。
// 参数/返回：无入参；返回 Config、配置文件路径与错误信息。
// 失败场景：文件读取失败、JSON 解析失败或路径解析失败时返回 error。
// 副作用：读取磁盘文件与环境变量。
func Load() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Config{}, "", err
	}

	cfg := Default()

	if b, err := os.ReadFile(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			applyEnvOverrides(&cfg)
			return cfg, path, nil
		}
		return Config{}, "", fmt.Errorf("read config %s: %w", path, err)
	} else if len(b) > 0 {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return Config{}, "", fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnvOverrides(&cfg)
	return cfg, path, nil
}

func applyEnvOverrides(cfg *Config) {
	if host := os.Getenv("VIBE_TREE_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if portStr := os.Getenv("VIBE_TREE_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 && port <= 65535 {
			cfg.Server.Port = port
		}
	}
	if raw := os.Getenv("VIBE_TREE_MAX_CONCURRENCY"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			cfg.Execution.MaxConcurrency = v
		}
	}
	if raw := os.Getenv("VIBE_TREE_KILL_GRACE_MS"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			cfg.Execution.KillGraceMs = v
		}
	}
}
