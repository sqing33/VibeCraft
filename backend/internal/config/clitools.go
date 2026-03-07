package config

import (
	"fmt"
	"strings"
)

type CLIToolConfig struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	ProtocolFamily string `json:"protocol_family"`
	CLIFamily      string `json:"cli_family"`
	DefaultModelID string `json:"default_model_id,omitempty"`
	CommandPath    string `json:"command_path,omitempty"`
	Enabled        bool   `json:"enabled"`
}

func NormalizeCLITools(tools *[]CLIToolConfig, llm *LLMSettings) error {
	if tools == nil {
		return nil
	}
	if len(*tools) == 0 {
		*tools = defaultCLITools()
		return nil
	}
	seen := map[string]struct{}{}
	modelByID := llmModelByID(llm)
	for i := range *tools {
		item := &(*tools)[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Label = strings.TrimSpace(item.Label)
		item.ProtocolFamily = normalizeProvider(item.ProtocolFamily)
		item.CLIFamily = normalizeCLIFamily(item.CLIFamily)
		item.DefaultModelID = normalizeModelIdentifier(item.DefaultModelID)
		item.CommandPath = strings.TrimSpace(item.CommandPath)
		if item.ID == "" {
			return fmt.Errorf("cli_tools[%d].id is required", i)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("cli_tools[%d].id %q is duplicated", i, item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.ProtocolFamily != "openai" && item.ProtocolFamily != "anthropic" {
			return fmt.Errorf("cli_tools[%d].protocol_family %q is not supported", i, item.ProtocolFamily)
		}
		if item.CLIFamily == "" {
			return fmt.Errorf("cli_tools[%d].cli_family is required", i)
		}
		if item.Label == "" {
			item.Label = item.ID
		}
		if item.DefaultModelID != "" {
			model, ok := modelByID[item.DefaultModelID]
			if !ok {
				return fmt.Errorf("cli_tools[%d].default_model_id %q does not exist", i, item.DefaultModelID)
			}
			if normalizeProvider(model.Provider) != item.ProtocolFamily {
				return fmt.Errorf("cli_tools[%d].default_model_id %q is not compatible with protocol_family %q", i, item.DefaultModelID, item.ProtocolFamily)
			}
		}
	}
	return nil
}

func defaultCLITools() []CLIToolConfig {
	return []CLIToolConfig{
		{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true},
		{ID: "claude", Label: "Claude Code", ProtocolFamily: "anthropic", CLIFamily: "claude", Enabled: true},
	}
}

func llmModelByID(llm *LLMSettings) map[string]LLMModelConfig {
	out := map[string]LLMModelConfig{}
	if llm == nil {
		return out
	}
	for _, model := range llm.Models {
		id := normalizeModelIdentifier(model.ID)
		if id == "" {
			continue
		}
		out[id] = model
	}
	return out
}

func CLIToolByID(cfg Config) map[string]CLIToolConfig {
	out := map[string]CLIToolConfig{}
	for _, item := range cfg.CLITools {
		out[strings.TrimSpace(item.ID)] = item
	}
	return out
}

func ModelsForProtocol(llm *LLMSettings, provider string) []LLMModelConfig {
	provider = normalizeProvider(provider)
	out := make([]LLMModelConfig, 0)
	if llm == nil {
		return out
	}
	for _, model := range llm.Models {
		if normalizeProvider(model.Provider) == provider {
			out = append(out, model)
		}
	}
	return out
}
