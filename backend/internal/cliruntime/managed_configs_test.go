package cliruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCodexProviderConfig_WritesManagedHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	homeDir, err := WriteCodexProviderConfig("codex", "https://example.com/v1")
	if err != nil {
		t.Fatalf("WriteCodexProviderConfig() error = %v", err)
	}
	configPath := filepath.Join(homeDir, "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(content), `base_url = "https://example.com/v1"`) {
		t.Fatalf("codex config missing base_url, got:\n%s", string(content))
	}
}

func TestWriteClaudeSettingsFile_WritesManagedSettings(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	settingsPath, err := WriteClaudeSettingsFile("claude", map[string]any{"source": "managed"})
	if err != nil {
		t.Fatalf("WriteClaudeSettingsFile() error = %v", err)
	}
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read claude settings: %v", err)
	}
	if !strings.Contains(string(content), `"source": "managed"`) {
		t.Fatalf("claude settings missing payload, got:\n%s", string(content))
	}
}

func TestWriteCodexProviderConfig_NormalizesOpenAIBaseURL(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	homeDir, err := WriteCodexProviderConfig("codex", "https://example.com")
	if err != nil {
		t.Fatalf("WriteCodexProviderConfig() error = %v", err)
	}
	configPath := filepath.Join(homeDir, "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(content), `base_url = "https://example.com/v1"`) {
		t.Fatalf("codex config missing normalized base_url, got:\n%s", string(content))
	}
}
