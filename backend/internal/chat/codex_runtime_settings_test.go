package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/store"
)

func TestResolveCodexRuntimeSettings_InjectsSelectedMCPsAndSkills(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	skillPath := filepath.Join(home, ".codex", "skills", "ui-ux-pro-max", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: ui-ux-pro-max\ndescription: ui\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{ID: "codex", Label: "Codex", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true}}
	cfg.Experts = append(cfg.Experts, config.ExpertConfig{ID: "ui-expert", Label: "UI", Provider: "cli", CLIFamily: "codex", Model: "gpt-5-codex", EnabledSkills: []string{"ui-ux-pro-max"}, Env: map[string]string{}, TimeoutMs: 30000})
	cfg.MCPServers = []config.MCPServerConfig{{ID: "filesystem", DefaultEnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "npx", "args": []any{"-y", "@modelcontextprotocol/server-filesystem"}}}}
	cfg.SkillBindings = []config.SkillBindingConfig{{ID: "ui-ux-pro-max", Enabled: true, Path: skillPath, Source: "codex"}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.NormalizeMCPServers(&cfg.MCPServers, cfg.CLITools); err != nil {
		t.Fatalf("normalize mcp: %v", err)
	}
	if err := config.NormalizeSkillBindings(&cfg.SkillBindings, cfg.CLITools); err != nil {
		t.Fatalf("normalize skill bindings: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	sess := store.ChatSession{WorkspacePath: ".", MCPServerIDs: []string{"filesystem"}}
	spec := runner.RunSpec{Cwd: ".", Env: map[string]string{"VIBECRAFT_SYSTEM_PROMPT": "You are Codex.", "VIBECRAFT_CLI_TOOL_ID": "codex"}}
	runtime, err := resolveCodexRuntimeSettings(sess, spec, "ui-expert", "codex")
	if err != nil {
		t.Fatalf("resolve runtime settings: %v", err)
	}
	mcp, ok := runtime.Config["mcp_servers"].(map[string]map[string]any)
	if !ok {
		t.Fatalf("unexpected mcp config type: %#v", runtime.Config["mcp_servers"])
	}
	if len(mcp) != 1 || mcp["filesystem"] == nil {
		t.Fatalf("unexpected mcp config: %#v", mcp)
	}
	if !strings.Contains(runtime.BaseInstructions, "[Enabled Skills]") || !strings.Contains(runtime.BaseInstructions, "ui-ux-pro-max") || !strings.Contains(runtime.BaseInstructions, "path=") {
		t.Fatalf("unexpected base instructions: %s", runtime.BaseInstructions)
	}
}

func TestResolveCodexRuntimeSettings_ExcludesDisabledSkills(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	skillPath := filepath.Join(home, ".codex", "skills", "ui-ux-pro-max", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: ui-ux-pro-max\ndescription: ui\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{ID: "codex", Label: "Codex", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true}}
	cfg.Experts = append(cfg.Experts, config.ExpertConfig{ID: "ui-expert", Label: "UI", Provider: "cli", CLIFamily: "codex", Model: "gpt-5-codex", EnabledSkills: []string{"ui-ux-pro-max"}, Env: map[string]string{}, TimeoutMs: 30000})
	cfg.SkillBindings = []config.SkillBindingConfig{{ID: "ui-ux-pro-max", Enabled: false, Path: skillPath, Source: "codex"}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.NormalizeSkillBindings(&cfg.SkillBindings, cfg.CLITools); err != nil {
		t.Fatalf("normalize skill bindings: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	runtime, err := resolveCodexRuntimeSettings(store.ChatSession{WorkspacePath: "."}, runner.RunSpec{Cwd: ".", Env: map[string]string{"VIBECRAFT_SYSTEM_PROMPT": "You are Codex."}}, "ui-expert", "codex")
	if err != nil {
		t.Fatalf("resolve runtime settings: %v", err)
	}
	if strings.Contains(runtime.BaseInstructions, "ui-ux-pro-max") {
		t.Fatalf("expected disabled skill to be excluded: %s", runtime.BaseInstructions)
	}
}
