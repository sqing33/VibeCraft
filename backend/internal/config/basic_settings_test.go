package config_test

import (
	"testing"

	"vibecraft/backend/internal/config"
)

func TestNormalizeBasicSettings_ClearsLegacyFields(t *testing.T) {
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{
			SourceID:       "legacy-source",
			Model:          "translator-fast",
			TargetModelIDs: []string{"gpt-5-codex"},
		},
	}

	config.NormalizeBasicSettings(&basic)
	if basic == nil || basic.ThinkingTranslation == nil {
		t.Fatalf("expected thinking translation to remain")
	}
	if basic.ThinkingTranslation.ModelID != "translator-fast" {
		t.Fatalf("unexpected model_id: %q", basic.ThinkingTranslation.ModelID)
	}
	if basic.ThinkingTranslation.SourceID != "" || basic.ThinkingTranslation.Model != "" {
		t.Fatalf("expected legacy fields to be cleared: %+v", basic.ThinkingTranslation)
	}
	if len(basic.ThinkingTranslation.TargetModelIDs) != 0 {
		t.Fatalf("expected target_model_ids to be cleared: %#v", basic.ThinkingTranslation.TargetModelIDs)
	}
}

func TestValidateBasicSettings_OK(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "openai", Model: "gpt-4.1-mini", SourceID: "openai-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{ModelID: "translator-fast"},
	}

	config.NormalizeBasicSettings(&basic)
	if err := config.ValidateBasicSettings(basic, llm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBasicSettings_RejectsUnknownModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "openai", Model: "gpt-4.1-mini", SourceID: "openai-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{ModelID: "missing-model"},
	}

	if err := config.ValidateBasicSettings(basic, llm); err == nil {
		t.Fatalf("expected error")
	}
}

func TestReconcileBasicSettingsWithLLM_ClearsMissingModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "openai", Model: "gpt-4.1-mini", SourceID: "openai-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{ModelID: "translator-fast"},
	}

	config.ReconcileBasicSettingsWithLLM(&basic, llm)
	if basic == nil || basic.ThinkingTranslation == nil {
		t.Fatalf("expected thinking translation to remain")
	}

	basic.ThinkingTranslation.ModelID = "missing-model"
	config.ReconcileBasicSettingsWithLLM(&basic, llm)
	if basic != nil {
		t.Fatalf("expected basic settings to be cleared when translation model is missing")
	}
}

func TestResolveThinkingTranslation_ReturnsConfiguredRuntime(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "anthropic-default", Provider: "anthropic", BaseURL: "https://anthropic.example.com", APIKey: "sk-ant-123"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "anthropic", Model: "claude-3-5-haiku", SourceID: "anthropic-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{ModelID: "translator-fast"},
	}

	runtime, err := config.ResolveThinkingTranslation(basic, llm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime == nil {
		t.Fatalf("expected runtime config")
	}
	if runtime.Provider != "anthropic" || runtime.Model != "claude-3-5-haiku" {
		t.Fatalf("unexpected runtime: %+v", runtime)
	}
}

func TestValidateBasicSettingsWithRuntime_UsesModelProviderInsteadOfSourceProvider(t *testing.T) {
	cfg := config.Default()
	cfg.APISources = []config.APISourceConfig{{
		ID:      "shared-gateway",
		Label:   "共享来源",
		BaseURL: "https://gateway.example.com/v1",
	}}
	cfg.RuntimeModels = &config.RuntimeModelSettings{
		Runtimes: []config.RuntimeModelRuntimeConfig{{
			ID: config.RuntimeIDSDKAnthropic,
			Models: []config.RuntimeModelConfig{{
				ID:       "translator-fast",
				Provider: "anthropic",
				Model:    "claude-3-5-haiku",
				SourceID: "shared-gateway",
			}},
			DefaultModelID: "translator-fast",
		}},
	}
	if err := config.NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
		t.Fatalf("normalize runtime settings: %v", err)
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{ModelID: "translator-fast"},
	}
	if err := config.ValidateBasicSettingsWithRuntime(basic, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
