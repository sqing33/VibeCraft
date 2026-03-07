package expert_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/expert"
)

func TestResolveWithOptions_UsesToolDefaultModel(t *testing.T) {
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"}},
	}
	cfg.CLITools = []config.CLIToolConfig{{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", DefaultModelID: "gpt-5.4", Enabled: true}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	res, err := expert.NewRegistry(cfg).ResolveWithOptions("codex", "hello", ".", expert.ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve with options: %v", err)
	}
	if res.Model != "gpt-5.4" {
		t.Fatalf("model = %q, want gpt-5.4", res.Model)
	}
	if res.ProtocolFamily != "openai" {
		t.Fatalf("protocol family = %q, want openai", res.ProtocolFamily)
	}
}
