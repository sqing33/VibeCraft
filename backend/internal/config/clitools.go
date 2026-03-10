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
	_ = llm
	if len(*tools) == 0 {
		*tools = defaultCLITools()
		return nil
	}

	seen := map[string]struct{}{}
	for i := range *tools {
		item := &(*tools)[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Label = strings.TrimSpace(item.Label)
		item.ProtocolFamily = normalizeProvider(item.ProtocolFamily)
		item.ProtocolFamilies = normalizeProtocolFamilies(item.ProtocolFamilies)
		if len(item.ProtocolFamilies) == 0 && item.ProtocolFamily != "" {
			item.ProtocolFamilies = []string{item.ProtocolFamily}
		}
		if item.ProtocolFamily == "" && len(item.ProtocolFamilies) > 0 {
			item.ProtocolFamily = item.ProtocolFamilies[0]
		}
		item.ProtocolFamilies = prioritizeProtocolFamily(item.ProtocolFamilies, item.ProtocolFamily)
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
		for _, family := range item.ProtocolFamilies {
			if family != "openai" && family != "anthropic" {
				return fmt.Errorf("cli_tools[%d].protocol_families contains unsupported value %q", i, family)
			}
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
		if item.ProtocolFamily == "" {
			return fmt.Errorf("cli_tools[%d].protocol_family %q is not supported", i, item.ProtocolFamily)
		}
		if len(item.ProtocolFamilies) == 0 {
			item.ProtocolFamilies = []string{item.ProtocolFamily}
		} else {
			item.ProtocolFamilies = prioritizeProtocolFamily(item.ProtocolFamilies, item.ProtocolFamily)
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
		{ID: "opencode", Label: "OpenCode CLI", ProtocolFamily: "openai", ProtocolFamilies: []string{"openai", "anthropic"}, CLIFamily: "opencode", Enabled: true},
	}
}

func normalizeProtocolFamilies(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeProvider(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func prioritizeProtocolFamily(families []string, primary string) []string {
	families = normalizeProtocolFamilies(families)
	primary = normalizeProvider(primary)
	if primary == "" {
		return families
	}
	index := -1
	for i, family := range families {
		if family == primary {
			index = i
			break
		}
	}
	if index == 0 {
		return families
	}
	out := make([]string, 0, len(families)+1)
	out = append(out, primary)
	for _, family := range families {
		if family == primary {
			continue
		}
		out = append(out, family)
	}
	return out
}

func CLIToolProtocolFamilies(tool CLIToolConfig) []string {
	families := prioritizeProtocolFamily(normalizeProtocolFamilies(tool.ProtocolFamilies), tool.ProtocolFamily)
	if len(families) > 0 {
		return families
	}
	if family := normalizeProvider(tool.ProtocolFamily); family != "" {
		return []string{family}
	}
	return nil
}

func CLIToolSupportsProtocolFamily(tool CLIToolConfig, provider string) bool {
	provider = normalizeProvider(provider)
	if provider == "" {
		return false
	}
	for _, family := range CLIToolProtocolFamilies(tool) {
		if family == provider {
			return true
		}
	}
	return false
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

func ModelsForProtocols(llm *LLMSettings, providers []string) []LLMModelConfig {
	allowed := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		provider = normalizeProvider(provider)
		if provider == "" {
			continue
		}
		allowed[provider] = struct{}{}
	}
	out := make([]LLMModelConfig, 0)
	if llm == nil || len(allowed) == 0 {
		return out
	}
	for _, model := range llm.Models {
		if _, ok := allowed[normalizeProvider(model.Provider)]; ok {
			out = append(out, model)
		}
	}
	return out
}

func CLIToolSupportsProtocol(item CLIToolConfig, provider string) bool {
	return CLIToolSupportsProtocolFamily(item, provider)
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
