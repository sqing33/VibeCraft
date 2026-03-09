package config_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
	iflowcli "vibe-tree/backend/internal/iflow"
)

func TestNormalizeCLITools_RejectsIncompatibleDefaultModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "anthropic-default", Provider: "anthropic"}},
		Models:  []config.LLMModelConfig{{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-sonnet", SourceID: "anthropic-default"}},
	}
	tools := []config.CLIToolConfig{{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", DefaultModelID: "claude-sonnet", Enabled: true}}
	if err := config.NormalizeCLITools(&tools, llm); err == nil {
		t.Fatalf("expected incompatible model error")
	}
}

func TestNormalizeCLITools_AcceptsMatchingDefaultModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"}},
	}
	tools := []config.CLIToolConfig{{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", DefaultModelID: "gpt-5.4", Enabled: true}}
	if err := config.NormalizeCLITools(&tools, llm); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
}

func TestNormalizeCLITools_AcceptsLegacyProtocolFamilies(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "openai"},
			{ID: "anthropic-default", Provider: "anthropic"},
		},
		Models: []config.LLMModelConfig{
			{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"},
			{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-sonnet", SourceID: "anthropic-default"},
		},
	}
	tools := []config.CLIToolConfig{{
		ID:               "opencode",
		Label:            "OpenCode CLI",
		ProtocolFamily:   "openai",
		ProtocolFamilies: []string{"openai", "anthropic"},
		CLIFamily:        "opencode",
		DefaultModelID:   "claude-sonnet",
		Enabled:          true,
	}}
	if err := config.NormalizeCLITools(&tools, llm); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if got := tools[0].ProtocolFamily; got != "anthropic" {
		t.Fatalf("protocol family = %q, want anthropic", got)
	}
	if len(tools[0].ProtocolFamilies) != 2 {
		t.Fatalf("protocol families len = %d, want 2", len(tools[0].ProtocolFamilies))
	}
}

func TestNormalizeCLITools_DefaultsIncludeIFLOW(t *testing.T) {
	var tools []config.CLIToolConfig
	if err := config.NormalizeCLITools(&tools, nil); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("cli tools len = %d, want 3", len(tools))
	}
	found := false
	for _, item := range tools {
		if item.ID != "iflow" {
			continue
		}
		found = true
		if item.ProtocolFamily != "openai" {
			t.Fatalf("iflow protocol_family = %q, want openai", item.ProtocolFamily)
		}
		if item.CLIFamily != "iflow" {
			t.Fatalf("iflow cli_family = %q, want iflow", item.CLIFamily)
		}
		if item.IFlowAuthMode != config.IFLOWAuthModeBrowser {
			t.Fatalf("iflow auth mode = %q, want %q", item.IFlowAuthMode, config.IFLOWAuthModeBrowser)
		}
		if item.IFlowBaseURL != iflowcli.DefaultBaseURL {
			t.Fatalf("iflow base url = %q, want %q", item.IFlowBaseURL, iflowcli.DefaultBaseURL)
		}
		if len(item.IFlowModels) != 1 || item.IFlowModels[0] != iflowcli.DefaultModel {
			t.Fatalf("iflow models = %#v, want [%q]", item.IFlowModels, iflowcli.DefaultModel)
		}
	}
	if !found {
		t.Fatalf("expected iflow in default cli tools")
	}
}

func TestNormalizeCLITools_BackfillsMissingBuiltinTools(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5.4", Provider: "openai", Model: "gpt-5.4", SourceID: "openai-default"}},
	}
	tools := []config.CLIToolConfig{
		{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", DefaultModelID: "gpt-5.4", Enabled: true},
		{ID: "claude", Label: "Claude Code", ProtocolFamily: "anthropic", CLIFamily: "claude", Enabled: true},
	}
	if err := config.NormalizeCLITools(&tools, llm); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("cli tools len = %d, want 3", len(tools))
	}
	found := false
	for _, item := range tools {
		if item.ID != "iflow" {
			continue
		}
		found = true
		if item.CLIFamily != "iflow" {
			t.Fatalf("iflow cli_family = %q, want iflow", item.CLIFamily)
		}
		if !item.Enabled {
			t.Fatalf("expected iflow to default enabled")
		}
		if item.IFlowDefaultModel != iflowcli.DefaultModel {
			t.Fatalf("iflow default model = %q, want %q", item.IFlowDefaultModel, iflowcli.DefaultModel)
		}
	}
	if !found {
		t.Fatalf("expected iflow to be backfilled")
	}
}

func TestNormalizeCLITools_NormalizesIFLOWFields(t *testing.T) {
	tools := []config.CLIToolConfig{{
		ID:                "iflow",
		Label:             "iFlow CLI",
		ProtocolFamily:    "openai",
		CLIFamily:         "iflow",
		Enabled:           true,
		IFlowAuthMode:     "API_KEY",
		IFlowBaseURL:      "",
		IFlowModels:       []string{" glm-4.7 ", "glm-4.7", "minimax-m2.5"},
		IFlowDefaultModel: "unknown",
	}}
	if err := config.NormalizeCLITools(&tools, nil); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	got := tools[0]
	if got.IFlowAuthMode != config.IFLOWAuthModeAPIKey {
		t.Fatalf("iflow auth mode = %q, want %q", got.IFlowAuthMode, config.IFLOWAuthModeAPIKey)
	}
	if got.IFlowBaseURL != iflowcli.DefaultBaseURL {
		t.Fatalf("iflow base url = %q, want %q", got.IFlowBaseURL, iflowcli.DefaultBaseURL)
	}
	if len(got.IFlowModels) != 2 {
		t.Fatalf("iflow models len = %d, want 2", len(got.IFlowModels))
	}
	if got.IFlowDefaultModel != "glm-4.7" {
		t.Fatalf("iflow default model = %q, want glm-4.7", got.IFlowDefaultModel)
	}
}
