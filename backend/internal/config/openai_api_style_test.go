package config_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestPreserveOpenAIAPIStyles_PreservesWhenModelAndSourceUnchanged(t *testing.T) {
	existing := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", BaseURL: "https://proxy.example.com", APIKey: "sk-old"}},
		Models:  []config.LLMModelConfig{{ID: "model-a", Provider: "openai", Model: "gpt-4.1", SourceID: "openai-default", OpenAIAPIStyle: config.OpenAIAPIStyleChatCompletions, OpenAIAPIStyleDetectedAt: 123}},
	}
	next := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", BaseURL: "https://proxy.example.com", APIKey: "sk-old"}},
		Models:  []config.LLMModelConfig{{ID: "model-a", Provider: "openai", Model: "gpt-4.1", SourceID: "openai-default"}},
	}

	config.PreserveOpenAIAPIStyles(existing, next)
	if next.Models[0].OpenAIAPIStyle != config.OpenAIAPIStyleChatCompletions {
		t.Fatalf("expected style to be preserved, got %q", next.Models[0].OpenAIAPIStyle)
	}
	if next.Models[0].OpenAIAPIStyleDetectedAt != 123 {
		t.Fatalf("expected detected_at to be preserved, got %d", next.Models[0].OpenAIAPIStyleDetectedAt)
	}
}

func TestPreserveOpenAIAPIStyles_ClearsWhenSourceChanges(t *testing.T) {
	existing := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", BaseURL: "https://proxy.example.com", APIKey: "sk-old"}},
		Models:  []config.LLMModelConfig{{ID: "model-a", Provider: "openai", Model: "gpt-4.1", SourceID: "openai-default", OpenAIAPIStyle: config.OpenAIAPIStyleResponses, OpenAIAPIStyleDetectedAt: 123}},
	}
	next := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", BaseURL: "https://new-proxy.example.com", APIKey: "sk-old"}},
		Models:  []config.LLMModelConfig{{ID: "model-a", Provider: "openai", Model: "gpt-4.1", SourceID: "openai-default"}},
	}

	config.PreserveOpenAIAPIStyles(existing, next)
	if next.Models[0].OpenAIAPIStyle != "" {
		t.Fatalf("expected style to be cleared, got %q", next.Models[0].OpenAIAPIStyle)
	}
	if next.Models[0].OpenAIAPIStyleDetectedAt != 0 {
		t.Fatalf("expected detected_at to be cleared, got %d", next.Models[0].OpenAIAPIStyleDetectedAt)
	}
}
