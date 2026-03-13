package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"vibecraft/backend/internal/api"
	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/mcpgateway"
)

func TestMCPSettings_GetAndPut(t *testing.T) {
	t.Cleanup(func() {
		api.ValidateMCPServerConfig = mcpgateway.ValidateDownstreamConfig
	})
	api.ValidateMCPServerConfig = func(_ context.Context, _ map[string]any) error { return nil }

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
		"raw_json":                     `{"mcpServers":{"filesystem":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem"]}}}`,
		"default_enabled_cli_tool_ids": []string{"codex"},
	}}, "gateway": map[string]any{"enabled": true, "idle_ttl_seconds": 321}})
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
			RawJSON                  string   `json:"raw_json"`
			DefaultEnabledCLIToolIDs []string `json:"default_enabled_cli_tool_ids"`
		} `json:"servers"`
		Gateway struct {
			Enabled        bool `json:"enabled"`
			IdleTTLSeconds int  `json:"idle_ttl_seconds"`
		} `json:"gateway"`
	}
	if err := json.NewDecoder(putRes.Body).Decode(&updated); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	if len(updated.Servers) != 1 || updated.Servers[0].ID != "filesystem" || len(updated.Servers[0].DefaultEnabledCLIToolIDs) != 1 {
		t.Fatalf("unexpected mcp response: %#v", updated.Servers)
	}
	if !strings.Contains(updated.Servers[0].RawJSON, "filesystem") {
		t.Fatalf("expected normalized raw_json, got %q", updated.Servers[0].RawJSON)
	}
	if !updated.Gateway.Enabled || updated.Gateway.IdleTTLSeconds != 321 {
		t.Fatalf("unexpected gateway response: %#v", updated.Gateway)
	}

	statusRes, err := http.Get(env.httpSrv.URL + "/api/v1/mcp-gateway/status")
	if err != nil {
		t.Fatalf("get gateway status: %v", err)
	}
	defer statusRes.Body.Close()
	if statusRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status endpoint code: %s", statusRes.Status)
	}
}
