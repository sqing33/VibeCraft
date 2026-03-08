package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestMCPSettings_GetAndPut(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
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

	res, err := http.Get(env.httpSrv.URL + "/api/v1/settings/mcp")
	if err != nil {
		t.Fatalf("get mcp settings: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}

	body, _ := json.Marshal(map[string]any{"servers": []map[string]any{{
		"id":                           "filesystem",
		"label":                        "Filesystem",
		"enabled":                      true,
		"enabled_cli_tool_ids":         []string{"codex"},
		"default_enabled_cli_tool_ids": []string{"codex"},
		"config":                       map[string]any{"command": "npx", "args": []string{"-y", "@modelcontextprotocol/server-filesystem"}},
	}}})
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/mcp", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new put request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	putRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put mcp settings: %v", err)
	}
	defer putRes.Body.Close()
	if putRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected put status: %s", putRes.Status)
	}
	var updated struct {
		Servers []struct {
			ID                       string   `json:"id"`
			DefaultEnabledCLIToolIDs []string `json:"default_enabled_cli_tool_ids"`
		} `json:"servers"`
	}
	if err := json.NewDecoder(putRes.Body).Decode(&updated); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if len(updated.Servers) != 1 || updated.Servers[0].ID != "filesystem" || len(updated.Servers[0].DefaultEnabledCLIToolIDs) != 1 {
		t.Fatalf("unexpected mcp response: %#v", updated.Servers)
	}
}
