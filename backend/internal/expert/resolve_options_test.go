package expert_test

import (
	"testing"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/expert"
	iflowcli "vibecraft/backend/internal/iflow"
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

func TestResolveWithOptions_AllowsMultiProtocolCLIToolModelSwitch(t *testing.T) {
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "openai"},
			{ID: "anthropic-default", Provider: "anthropic"},
		},
		Models: []config.LLMModelConfig{
			{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"},
			{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-sonnet", SourceID: "anthropic-default"},
		},
	}
	cfg.CLITools = []config.CLIToolConfig{{
		ID:               "codex",
		Label:            "Codex CLI",
		ProtocolFamily:   "openai",
		ProtocolFamilies: []string{"openai", "anthropic"},
		CLIFamily:        "codex",
		DefaultModelID:   "claude-sonnet",
		Enabled:          true,
	}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	res, err := expert.NewRegistry(cfg).ResolveWithOptions("codex", "hello", ".", expert.ResolveOptions{CLIToolID: "codex", ModelID: "gpt-5.4"})
	if err != nil {
		t.Fatalf("resolve with options: %v", err)
	}
	if got := res.Model; got != "gpt-5.4" {
		t.Fatalf("model = %q, want gpt-5.4", got)
	}
	if got := res.ProtocolFamily; got != "openai" {
		t.Fatalf("protocol family = %q, want openai", got)
	}
}

func TestResolveWithOptions_IFLOWUsesOfficialModelSelection(t *testing.T) {
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{
		ID:                "iflow",
		Label:             "iFlow CLI",
		ProtocolFamily:    "openai",
		CLIFamily:         "iflow",
		Enabled:           true,
		IFlowAuthMode:     config.IFLOWAuthModeAPIKey,
		IFlowAPIKey:       "sk-iflow-123",
		IFlowBaseURL:      iflowcli.DefaultBaseURL,
		IFlowModels:       []string{"glm-4.7", "minimax-m2.5"},
		IFlowDefaultModel: "minimax-m2.5",
	}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	res, err := expert.NewRegistry(cfg).ResolveWithOptions("iflow", "hello", ".", expert.ResolveOptions{CLIToolID: "iflow"})
	if err != nil {
		t.Fatalf("resolve with options: %v", err)
	}
	if got := res.Model; got != "minimax-m2.5" {
		t.Fatalf("model = %q, want minimax-m2.5", got)
	}
	if got := res.Spec.Env["OPENAI_API_KEY"]; got != "" {
		t.Fatalf("OPENAI_API_KEY = %q, want empty", got)
	}
	if got := res.Spec.Env["OPENAI_BASE_URL"]; got != "" {
		t.Fatalf("OPENAI_BASE_URL = %q, want empty", got)
	}
	if got := res.Spec.Env["VIBECRAFT_MODEL_ID"]; got != "minimax-m2.5" {
		t.Fatalf("VIBECRAFT_MODEL_ID = %q, want minimax-m2.5", got)
	}
}

func TestResolveWithOptions_OpenCodeSupportsAnthropicModel(t *testing.T) {
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "anthropic-default", Provider: "anthropic", BaseURL: "https://anthropic.example.com", APIKey: "sk-ant-123"}},
		Models:  []config.LLMModelConfig{{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-3-7-sonnet", SourceID: "anthropic-default"}},
	}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	res, err := expert.NewRegistry(cfg).ResolveWithOptions("opencode", "hello", ".", expert.ResolveOptions{CLIToolID: "opencode", ModelID: "claude-sonnet"})
	if err != nil {
		t.Fatalf("resolve with options: %v", err)
	}
	if res.ProtocolFamily != "anthropic" {
		t.Fatalf("protocol family = %q, want anthropic", res.ProtocolFamily)
	}
	if got := res.Spec.Env["ANTHROPIC_API_KEY"]; got != "sk-ant-123" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want sk-ant-123", got)
	}
	if got := res.Spec.Env["ANTHROPIC_BASE_URL"]; got != "https://anthropic.example.com" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q, want https://anthropic.example.com", got)
	}
	if got := res.Spec.Env["VIBECRAFT_PROTOCOL_FAMILY"]; got != "anthropic" {
		t.Fatalf("VIBECRAFT_PROTOCOL_FAMILY = %q, want anthropic", got)
	}
	if got := res.Model; got != "claude-3-7-sonnet" {
		t.Fatalf("model = %q, want claude-3-7-sonnet", got)
	}
}
