package cliruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/runner"
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
	if err := os.WriteFile(configPath, []byte("# managed by vibe-tree\n"), 0o600); err != nil {
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
model_provider = "vibe_tree"

[model_providers.vibe_tree]
name = "vibe-tree-managed"
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
