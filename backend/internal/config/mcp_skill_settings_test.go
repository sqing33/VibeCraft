package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/skillcatalog"
)

func TestParseMCPServersJSON_SupportsWrappedAndFlatShapes(t *testing.T) {
	wrapped, err := config.ParseMCPServersJSON(`{"mcpServers":{"mcp-router":{"command":"npx"}}}`)
	if err != nil {
		t.Fatalf("parse wrapped: %v", err)
	}
	if len(wrapped) != 1 || wrapped[0].ID != "mcp-router" {
		t.Fatalf("unexpected wrapped result: %#v", wrapped)
	}

	flat, err := config.ParseMCPServersJSON(`{"mcp-router":{"command":"npx"}}`)
	if err != nil {
		t.Fatalf("parse flat: %v", err)
	}
	if len(flat) != 1 || flat[0].ID != "mcp-router" {
		t.Fatalf("unexpected flat result: %#v", flat)
	}
}

func TestNormalizeMCPServers_GeneratesRawJSONAndKeepsDefaults(t *testing.T) {
	tools := []config.CLIToolConfig{{ID: "codex", Enabled: true}, {ID: "claude", Enabled: true}}
	servers := []config.MCPServerConfig{{
		ID:                       "filesystem",
		DefaultEnabledCLIToolIDs: []string{"claude", "codex", "claude"},
		Config:                   map[string]any{"command": "npx"},
	}}
	if err := config.NormalizeMCPServers(&servers, tools); err != nil {
		t.Fatalf("normalize mcp: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("unexpected server size: %d", len(servers))
	}
	if len(servers[0].DefaultEnabledCLIToolIDs) != 2 {
		t.Fatalf("unexpected defaults: %#v", servers[0].DefaultEnabledCLIToolIDs)
	}
	if !strings.Contains(servers[0].RawJSON, "filesystem") {
		t.Fatalf("expected raw_json to contain id: %q", servers[0].RawJSON)
	}
}

func TestDefaultEnabledMCPServerIDs_ReturnsSortedMatches(t *testing.T) {
	cfg := config.Config{MCPServers: []config.MCPServerConfig{
		{ID: "zeta", DefaultEnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "z"}},
		{ID: "alpha", DefaultEnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "a"}},
		{ID: "off", DefaultEnabledCLIToolIDs: []string{"claude"}, Config: map[string]any{"command": "off"}},
	}}
	got := config.DefaultEnabledMCPServerIDs(cfg, "codex")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("unexpected ids: %#v", got)
	}
}

func TestEffectiveSkillCatalogEntries_RespectsExpertIntersection(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, "ui-ux-pro-max", "SKILL.md")
	otherPath := filepath.Join(dir, "worktree-lite", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(otherPath), 0o755); err != nil {
		t.Fatalf("mkdir other skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: ui-ux-pro-max\ndescription: ui\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	if err := os.WriteFile(otherPath, []byte("name: worktree-lite\ndescription: wt\n"), 0o644); err != nil {
		t.Fatalf("write other skill file: %v", err)
	}
	got := config.EffectiveSkillCatalogEntries(config.Config{}, "codex", []string{"ui-ux-pro-max"}, []skillcatalog.Entry{
		{ID: "ui-ux-pro-max", Path: skillPath, Description: "ui"},
		{ID: "worktree-lite", Path: otherPath, Description: "wt"},
	})
	if len(got) != 1 || got[0].ID != "ui-ux-pro-max" {
		t.Fatalf("unexpected effective skills: %#v", got)
	}
}

func TestEffectiveSkillCatalogEntries_RespectsDisabledBinding(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, "ui-ux-pro-max", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: ui-ux-pro-max\ndescription: ui\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	got := config.EffectiveSkillCatalogEntries(config.Config{SkillBindings: []config.SkillBindingConfig{{ID: "ui-ux-pro-max", Enabled: false}}}, "codex", []string{"ui-ux-pro-max"}, []skillcatalog.Entry{{ID: "ui-ux-pro-max", Path: skillPath, Description: "ui"}})
	if len(got) != 0 {
		t.Fatalf("expected disabled skill to be filtered out: %#v", got)
	}
}
