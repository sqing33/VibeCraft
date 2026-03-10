package config_test

import (
	"strings"
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestHydrateRuntimeSettings_DerivesRuntimeModelsFromLegacyConfig(t *testing.T) {
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{
			ID:       "openai-default",
			Label:    "OpenAI 官方",
			Provider: "openai",
			BaseURL:  "https://api.openai.com/v1",
			APIKey:   "sk-test",
		}},
		Models: []config.LLMModelConfig{{
			ID:       "gpt-5-codex",
			Label:    "GPT-5 Codex",
			Provider: "openai",
			Model:    "gpt-5-codex",
			SourceID: "openai-default",
		}},
	}

	if err := config.HydrateRuntimeSettings(&cfg); err != nil {
		t.Fatalf("hydrate runtime settings: %v", err)
	}
	if len(cfg.APISources) == 0 {
		t.Fatalf("expected api_sources to be derived from legacy llm settings")
	}
	if cfg.RuntimeModels == nil {
		t.Fatalf("expected runtime_model_settings to be populated")
	}

	codexRuntime, ok := config.FindRuntimeConfigByID(cfg.RuntimeModels, config.RuntimeIDCodex)
	if !ok {
		t.Fatalf("expected codex runtime to exist")
	}
	if len(codexRuntime.Models) == 0 {
		t.Fatalf("expected codex runtime to inherit legacy openai models")
	}
	if codexRuntime.DefaultModelID != "gpt-5-codex" {
		t.Fatalf("codex default_model_id = %q, want gpt-5-codex", codexRuntime.DefaultModelID)
	}

	resolved, ok := config.ResolveRuntimeModelBinding(cfg.RuntimeModels, cfg.APISources, config.RuntimeIDCodex, "gpt-5-codex")
	if !ok {
		t.Fatalf("expected runtime model binding to resolve")
	}
	if resolved.Source.ID != "openai-default" {
		t.Fatalf("resolved source id = %q, want openai-default", resolved.Source.ID)
	}
}

func TestNormalizeRuntimeModelSettings_DerivesProviderModelAndLabelFromSimplifiedPayload(t *testing.T) {
	settings := &config.RuntimeModelSettings{
		Runtimes: []config.RuntimeModelRuntimeConfig{{
			ID: config.RuntimeIDCodex,
			Models: []config.RuntimeModelConfig{{
				ID:       "GPT-5-CODEX",
				SourceID: "shared-gateway",
			}},
		}},
	}
	sources := []config.APISourceConfig{{
		ID:      "shared-gateway",
		Label:   "共享网关",
		BaseURL: "https://api.openai.com/v1",
	}}

	if err := config.NormalizeRuntimeModelSettings(&settings, sources); err != nil {
		t.Fatalf("normalize runtime settings: %v", err)
	}

	runtime, ok := config.FindRuntimeConfigByID(settings, config.RuntimeIDCodex)
	if !ok {
		t.Fatalf("expected codex runtime to exist")
	}
	if len(runtime.Models) != 1 {
		t.Fatalf("runtime models len = %d, want 1", len(runtime.Models))
	}
	model := runtime.Models[0]
	if model.ID != "gpt-5-codex" {
		t.Fatalf("model id = %q, want gpt-5-codex", model.ID)
	}
	if model.Label != "gpt-5-codex" {
		t.Fatalf("model label = %q, want gpt-5-codex", model.Label)
	}
	if model.Model != "gpt-5-codex" {
		t.Fatalf("model model = %q, want gpt-5-codex", model.Model)
	}
	if model.Provider != "openai" {
		t.Fatalf("model provider = %q, want openai", model.Provider)
	}
	if runtime.DefaultModelID != "gpt-5-codex" {
		t.Fatalf("default_model_id = %q, want gpt-5-codex", runtime.DefaultModelID)
	}
}

func TestNormalizeRuntimeModelSettings_RejectsExplicitRuntimeProviderMismatch(t *testing.T) {
	settings := &config.RuntimeModelSettings{
		Runtimes: []config.RuntimeModelRuntimeConfig{{
			ID: config.RuntimeIDClaude,
			Models: []config.RuntimeModelConfig{{
				ID:       "claude-3-7-sonnet",
				Provider: "openai",
				SourceID: "shared-gateway",
			}},
		}},
	}
	sources := []config.APISourceConfig{{
		ID:      "shared-gateway",
		BaseURL: "https://gateway.example.com/v1",
	}}

	err := config.NormalizeRuntimeModelSettings(&settings, sources)
	if err == nil {
		t.Fatalf("expected incompatible provider to fail")
	}
	if !strings.Contains(err.Error(), "is not allowed for runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeAPISources_GeneratesIDFromLabelAndKeepsExistingID(t *testing.T) {
	sources := []config.APISourceConfig{
		{Label: "Shared Gateway", BaseURL: "https://api.example.com/v1"},
		{Label: "Shared Gateway", BaseURL: "https://api2.example.com/v1"},
		{ID: "custom-source", Label: "Renamed Source", BaseURL: "https://api3.example.com/v1"},
	}

	if err := config.NormalizeAPISources(&sources); err != nil {
		t.Fatalf("normalize api sources: %v", err)
	}
	if len(sources) != 3 {
		t.Fatalf("sources len = %d, want 3", len(sources))
	}
	if sources[0].ID != "shared-gateway" {
		t.Fatalf("sources[0].id = %q, want shared-gateway", sources[0].ID)
	}
	if sources[1].ID != "shared-gateway-2" {
		t.Fatalf("sources[1].id = %q, want shared-gateway-2", sources[1].ID)
	}
	if sources[2].ID != "custom-source" {
		t.Fatalf("sources[2].id = %q, want custom-source", sources[2].ID)
	}
}
