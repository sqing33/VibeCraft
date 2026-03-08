package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/store"
)

func TestChatSession_CreateUsesDefaultMCPSelectionAndPatchPersists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{ID: "codex", Label: "Codex", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true}}
	cfg.MCPServers = []config.MCPServerConfig{{ID: "filesystem", DefaultEnabledCLIToolIDs: []string{"codex"}, Config: map[string]any{"command": "npx"}}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.NormalizeMCPServers(&cfg.MCPServers, cfg.CLITools); err != nil {
		t.Fatalf("normalize mcp servers: %v", err)
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

	createBody, _ := json.Marshal(map[string]any{"title": "chat", "expert_id": "codex", "cli_tool_id": "codex", "workspace_path": "."})
	createRes, err := http.Post(env.httpSrv.URL+"/api/v1/chat/sessions", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer createRes.Body.Close()
	if createRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create status: %s", createRes.Status)
	}
	var sess store.ChatSession
	if err := json.NewDecoder(createRes.Body).Decode(&sess); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if len(sess.MCPServerIDs) != 1 || sess.MCPServerIDs[0] != "filesystem" {
		t.Fatalf("unexpected default mcp selection: %#v", sess.MCPServerIDs)
	}

	patchBody, _ := json.Marshal(map[string]any{"mcp_server_ids": []string{}})
	req, err := http.NewRequest(http.MethodPatch, env.httpSrv.URL+"/api/v1/chat/sessions/"+sess.ID, bytes.NewReader(patchBody))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	patchRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch session: %v", err)
	}
	defer patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch status: %s", patchRes.Status)
	}
	var updated store.ChatSession
	if err := json.NewDecoder(patchRes.Body).Decode(&updated); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if len(updated.MCPServerIDs) != 0 {
		t.Fatalf("expected explicit empty selection, got %#v", updated.MCPServerIDs)
	}
}
