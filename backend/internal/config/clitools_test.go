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

func TestNormalizeCLITools_AllowsMultiProtocolDefaultModel(t *testing.T) {
	llm := &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "anthropic-default", Provider: "anthropic"}},
		Models:  []config.LLMModelConfig{{ID: "claude-sonnet", Provider: "anthropic", Model: "claude-sonnet", SourceID: "anthropic-default"}},
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
	families := config.CLIToolProtocolFamilies(tools[0])
	if len(families) != 2 || families[0] != "openai" || families[1] != "anthropic" {
		t.Fatalf("protocol families = %#v, want [openai anthropic]", families)
	}
	if tools[0].ProtocolFamily != "openai" {
		t.Fatalf("protocol family = %q, want openai", tools[0].ProtocolFamily)
	}
}

func TestNormalizeCLITools_DefaultsIncludeIFLOWAndOpenCode(t *testing.T) {
	tools := []config.CLIToolConfig{}
	if err := config.NormalizeCLITools(&tools, nil); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if len(tools) != 4 {
		t.Fatalf("default cli tools len = %d, want 4", len(tools))
	}
	foundIFLOW := false
	foundOpenCode := false
	for _, item := range tools {
		switch item.ID {
		case "iflow":
			foundIFLOW = true
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
		case "opencode":
			foundOpenCode = true
			if item.ProtocolFamily != "openai" {
				t.Fatalf("opencode protocol_family = %q, want openai", item.ProtocolFamily)
			}
			families := config.CLIToolProtocolFamilies(item)
			if len(families) != 2 || families[0] != "openai" || families[1] != "anthropic" {
				t.Fatalf("opencode protocol_families = %#v, want [openai anthropic]", families)
			}
			if item.CLIFamily != "opencode" {
				t.Fatalf("opencode cli_family = %q, want opencode", item.CLIFamily)
			}
		}
	}
	if !foundIFLOW {
		t.Fatalf("expected iflow in default cli tools")
	}
	if !foundOpenCode {
		t.Fatalf("expected opencode in default cli tools")
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
	if len(tools) != 4 {
		t.Fatalf("cli tools len = %d, want 4", len(tools))
	}
	if tools[0].DefaultModelID != "gpt-5.4" {
		t.Fatalf("codex default_model_id = %q, want gpt-5.4", tools[0].DefaultModelID)
	}
	foundIFLOW := false
	foundOpenCode := false
	for _, item := range tools {
		switch item.ID {
		case "iflow":
			foundIFLOW = true
			if item.CLIFamily != "iflow" {
				t.Fatalf("iflow cli_family = %q, want iflow", item.CLIFamily)
			}
			if !item.Enabled {
				t.Fatalf("expected iflow to default enabled")
			}
			if item.IFlowDefaultModel != iflowcli.DefaultModel {
				t.Fatalf("iflow default model = %q, want %q", item.IFlowDefaultModel, iflowcli.DefaultModel)
			}
		case "opencode":
			foundOpenCode = true
			if !item.Enabled {
				t.Fatalf("expected opencode to default enabled")
			}
			families := config.CLIToolProtocolFamilies(item)
			if len(families) != 2 || families[0] != "openai" || families[1] != "anthropic" {
				t.Fatalf("opencode protocol_families = %#v, want [openai anthropic]", families)
			}
		}
	}
	if !foundIFLOW {
		t.Fatalf("expected iflow to be backfilled")
	}
	if !foundOpenCode {
		t.Fatalf("expected opencode to be backfilled")
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
