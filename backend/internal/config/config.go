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
	LLM       *LLMSettings    `json:"llm,omitempty"`
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
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Description       string   `json:"description,omitempty"`
	Category          string   `json:"category,omitempty"`
	Avatar            string   `json:"avatar,omitempty"`
	ManagedSource     string   `json:"managed_source,omitempty"`
	PrimaryModelID    string   `json:"primary_model_id,omitempty"`
	SecondaryModelID  string   `json:"secondary_model_id,omitempty"`
	FallbackOn        []string `json:"fallback_on,omitempty"`
	EnabledSkills     []string `json:"enabled_skills,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	BuilderExpertID   string   `json:"builder_expert_id,omitempty"`
	BuilderSessionID  string   `json:"builder_session_id,omitempty"`
	BuilderSnapshotID string   `json:"builder_snapshot_id,omitempty"`
	GeneratedBy       string   `json:"generated_by,omitempty"`
	GeneratedAt       int64    `json:"generated_at,omitempty"`
	UpdatedAt         int64    `json:"updated_at,omitempty"`
	Disabled          bool     `json:"disabled,omitempty"`

	// Provider 表示该 expert 的执行后端（SDK 驱动，不再启动外部 CLI）。
	// 支持值：
	// - "openai"：Codex（OpenAI SDK）
	// - "anthropic"：ClaudeCode（Anthropic SDK）
	// - "demo"：内置演示（不依赖外部网络/密钥）
	// - "process"：本地进程执行（兼容 bash 等 worker）
	Provider string `json:"provider"`

	// Model 为 SDK 调用的模型名；demo 可留空。
	Model             string            `json:"model"`
	SecondaryProvider string            `json:"secondary_provider,omitempty"`
	SecondaryModel    string            `json:"secondary_model,omitempty"`
	SecondaryBaseURL  string            `json:"secondary_base_url,omitempty"`
	SecondaryEnv      map[string]string `json:"secondary_env,omitempty"`

	// BaseURL 可选：覆盖 SDK 的 base URL（支持 `${ENV}` 注入）。
	BaseURL string `json:"base_url,omitempty"`

	// PromptTemplate 可选：支持 `{{prompt}}` 与 `{{workspace}}` 占位。
	// 留空表示直接使用节点 prompt。
	PromptTemplate string `json:"prompt_template"`

	// SystemPrompt 可选：作为 system 角色注入（不同 provider 语义略有差异，MVP 先按通用文本处理）。
	SystemPrompt string `json:"system_prompt"`

	// MaxOutputTokens/Temperature 为可选采样参数（0 表示由 SDK/模型默认值决定）。
	MaxOutputTokens int      `json:"max_output_tokens"`
	Temperature     *float64 `json:"temperature,omitempty"`

	// OutputSchema 可选：structured output schema 名称（MVP：仅支持 "dag_v1"）。
	OutputSchema string `json:"output_schema,omitempty"`

	// Env 用于注入敏感配置（如 API Key），支持 `${ENV}` 模板替换。
	Env map[string]string `json:"env"`

	TimeoutMs int `json:"timeout_ms"`

	// Deprecated：旧版 CLI/PTY 配置字段（SDK 驱动后不再生效，仅用于给出更清晰的报错信息）。
	RunMode string   `json:"run_mode,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

type LLMSettings struct {
	Sources []LLMSourceConfig `json:"sources"`
	Models  []LLMModelConfig  `json:"models"`
}

type LLMSourceConfig struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`

	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

type LLMModelConfig struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`

	Model    string `json:"model"`
	SourceID string `json:"source_id"`

	SystemPrompt    string   `json:"system_prompt,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	OutputSchema    string   `json:"output_schema,omitempty"`
	TimeoutMs       int      `json:"timeout_ms,omitempty"`
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
				ID:            "master",
				Label:         "Master Planner",
				ManagedSource: ManagedSourceBuiltin,
				Provider:      "anthropic",
				Model:         "claude-3-7-sonnet-latest",
				SystemPrompt:  "You are the workflow master planner for vibe-tree. Output MUST be a single JSON object (no markdown, no extra text).",
				Env:           map[string]string{"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}"},
				OutputSchema:  "dag_v1",
				// 30min：AI 节点默认超时（可按节点覆盖）。
				TimeoutMs: 30 * 60 * 1000,
			},
			{
				ID:            "bash",
				Label:         "Bash",
				ManagedSource: ManagedSourceBuiltin,
				Provider:      "process",
				Command:       "bash",
				Args:          []string{"-lc", "{{prompt}}"},
				Env:           map[string]string{},
				// 30min：bash 节点默认超时（后续由 scheduler/execution 实际 enforce）。
				TimeoutMs: 30 * 60 * 1000,
			},
			{
				ID:            "demo",
				Label:         "Demo",
				ManagedSource: ManagedSourceBuiltin,
				Provider:      "demo",
				Env:           map[string]string{},
				// 30s：演示执行默认超时（不会触发网络请求）。
				TimeoutMs: 30 * 1000,
			},
			{
				ID:            "codex",
				Label:         "Codex",
				ManagedSource: ManagedSourceBuiltin,
				Provider:      "openai",
				Model:         "gpt-5-codex",
				SystemPrompt:  "You are Codex. Respond in plain text suitable for a terminal. Do not use markdown unless explicitly requested.",
				Env:           map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}"},
				// 30min：AI 节点默认超时（可按节点覆盖）。
				TimeoutMs: 30 * 60 * 1000,
			},
			{
				ID:            "claudecode",
				Label:         "ClaudeCode",
				ManagedSource: ManagedSourceBuiltin,
				Provider:      "anthropic",
				Model:         "claude-3-7-sonnet-latest",
				SystemPrompt:  "You are Claude. Respond in plain text suitable for a terminal. Do not use markdown unless explicitly requested.",
				Env:           map[string]string{"ANTHROPIC_API_KEY": "${ANTHROPIC_API_KEY}"},
				TimeoutMs:     30 * 60 * 1000,
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
			if err := RebuildExperts(&cfg); err != nil {
				return Config{}, "", err
			}
			return cfg, path, nil
		}
		return Config{}, "", fmt.Errorf("read config %s: %w", path, err)
	} else if len(b) > 0 {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return Config{}, "", fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnvOverrides(&cfg)
	if err := RebuildExperts(&cfg); err != nil {
		return Config{}, "", err
	}
	return cfg, path, nil
}

// LoadPersisted 功能：读取 config.json（不应用环境变量覆盖），用于“写盘前读现状”场景。
// 参数/返回：无入参；返回 Config、配置文件路径与错误信息。
// 失败场景：文件读取失败、JSON 解析失败或路径解析失败时返回 error。
// 副作用：读取磁盘文件与 home 目录信息（间接）。
func LoadPersisted() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Config{}, "", err
	}

	cfg := Default()

	if b, err := os.ReadFile(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := RebuildExperts(&cfg); err != nil {
				return Config{}, "", err
			}
			return cfg, path, nil
		}
		return Config{}, "", fmt.Errorf("read config %s: %w", path, err)
	} else if len(b) > 0 {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return Config{}, "", fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	if err := RebuildExperts(&cfg); err != nil {
		return Config{}, "", err
	}
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
