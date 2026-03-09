package config_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestValidateBasicSettings_OK(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "openai", Model: "gpt-4.1-mini", SourceID: "openai-default"},
			{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{
			ModelID:        "translator-fast",
			TargetModelIDs: []string{"GPT-5-CODEX"},
		},
	}

	config.NormalizeBasicSettings(&basic)
	if err := config.ValidateBasicSettings(basic, llm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBasicSettings_RejectsUnknownTargetModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "openai", Model: "gpt-4.1-mini", SourceID: "openai-default"},
			{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{
			ModelID:        "translator-fast",
			TargetModelIDs: []string{"missing-model"},
		},
	}

	if err := config.ValidateBasicSettings(basic, llm); err == nil {
		t.Fatalf("expected error")
	}
}

func TestReconcileBasicSettingsWithLLM_TrimsRemovedModelsAndClearsMissingModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "openai", Model: "gpt-4.1-mini", SourceID: "openai-default"},
			{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{
			ModelID:        "translator-fast",
			TargetModelIDs: []string{"gpt-5-codex", "claude-3-7-sonnet"},
		},
	}

	config.ReconcileBasicSettingsWithLLM(&basic, llm)
	if basic == nil || basic.ThinkingTranslation == nil {
		t.Fatalf("expected thinking translation to remain")
	}
	if got := basic.ThinkingTranslation.TargetModelIDs; len(got) != 1 || got[0] != "gpt-5-codex" {
		t.Fatalf("unexpected target_model_ids: %#v", got)
	}

	basic.ThinkingTranslation.ModelID = "missing-model"
	config.ReconcileBasicSettingsWithLLM(&basic, llm)
	if basic != nil {
		t.Fatalf("expected basic settings to be cleared when translation model is missing")
	}
}

func TestResolveThinkingTranslation_MatchesTargetModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "anthropic-default", Provider: "anthropic", BaseURL: "https://anthropic.example.com", APIKey: "sk-ant-123"}},
		Models: []config.LLMModelConfig{
			{ID: "translator-fast", Provider: "anthropic", Model: "claude-3-5-haiku", SourceID: "anthropic-default"},
			{ID: "claude-3-7-sonnet", Provider: "anthropic", Model: "claude-3-7-sonnet", SourceID: "anthropic-default"},
		},
	}
	basic := &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{
			ModelID:        "translator-fast",
			TargetModelIDs: []string{"claude-3-7-sonnet"},
		},
	}

	runtime, err := config.ResolveThinkingTranslation(basic, llm, "claude-3-7-sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime == nil {
		t.Fatalf("expected runtime config")
	}
	if runtime.Provider != "anthropic" || runtime.Model != "claude-3-5-haiku" {
		t.Fatalf("unexpected runtime: %+v", runtime)
	}
	miss, err := config.ResolveThinkingTranslation(basic, llm, "other-model")
	if err != nil {
		t.Fatalf("unexpected miss error: %v", err)
	}
	if miss != nil {
		t.Fatalf("expected nil runtime for unmatched model")
	}
}
