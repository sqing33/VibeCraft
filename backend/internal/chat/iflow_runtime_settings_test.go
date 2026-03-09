package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibe-tree/backend/internal/config"
	iflowcli "vibe-tree/backend/internal/iflow"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

func TestPrepareIFLOWRunSpec_UsesImportedBrowserModelAndRequiresAuth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, ".iflow"), 0o755); err != nil {
		t.Fatalf("mkdir global iflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".iflow", "settings.json"), []byte(`{
  "selectedAuthType": "iflow",
  "modelName": "minimax-m2.5",
  "baseUrl": "https://apis.iflow.cn/v1",
  "apiKey": "sk-iflow-user"
}`), 0o644); err != nil {
		t.Fatalf("write global settings: %v", err)
	}
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{
		ID:                "iflow",
		Label:             "iFlow CLI",
		ProtocolFamily:    "openai",
		CLIFamily:         "iflow",
		Enabled:           true,
		IFlowAuthMode:     config.IFLOWAuthModeBrowser,
		IFlowBaseURL:      iflowcli.DefaultBaseURL,
		IFlowModels:       []string{iflowcli.DefaultModel},
		IFlowDefaultModel: iflowcli.DefaultModel,
	}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	prepared, err := prepareIFLOWRunSpec(store.ChatSession{WorkspacePath: ".", ExpertID: "iflow"}, runner.RunSpec{Env: map[string]string{
		"VIBE_TREE_CLI_FAMILY":  "iflow",
		"VIBE_TREE_CLI_TOOL_ID": "iflow",
		"VIBE_TREE_MODEL":       iflowcli.DefaultModel,
		"VIBE_TREE_MODEL_ID":    iflowcli.DefaultModel,
	}}, "iflow")
	if err != nil {
		t.Fatalf("prepareIFLOWRunSpec: %v", err)
	}
	if got := prepared.Env["VIBE_TREE_MODEL"]; got != "minimax-m2.5" {
		t.Fatalf("VIBE_TREE_MODEL = %q, want minimax-m2.5", got)
	}
	if got := prepared.Env["VIBE_TREE_MODEL_ID"]; got != "minimax-m2.5" {
		t.Fatalf("VIBE_TREE_MODEL_ID = %q, want minimax-m2.5", got)
	}
}

func TestPrepareIFLOWRunSpec_BrowserModeRequiresAuth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{
		ID:                "iflow",
		Label:             "iFlow CLI",
		ProtocolFamily:    "openai",
		CLIFamily:         "iflow",
		Enabled:           true,
		IFlowAuthMode:     config.IFLOWAuthModeBrowser,
		IFlowBaseURL:      iflowcli.DefaultBaseURL,
		IFlowModels:       []string{iflowcli.DefaultModel},
		IFlowDefaultModel: iflowcli.DefaultModel,
	}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	_, err = prepareIFLOWRunSpec(store.ChatSession{WorkspacePath: ".", ExpertID: "iflow"}, runner.RunSpec{Env: map[string]string{
		"VIBE_TREE_CLI_FAMILY":  "iflow",
		"VIBE_TREE_CLI_TOOL_ID": "iflow",
	}}, "iflow")
	if err == nil || !strings.Contains(err.Error(), "网页登录未完成") {
		t.Fatalf("expected missing auth error, got %v", err)
	}
}

func TestPrepareIFLOWRunSpec_InjectsManagedHomeMCPAndSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	skillPath := filepath.Join(home, ".codex", "skills", "ui-ux-pro-max", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("# UI UX\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{
		ID:                "iflow",
		Label:             "iFlow CLI",
		ProtocolFamily:    "openai",
		CLIFamily:         "iflow",
		Enabled:           true,
		IFlowAuthMode:     config.IFLOWAuthModeAPIKey,
		IFlowAPIKey:       "sk-iflow-123",
		IFlowBaseURL:      iflowcli.DefaultBaseURL,
		IFlowModels:       []string{"glm-4.7"},
		IFlowDefaultModel: "glm-4.7",
	}}
	cfg.MCPServers = []config.MCPServerConfig{{
		ID:                       "mcp-router",
		DefaultEnabledCLIToolIDs: []string{"iflow"},
		Config:                   map[string]any{"command": "npx", "args": []any{"-y", "mcp-router"}},
	}}
	cfg.SkillBindings = []config.SkillBindingConfig{{ID: "ui-ux-pro-max", Enabled: true, Path: skillPath, Source: "codex"}}
	for i := range cfg.Experts {
		if strings.TrimSpace(cfg.Experts[i].ID) == "iflow" {
			cfg.Experts[i].EnabledSkills = []string{"ui-ux-pro-max"}
		}
	}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.NormalizeMCPServers(&cfg.MCPServers, cfg.CLITools); err != nil {
		t.Fatalf("normalize mcp: %v", err)
	}
	if err := config.NormalizeSkillBindings(&cfg.SkillBindings, cfg.CLITools); err != nil {
		t.Fatalf("normalize skills: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	prepared, err := prepareIFLOWRunSpec(store.ChatSession{WorkspacePath: ".", ExpertID: "iflow"}, runner.RunSpec{Env: map[string]string{
		"VIBE_TREE_CLI_FAMILY":    "iflow",
		"VIBE_TREE_CLI_TOOL_ID":   "iflow",
		"VIBE_TREE_SYSTEM_PROMPT": "Base instructions",
	}}, "iflow")
	if err != nil {
		t.Fatalf("prepareIFLOWRunSpec: %v", err)
	}
	if got := prepared.Env["VIBE_TREE_IFLOW_AUTH_MODE"]; got != config.IFLOWAuthModeAPIKey {
		t.Fatalf("auth mode = %q, want %q", got, config.IFLOWAuthModeAPIKey)
	}
	if got := prepared.Env["VIBE_TREE_IFLOW_API_KEY"]; got != "sk-iflow-123" {
		t.Fatalf("api key = %q, want sk-iflow-123", got)
	}
	if got := prepared.Env["VIBE_TREE_IFLOW_BASE_URL"]; got != iflowcli.DefaultBaseURL {
		t.Fatalf("base url = %q, want %q", got, iflowcli.DefaultBaseURL)
	}
	if strings.TrimSpace(prepared.Env["VIBE_TREE_IFLOW_HOME"]) == "" {
		t.Fatalf("expected managed iflow home")
	}
	if got := prepared.Env["VIBE_TREE_IFLOW_ALLOWED_MCP_SERVERS"]; got != "mcp-router" {
		t.Fatalf("allowed mcp servers = %q, want mcp-router", got)
	}
	if got := prepared.Env["VIBE_TREE_IFLOW_MCP_SERVERS_JSON"]; !strings.Contains(got, "mcp-router") {
		t.Fatalf("mcp json missing server: %q", got)
	}
	if got := prepared.Env["VIBE_TREE_SYSTEM_PROMPT"]; !strings.Contains(got, "[Enabled Skills]") || !strings.Contains(got, "ui-ux-pro-max") {
		t.Fatalf("system prompt missing skill instructions: %q", got)
	}
}
