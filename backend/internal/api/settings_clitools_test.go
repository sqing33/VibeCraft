package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestCLIToolSettings_GetAndPut(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"}},
	}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	res, err := http.Get(env.httpSrv.URL + "/api/v1/settings/cli-tools")
	if err != nil {
		t.Fatalf("get cli tools: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var got struct {
		Tools []struct {
			ID             string `json:"id"`
			DefaultModelID string `json:"default_model_id"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Tools) == 0 {
		t.Fatalf("expected cli tools")
	}

	body, _ := json.Marshal(map[string]any{
		"tools": []map[string]any{
			{"id": "codex", "label": "Codex CLI", "protocol_family": "openai", "cli_family": "codex", "default_model_id": "gpt-5.4", "enabled": true},
			{"id": "claude", "label": "Claude Code", "protocol_family": "anthropic", "cli_family": "claude", "default_model_id": "", "enabled": true},
		},
	})
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/cli-tools", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new put request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	putRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put cli tools: %v", err)
	}
	defer putRes.Body.Close()
	if putRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected put status: %s", putRes.Status)
	}
	var updated struct {
		Tools []struct {
			ID             string `json:"id"`
			DefaultModelID string `json:"default_model_id"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(putRes.Body).Decode(&updated); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	for _, item := range updated.Tools {
		if item.ID == "codex" && item.DefaultModelID != "gpt-5.4" {
			t.Fatalf("codex default model = %q, want gpt-5.4", item.DefaultModelID)
		}
	}
}
