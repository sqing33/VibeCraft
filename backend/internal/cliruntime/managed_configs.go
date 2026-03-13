package cliruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vibecraft/backend/internal/paths"
	"vibecraft/backend/internal/runner"
)

const defaultManagedOpenAIBaseURL = "https://api.openai.com/v1"

func ManagedRuntimeRoot(toolID string) (string, error) {
	root, err := paths.ManagedCLIRuntimesDir()
	if err != nil {
		return "", err
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		toolID = "runtime"
	}
	return filepath.Join(root, toolID), nil
}

func EnsureCodexHome(toolID string) (string, error) {
	root, err := ManagedRuntimeRoot(firstNonEmpty(toolID, "codex"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("mkdir codex managed home: %w", err)
	}
	configPath := filepath.Join(root, "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		return root, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat codex config: %w", err)
	}
	if err := os.WriteFile(configPath, []byte("# managed by vibecraft\n"), 0o600); err != nil {
		return "", fmt.Errorf("write codex config: %w", err)
	}
	return root, nil
}

func WriteCodexProviderConfig(toolID, baseURL string) (string, error) {
	homeDir, err := EnsureCodexHome(toolID)
	if err != nil {
		return "", err
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultManagedOpenAIBaseURL
	}
	baseURL = runner.NormalizeBaseURL("openai", baseURL)
	content := strings.TrimSpace(fmt.Sprintf(`
model_provider = "vibecraft"

[model_providers.vibecraft]
name = "vibecraft-managed"
base_url = %s
env_key = "OPENAI_API_KEY"
wire_api = "responses"
`, strconv.Quote(baseURL))) + "\n"
	configPath := filepath.Join(homeDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write codex provider config: %w", err)
	}
	return homeDir, nil
}

func EnsureClaudeSettingsFile(toolID string) (string, error) {
	root, err := ManagedRuntimeRoot(firstNonEmpty(toolID, "claude"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("mkdir claude managed root: %w", err)
	}
	settingsPath := filepath.Join(root, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		return settingsPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat claude settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, []byte("{}\n"), 0o600); err != nil {
		return "", fmt.Errorf("write claude settings: %w", err)
	}
	return settingsPath, nil
}

func WriteClaudeSettingsFile(toolID string, payload map[string]any) (string, error) {
	settingsPath, err := EnsureClaudeSettingsFile(toolID)
	if err != nil {
		return "", err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal claude settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write claude settings: %w", err)
	}
	return settingsPath, nil
}

func WriteClaudeMCPConfigFile(toolID string, payload map[string]any) (string, error) {
	root, err := ManagedRuntimeRoot(firstNonEmpty(toolID, "claude"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("mkdir claude managed root: %w", err)
	}
	path := filepath.Join(root, "mcp.json")
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal claude mcp config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write claude mcp config: %w", err)
	}
	return path, nil
}

func WriteOpenCodeGatewayConfig(toolID string, payload map[string]any) (string, error) {
	root, err := ManagedRuntimeRoot(firstNonEmpty(toolID, "opencode"))
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(root, "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir opencode managed root: %w", err)
	}
	path := filepath.Join(configDir, "opencode.json")
	base := map[string]any{
		"$schema": "https://opencode.ai/config.json",
	}
	for key, value := range payload {
		base[key] = value
	}
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal opencode gateway config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write opencode gateway config: %w", err)
	}
	return path, nil
}

func ClaudeGatewayPayloadFromEnv(env map[string]string) (map[string]any, bool) {
	name := strings.TrimSpace(env["VIBECRAFT_MCP_GATEWAY_NAME"])
	url := strings.TrimSpace(env["VIBECRAFT_MCP_GATEWAY_URL"])
	token := strings.TrimSpace(env["VIBECRAFT_MCP_GATEWAY_TOKEN"])
	if name == "" || url == "" || token == "" {
		return nil, false
	}
	return map[string]any{
		"mcpServers": map[string]any{
			name: map[string]any{
				"type": "http",
				"url":  url,
				"headers": map[string]string{
					"Authorization": "Bearer " + token,
				},
			},
		},
	}, true
}

func OpenCodeGatewayPayloadFromEnv(env map[string]string) (map[string]any, bool) {
	name := strings.TrimSpace(env["VIBECRAFT_MCP_GATEWAY_NAME"])
	url := strings.TrimSpace(env["VIBECRAFT_MCP_GATEWAY_URL"])
	token := strings.TrimSpace(env["VIBECRAFT_MCP_GATEWAY_TOKEN"])
	if name == "" || url == "" || token == "" {
		return nil, false
	}
	return map[string]any{
		"mcp": map[string]any{
			name: map[string]any{
				"type": "remote",
				"url":  url,
				"headers": map[string]string{
					"Authorization": "Bearer " + token,
				},
			},
		},
	}, true
}
