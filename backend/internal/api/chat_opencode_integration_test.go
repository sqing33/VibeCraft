package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/store"
)

func TestChatSession_CreateBackfillsBuiltinOpenCodeExpertFromLegacyConfig(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{
		{ID: "codex", Label: "Codex", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true},
		{ID: "claude", Label: "Claude", ProtocolFamily: "anthropic", CLIFamily: "claude", Enabled: true},
	}
	cfg.Experts = []config.ExpertConfig{
		{ID: "master", Label: "Master", Provider: "openai", Model: "gpt-5-codex", ManagedSource: config.ManagedSourceBuiltin},
		{ID: "codex", Label: "Codex", Provider: "cli", RuntimeKind: "cli", CLIFamily: "codex", Model: "gpt-5-codex", ManagedSource: config.ManagedSourceBuiltin, Env: map[string]string{}},
		{ID: "claudecode", Label: "ClaudeCode", Provider: "cli", RuntimeKind: "cli", CLIFamily: "claude", Model: "claude-3-7-sonnet-latest", ManagedSource: config.ManagedSourceBuiltin, Env: map[string]string{}},
	}
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
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat",
		"expert_id":      "opencode",
		"cli_tool_id":    "opencode",
		"workspace_path": ".",
	})
	createRes, err := http.Post(env.httpSrv.URL+"/api/v1/chat/sessions", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer createRes.Body.Close()
	if createRes.StatusCode != http.StatusOK {
		var body map[string]any
		_ = json.NewDecoder(createRes.Body).Decode(&body)
		t.Fatalf("unexpected create status: %s body=%v", createRes.Status, body)
	}
	var sess store.ChatSession
	if err := json.NewDecoder(createRes.Body).Decode(&sess); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if sess.ExpertID != "opencode" {
		t.Fatalf("expert_id = %q, want opencode", sess.ExpertID)
	}
	if sess.CLIToolID == nil || *sess.CLIToolID != "opencode" {
		t.Fatalf("cli_tool_id = %#v, want opencode", sess.CLIToolID)
	}
}
