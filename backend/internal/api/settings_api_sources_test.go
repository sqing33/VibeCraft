package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestAPISourceSettings_PutRebindsMissingRuntimeSourcesToRemainingSource(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.APISources = []config.APISourceConfig{
		{ID: "openai-default", Label: "OpenAI 官方", BaseURL: "https://api.openai.com/v1", APIKey: "sk-openai"},
		{ID: "anthropic-default", Label: "Anthropic 官方", BaseURL: "https://api.anthropic.com", APIKey: "sk-anthropic"},
	}
	cfg.RuntimeModels = &config.RuntimeModelSettings{
		Runtimes: []config.RuntimeModelRuntimeConfig{
			{
				ID:             config.RuntimeIDSDKOpenAI,
				DefaultModelID: "gpt-5-codex",
				Models: []config.RuntimeModelConfig{{
					ID:       "gpt-5-codex",
					Label:    "gpt-5-codex",
					Provider: "openai",
					Model:    "gpt-5-codex",
					SourceID: "openai-default",
				}},
			},
			{
				ID:             config.RuntimeIDSDKAnthropic,
				DefaultModelID: "claude-3-7-sonnet",
				Models: []config.RuntimeModelConfig{{
					ID:       "claude-3-7-sonnet",
					Label:    "claude-3-7-sonnet",
					Provider: "anthropic",
					Model:    "claude-3-7-sonnet",
					SourceID: "anthropic-default",
				}},
			},
		},
	}
	if err := config.NormalizeAPISources(&cfg.APISources); err != nil {
		t.Fatalf("normalize sources: %v", err)
	}
	if err := config.NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
		t.Fatalf("normalize runtime models: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body := []byte(`{"sources":[{"label":"Shared Gateway","base_url":"https://gateway.example.com/v1"}]}`)
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/api-sources", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put api sources: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: %s body=%s", res.Status, strings.TrimSpace(string(payload)))
	}

	var out struct {
		Sources []struct {
			ID string `json:"id"`
		} `json:"sources"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Sources) != 1 {
		t.Fatalf("response sources len = %d, want 1", len(out.Sources))
	}
	if out.Sources[0].ID != "shared-gateway" {
		t.Fatalf("response source id = %q, want shared-gateway", out.Sources[0].ID)
	}

	persisted, _, err := config.LoadPersisted()
	if err != nil {
		t.Fatalf("load persisted: %v", err)
	}
	openaiRuntime, ok := config.FindRuntimeConfigByID(persisted.RuntimeModels, config.RuntimeIDSDKOpenAI)
	if !ok || len(openaiRuntime.Models) != 1 {
		t.Fatalf("expected openai runtime model to exist")
	}
	if openaiRuntime.Models[0].SourceID != "shared-gateway" {
		t.Fatalf("openai runtime source_id = %q, want shared-gateway", openaiRuntime.Models[0].SourceID)
	}
	anthropicRuntime, ok := config.FindRuntimeConfigByID(persisted.RuntimeModels, config.RuntimeIDSDKAnthropic)
	if !ok || len(anthropicRuntime.Models) != 1 {
		t.Fatalf("expected anthropic runtime model to exist")
	}
	if anthropicRuntime.Models[0].SourceID != "shared-gateway" {
		t.Fatalf("anthropic runtime source_id = %q, want shared-gateway", anthropicRuntime.Models[0].SourceID)
	}
}
