package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/skillcatalog"
)

func TestNormalizeMCPServers_RejectsDefaultOutsideEnabledTools(t *testing.T) {
	tools := []config.CLIToolConfig{{ID: "codex", Enabled: true}, {ID: "claude", Enabled: true}}
	servers := []config.MCPServerConfig{{
		ID:                       "filesystem",
		Enabled:                  true,
		EnabledCLIToolIDs:        []string{"codex"},
		DefaultEnabledCLIToolIDs: []string{"claude"},
		Config:                   map[string]any{"command": "npx"},
	}}
	if err := config.NormalizeMCPServers(&servers, tools); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDefaultEnabledMCPServerIDs_ReturnsSortedMatches(t *testing.T) {
	cfg := config.Config{MCPServers: []config.MCPServerConfig{
		{ID: "zeta", Enabled: true, DefaultEnabledCLIToolIDs: []string{"codex"}, EnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "z"}},
		{ID: "alpha", Enabled: true, DefaultEnabledCLIToolIDs: []string{"codex"}, EnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "a"}},
		{ID: "off", Enabled: false, DefaultEnabledCLIToolIDs: []string{"codex"}, EnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "off"}},
	}}
	got := config.DefaultEnabledMCPServerIDs(cfg, "codex")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("unexpected ids: %#v", got)
	}
}

func TestMergeDiscoveredSkillBindings_DefaultsNewSkillToAllTools(t *testing.T) {
	cfg := config.Config{CLITools: []config.CLIToolConfig{{ID: "claude"}, {ID: "codex"}}}
	merged := config.MergeDiscoveredSkillBindings(cfg, []skillcatalog.Entry{{
		ID:          "new-skill",
		Description: "desc",
		Path:        "/tmp/SKILL.md",
	}})
	if len(merged) != 1 {
		t.Fatalf("unexpected merged size: %d", len(merged))
	}
	if !merged[0].Enabled {
		t.Fatalf("expected discovered binding enabled")
	}
	if len(merged[0].EnabledCLIToolIDs) != 2 || merged[0].EnabledCLIToolIDs[0] != "claude" || merged[0].EnabledCLIToolIDs[1] != "codex" {
		t.Fatalf("unexpected tool defaults: %#v", merged[0].EnabledCLIToolIDs)
	}
}

func TestEffectiveSkillCatalogEntries_RespectsExpertIntersection(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, "ui-ux-pro-max", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: ui-ux-pro-max\ndescription: ui\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	cfg := config.Config{
		CLITools: []config.CLIToolConfig{{ID: "codex"}},
		SkillBindings: []config.SkillBindingConfig{
			{ID: "ui-ux-pro-max", Path: skillPath, Enabled: true, EnabledCLIToolIDs: []string{"codex"}},
			{ID: "worktree-lite", Path: skillPath, Enabled: true, EnabledCLIToolIDs: []string{"codex"}},
		},
	}
	got := config.EffectiveSkillCatalogEntries(cfg, "codex", []string{"ui-ux-pro-max"}, nil)
	if len(got) != 1 || got[0].ID != "ui-ux-pro-max" {
		t.Fatalf("unexpected effective skills: %#v", got)
	}
}
