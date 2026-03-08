package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestSkillSettings_GetReturnsDiscoveredCatalog(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	skillPath := filepath.Join(home, ".codex", "skills", "my-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: my-skill\ndescription: local skill\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{ID: "codex", Label: "Codex", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	env := newTestEnv(t, cfg, 2)

	res, err := http.Get(env.httpSrv.URL + "/api/v1/settings/skills")
	if err != nil {
		t.Fatalf("get skill settings: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var got struct {
		Skills []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Path        string `json:"path"`
			Source      string `json:"source"`
		} `json:"skills"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	found := false
	for _, item := range got.Skills {
		if item.ID != "my-skill" {
			continue
		}
		found = true
		if item.Source == "" || item.Path == "" {
			t.Fatalf("expected source/path in response: %#v", item)
		}
	}
	if !found {
		t.Fatalf("expected my-skill in response: %#v", got.Skills)
	}
}
