package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibecraft/backend/internal/config"
	iflowcli "vibecraft/backend/internal/iflow"
)

func TestCLIToolSettings_GetAndPut(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	env := newTestEnv(t, config.Default(), 2)
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}, {ID: "anthropic-default", Provider: "anthropic"}},
		Models: []config.LLMModelConfig{
			{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"},
			{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-sonnet", SourceID: "anthropic-default"},
		},
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
			ID                 string   `json:"id"`
			DefaultModelID     string   `json:"default_model_id"`
			ProtocolFamilies   []string `json:"protocol_families"`
			IFLOWAuthMode      string   `json:"iflow_auth_mode"`
			IFLOWModels        []string `json:"iflow_models"`
			IFLOWDefaultModel  string   `json:"iflow_default_model"`
			IFLOWMaskedKey     string   `json:"iflow_masked_key"`
			IFLOWBrowserAuthed bool     `json:"iflow_browser_authenticated"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Tools) == 0 {
		t.Fatalf("expected cli tools")
	}
	foundIFLOW := false
	foundOpenCode := false
	for _, item := range got.Tools {
		switch item.ID {
		case "iflow":
			foundIFLOW = true
			if item.IFLOWAuthMode != config.IFLOWAuthModeBrowser {
				t.Fatalf("iflow auth mode = %q, want %q", item.IFLOWAuthMode, config.IFLOWAuthModeBrowser)
			}
			if item.IFLOWDefaultModel != iflowcli.DefaultModel {
				t.Fatalf("iflow default model = %q, want %q", item.IFLOWDefaultModel, iflowcli.DefaultModel)
			}
			if len(item.IFLOWModels) == 0 {
				t.Fatalf("expected iflow models in get response")
			}
			if item.IFLOWMaskedKey != "" {
				t.Fatalf("unexpected masked key before save: %q", item.IFLOWMaskedKey)
			}
			if item.IFLOWBrowserAuthed {
				t.Fatalf("expected browser auth false in isolated test env")
			}
		case "opencode":
			foundOpenCode = true
			if len(item.ProtocolFamilies) != 2 || item.ProtocolFamilies[0] != "openai" || item.ProtocolFamilies[1] != "anthropic" {
				t.Fatalf("opencode protocol_families = %#v, want [openai anthropic]", item.ProtocolFamilies)
			}
		}
	}
	if !foundIFLOW {
		t.Fatalf("expected iflow in get response")
	}
	if !foundOpenCode {
		t.Fatalf("expected opencode in get response")
	}

	apiKey := "sk-iflow-123456"
	body, _ := json.Marshal(map[string]any{
		"tools": []map[string]any{
			{"id": "codex", "label": "Codex CLI", "protocol_family": "openai", "protocol_families": []string{"openai"}, "cli_family": "codex", "default_model_id": "gpt-5.4", "enabled": true},
			{"id": "claude", "label": "Claude Code", "protocol_family": "anthropic", "protocol_families": []string{"anthropic"}, "cli_family": "claude", "default_model_id": "", "enabled": true},
			{"id": "iflow", "label": "iFlow CLI", "protocol_family": "openai", "protocol_families": []string{"openai"}, "cli_family": "iflow", "enabled": true, "iflow_auth_mode": "api_key", "iflow_base_url": iflowcli.DefaultBaseURL, "iflow_models": []string{"glm-4.7", "minimax-m2.5"}, "iflow_default_model": "minimax-m2.5", "iflow_api_key": apiKey},
			{"id": "opencode", "label": "OpenCode CLI", "protocol_family": "openai", "protocol_families": []string{"openai", "anthropic"}, "cli_family": "opencode", "default_model_id": "claude-sonnet", "enabled": true},
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
			ID                string   `json:"id"`
			DefaultModelID    string   `json:"default_model_id"`
			ProtocolFamilies  []string `json:"protocol_families"`
			IFLOWAuthMode     string   `json:"iflow_auth_mode"`
			IFLOWModels       []string `json:"iflow_models"`
			IFLOWDefaultModel string   `json:"iflow_default_model"`
			IFLOWHasKey       bool     `json:"iflow_has_key"`
			IFLOWMaskedKey    string   `json:"iflow_masked_key"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(putRes.Body).Decode(&updated); err != nil {
		t.Fatalf("decode put response: %v", err)
	}
	foundIFLOW = false
	foundOpenCode = false
	for _, item := range updated.Tools {
		switch item.ID {
		case "codex":
			if item.DefaultModelID != "gpt-5.4" {
				t.Fatalf("codex default model = %q, want gpt-5.4", item.DefaultModelID)
			}
		case "iflow":
			foundIFLOW = true
			if item.IFLOWAuthMode != config.IFLOWAuthModeAPIKey {
				t.Fatalf("iflow auth mode = %q, want %q", item.IFLOWAuthMode, config.IFLOWAuthModeAPIKey)
			}
			if !item.IFLOWHasKey {
				t.Fatalf("expected iflow_has_key true")
			}
			if item.IFLOWMaskedKey != "****3456" {
				t.Fatalf("iflow masked key = %q, want ****3456", item.IFLOWMaskedKey)
			}
			if item.IFLOWDefaultModel != "minimax-m2.5" {
				t.Fatalf("iflow default model = %q, want minimax-m2.5", item.IFLOWDefaultModel)
			}
			if len(item.IFLOWModels) != 2 {
				t.Fatalf("iflow models len = %d, want 2", len(item.IFLOWModels))
			}
		case "opencode":
			foundOpenCode = true
			if item.DefaultModelID != "claude-sonnet" {
				t.Fatalf("opencode default model = %q, want claude-sonnet", item.DefaultModelID)
			}
			if len(item.ProtocolFamilies) != 2 || item.ProtocolFamilies[0] != "openai" || item.ProtocolFamilies[1] != "anthropic" {
				t.Fatalf("opencode protocol_families = %#v, want [openai anthropic]", item.ProtocolFamilies)
			}
		}
	}
	if !foundIFLOW {
		t.Fatalf("expected iflow in put response")
	}
	if !foundOpenCode {
		t.Fatalf("expected opencode in put response")
	}
}

func TestCLIToolSettings_GetBackfillsBuiltinsFromLegacyConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"}},
	}
	cfg.CLITools = []config.CLIToolConfig{
		{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true},
		{ID: "claude", Label: "Claude Code", ProtocolFamily: "anthropic", CLIFamily: "claude", Enabled: true},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
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
			ID                string `json:"id"`
			IFLOWDefaultModel string `json:"iflow_default_model"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Tools) != 4 {
		t.Fatalf("cli tools len = %d, want 4", len(got.Tools))
	}
	foundIFLOW := false
	foundOpenCode := false
	for _, item := range got.Tools {
		switch item.ID {
		case "iflow":
			foundIFLOW = true
			if item.IFLOWDefaultModel == "" {
				t.Fatalf("expected backfilled iflow default model")
			}
		case "opencode":
			foundOpenCode = true
		}
	}
	if !foundIFLOW {
		t.Fatalf("expected iflow in get response")
	}
	if !foundOpenCode {
		t.Fatalf("expected opencode in get response")
	}
}
