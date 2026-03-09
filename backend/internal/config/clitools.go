package config

import (
	"fmt"
	"strings"
)

const (
	IFLOWAuthModeBrowser = "browser"
	IFLOWAuthModeAPIKey  = "api_key"
	defaultIFLOWBaseURL  = "https://apis.iflow.cn/v1"
	defaultIFLOWModel    = "glm-4.7"
)

type CLIToolConfig struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	ProtocolFamily    string   `json:"protocol_family"`
	ProtocolFamilies  []string `json:"protocol_families,omitempty"`
	CLIFamily         string   `json:"cli_family"`
	DefaultModelID    string   `json:"default_model_id,omitempty"`
	CommandPath       string   `json:"command_path,omitempty"`
	Enabled           bool     `json:"enabled"`
	IFlowAuthMode     string   `json:"iflow_auth_mode,omitempty"`
	IFlowAPIKey       string   `json:"iflow_api_key,omitempty"`
	IFlowBaseURL      string   `json:"iflow_base_url,omitempty"`
	IFlowModels       []string `json:"iflow_models,omitempty"`
	IFlowDefaultModel string   `json:"iflow_default_model,omitempty"`
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
		item.ProtocolFamilies = normalizeProtocolFamilyList(item.ProtocolFamilies)
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
		if item.CLIFamily == "" {
			return fmt.Errorf("cli_tools[%d].cli_family is required", i)
		}
		if item.Label == "" {
			item.Label = item.ID
		}
		if item.ProtocolFamily != "" && item.ProtocolFamily != "openai" && item.ProtocolFamily != "anthropic" {
			return fmt.Errorf("cli_tools[%d].protocol_family %q is not supported", i, item.ProtocolFamily)
		}
		for _, provider := range item.ProtocolFamilies {
			if provider != "openai" && provider != "anthropic" {
				return fmt.Errorf("cli_tools[%d].protocol_families contains unsupported value %q", i, provider)
			}
		}
		if item.ProtocolFamily != "" && !containsExact(item.ProtocolFamilies, item.ProtocolFamily) {
			item.ProtocolFamilies = append(item.ProtocolFamilies, item.ProtocolFamily)
		}
		if item.ProtocolFamily == "" && len(item.ProtocolFamilies) > 0 {
			item.ProtocolFamily = item.ProtocolFamilies[0]
		}
		if item.ProtocolFamily == "" {
			return fmt.Errorf("cli_tools[%d].protocol_family %q is not supported", i, item.ProtocolFamily)
		}
		if len(item.ProtocolFamilies) == 0 {
			item.ProtocolFamilies = []string{item.ProtocolFamily}
		}
		if isIFLOWTool(*item) {
			item.DefaultModelID = ""
			item.ProtocolFamily = firstNonEmptyTrimmed(item.ProtocolFamily, "openai")
			item.ProtocolFamilies = []string{"openai"}
			item.IFlowAuthMode = normalizeIFLOWAuthMode(item.IFlowAuthMode)
			item.IFlowBaseURL = firstNonEmptyTrimmed(item.IFlowBaseURL, defaultIFLOWBaseURL)
			item.IFlowAPIKey = strings.TrimSpace(item.IFlowAPIKey)
			item.IFlowModels = normalizeIFLOWModelList(item.IFlowModels)
			if len(item.IFlowModels) == 0 {
				item.IFlowModels = []string{defaultIFLOWModel}
			}
			item.IFlowDefaultModel = strings.TrimSpace(item.IFlowDefaultModel)
			if item.IFlowDefaultModel == "" || !containsExact(item.IFlowModels, item.IFlowDefaultModel) {
				item.IFlowDefaultModel = item.IFlowModels[0]
			}
			continue
		}
		item.IFlowAuthMode = ""
		item.IFlowAPIKey = ""
		item.IFlowBaseURL = ""
		item.IFlowModels = nil
		item.IFlowDefaultModel = ""
		if item.DefaultModelID != "" {
			model, ok := modelByID[item.DefaultModelID]
			if !ok {
				return fmt.Errorf("cli_tools[%d].default_model_id %q does not exist", i, item.DefaultModelID)
			}
			modelProvider := normalizeProvider(model.Provider)
			if !CLIToolSupportsProtocol(*item, modelProvider) {
				return fmt.Errorf("cli_tools[%d].default_model_id %q is not compatible with protocol_family %q", i, item.DefaultModelID, item.ProtocolFamily)
			}
			item.ProtocolFamily = modelProvider
			if !containsExact(item.ProtocolFamilies, modelProvider) {
				item.ProtocolFamilies = append(item.ProtocolFamilies, modelProvider)
			}
		}
	}
	for _, builtin := range defaultCLITools() {
		if _, ok := seen[builtin.ID]; ok {
			continue
		}
		*tools = append(*tools, builtin)
		seen[builtin.ID] = struct{}{}
	}
	return nil
}

func defaultCLITools() []CLIToolConfig {
	return []CLIToolConfig{
		{ID: "codex", Label: "Codex CLI", ProtocolFamily: "openai", ProtocolFamilies: []string{"openai"}, CLIFamily: "codex", Enabled: true},
		{ID: "claude", Label: "Claude Code", ProtocolFamily: "anthropic", ProtocolFamilies: []string{"anthropic"}, CLIFamily: "claude", Enabled: true},
		{ID: "iflow", Label: "iFlow CLI", ProtocolFamily: "openai", ProtocolFamilies: []string{"openai"}, CLIFamily: "iflow", Enabled: true, IFlowAuthMode: IFLOWAuthModeBrowser, IFlowBaseURL: defaultIFLOWBaseURL, IFlowModels: []string{defaultIFLOWModel}, IFlowDefaultModel: defaultIFLOWModel},
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

func CLIToolSupportsProtocol(item CLIToolConfig, provider string) bool {
	provider = normalizeProvider(provider)
	if provider == "" {
		return false
	}
	if normalizeProvider(item.ProtocolFamily) == provider {
		return true
	}
	for _, value := range item.ProtocolFamilies {
		if normalizeProvider(value) == provider {
			return true
		}
	}
	return false
}

func isIFLOWTool(item CLIToolConfig) bool {
	return normalizeCLIFamily(item.CLIFamily) == "iflow" || strings.TrimSpace(item.ID) == "iflow"
}

func normalizeIFLOWAuthMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case IFLOWAuthModeAPIKey:
		return IFLOWAuthModeAPIKey
	default:
		return IFLOWAuthModeBrowser
	}
}

func normalizeProtocolFamilyList(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := normalizeProvider(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeIFLOWModelList(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsExact(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
