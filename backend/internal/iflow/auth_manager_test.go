package iflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBrowserAuthURL_ReconstructsWrappedURL(t *testing.T) {
	input := `
 iFlow OAuth Login
 1. Please copy the following link and open it in your browser:

 https://iflow.cn/oauth?loginMethod=phone&type=phone&redirect=https%3A%2F
 %2Fiflow.cn%2Foauth%2Fcode-display&state=abc123&client_id=10009311001
 2. Login to your iFlow account and authorize`
	got := ParseBrowserAuthURL(input)
	want := "https://iflow.cn/oauth?loginMethod=phone&type=phone&redirect=https%3A%2F%2Fiflow.cn%2Foauth%2Fcode-display&state=abc123&client_id=10009311001"
	if got != want {
		t.Fatalf("ParseBrowserAuthURL() = %q, want %q", got, want)
	}
}

func TestEnsureHome_ImportsUserOfficialAuth(t *testing.T) {
	home := t.TempDir()
	xdgData := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", xdgData)
	globalDir := filepath.Join(home, ".iflow")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir global iflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte(`{
  "selectedAuthType": "iflow",
  "modelName": "minimax-m2.5",
  "baseUrl": "https://apis.iflow.cn/v1",
  "apiKey": "sk-iflow-user",
  "searchApiKey": "sk-iflow-user"
}`), 0o644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}
	_, err := EnsureHome()
	if err != nil {
		t.Fatalf("EnsureHome() error = %v", err)
	}
	status, err := DetectBrowserAuthStatus()
	if err != nil {
		t.Fatalf("DetectBrowserAuthStatus() error = %v", err)
	}
	if !status.Authenticated {
		t.Fatalf("expected imported browser auth")
	}
	if got := status.ModelName; got != "minimax-m2.5" {
		t.Fatalf("model name = %q, want minimax-m2.5", got)
	}
}

func TestEnsureHome_WritesBootstrapSettings(t *testing.T) {
	xdgData := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", xdgData)
	homeDir, err := EnsureHome()
	if err != nil {
		t.Fatalf("EnsureHome() error = %v", err)
	}
	if !strings.Contains(homeDir, filepath.Join(xdgData, "vibe-tree")) {
		t.Fatalf("homeDir = %q, want under %q", homeDir, xdgData)
	}
	settingsPath, err := SettingsPath()
	if err != nil {
		t.Fatalf("SettingsPath() error = %v", err)
	}
	payload, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	text := string(payload)
	for _, needle := range []string{"bootAnimationShown", "hasViewedOfflineOutput", "checkpointing"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("bootstrap settings missing %q: %s", needle, text)
		}
	}
}
