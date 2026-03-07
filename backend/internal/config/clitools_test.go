package config_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestNormalizeCLITools_RejectsProtocolMismatchDefaultModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "anthropic-default", Provider: "anthropic"}},
		Models:  []config.LLMModelConfig{{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-sonnet", SourceID: "anthropic-default"}},
	}
	tools := []config.CLIToolConfig{{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", DefaultModelID: "claude-sonnet", Enabled: true}}
	if err := config.NormalizeCLITools(&tools, llm); err == nil {
		t.Fatalf("expected protocol mismatch error")
	}
}

func TestNormalizeCLITools_AllowsMatchingDefaultModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"}},
	}
	tools := []config.CLIToolConfig{{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", DefaultModelID: "gpt-5.4", Enabled: true}}
	if err := config.NormalizeCLITools(&tools, llm); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
}
