package config_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
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
