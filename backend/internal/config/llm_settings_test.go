package config_test

import (
	"testing"

	"vibecraft/backend/internal/config"
)

func TestValidateLLMSettings_OK(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "openai", BaseURL: "https://api.example.com"},
		},
		Models: []config.LLMModelConfig{
			{ID: "codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
		},
	}

	if err := config.ValidateLLMSettings(llm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLLMSettings_RejectsInvalidProvider(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "s1", Provider: "unknown"},
		},
	}

	if err := config.ValidateLLMSettings(llm); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateLLMSettings_RejectsInvalidURL(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "s1", Provider: "openai", BaseURL: "ftp://example.com"},
		},
	}

	if err := config.ValidateLLMSettings(llm); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateLLMSettings_RejectsUnknownSource(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "s1", Provider: "openai"},
		},
		Models: []config.LLMModelConfig{
			{ID: "m1", Provider: "openai", Model: "gpt", SourceID: "missing"},
		},
	}

	if err := config.ValidateLLMSettings(llm); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateLLMSettings_AllowsSharedSourceAcrossProviders(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "shared", Provider: ""},
		},
		Models: []config.LLMModelConfig{
			{ID: "m1", Provider: "openai", Model: "gpt", SourceID: "shared"},
			{ID: "m2", Provider: "anthropic", Model: "claude", SourceID: "shared"},
		},
	}

	if err := config.ValidateLLMSettings(llm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLLMSettings_RejectsCaseInsensitiveDuplicateModelIDs(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "openai"},
		},
		Models: []config.LLMModelConfig{
			{ID: "GPT-5-CODEX", Provider: "openai", Model: "GPT-5-CODEX", SourceID: "openai-default"},
			{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
		},
	}

	if err := config.ValidateLLMSettings(llm); err == nil {
		t.Fatalf("expected duplicate id error after lowercase normalization")
	}
}

func TestNormalizeLLMSettings_LowercasesModelIdentifiers(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "OpenAI", BaseURL: "https://api.example.com/", APIKey: " sk_test_1234 "},
		},
		Models: []config.LLMModelConfig{
			{ID: "GPT-5-CODEX", Label: "GPT-5-CODEX", Provider: "OpenAI", Model: "GPT-5-CODEX", SourceID: " openai-default "},
		},
	}

	if err := config.NormalizeLLMSettings(llm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := llm.Sources[0].Provider; got != "openai" {
		t.Fatalf("unexpected source provider: %q", got)
	}
	if got := llm.Models[0].ID; got != "gpt-5-codex" {
		t.Fatalf("unexpected model id: %q", got)
	}
	if got := llm.Models[0].Provider; got != "openai" {
		t.Fatalf("unexpected model provider: %q", got)
	}
	if got := llm.Models[0].Model; got != "gpt-5-codex" {
		t.Fatalf("unexpected model name: %q", got)
	}
	if got := llm.Models[0].SourceID; got != "openai-default" {
		t.Fatalf("unexpected source id: %q", got)
	}
	if got := llm.Models[0].Label; got != "GPT-5-CODEX" {
		t.Fatalf("unexpected label: %q", got)
	}
}
