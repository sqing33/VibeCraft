package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderIFLOW     = "iflow"

	RuntimeIDSDKOpenAI    = "sdk-openai"
	RuntimeIDSDKAnthropic = "sdk-anthropic"
	RuntimeIDCodex        = "codex"
	RuntimeIDClaude       = "claude"
	RuntimeIDIFLOW        = "iflow"
	RuntimeIDOpenCode     = "opencode"

	RuntimeKindSDK = "sdk"
	RuntimeKindCLI = "cli"
)

type APISourceConfig struct {
	ID    string `json:"id"`
	Label string `json:"label"`

	// Deprecated：仅用于兼容旧配置解码；新逻辑不再依赖来源级 provider。
	Provider string `json:"provider,omitempty"`

	BaseURL  string `json:"base_url,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	AuthMode string `json:"auth_mode,omitempty"`
}

type RuntimeModelSettings struct {
	Runtimes []RuntimeModelRuntimeConfig `json:"runtimes"`
}

type RuntimeModelRuntimeConfig struct {
	ID             string               `json:"id"`
	Label          string               `json:"label"`
	Kind           string               `json:"kind"`
	Provider       string               `json:"provider,omitempty"`
	CLIToolID      string               `json:"cli_tool_id,omitempty"`
	DefaultModelID string               `json:"default_model_id,omitempty"`
	Models         []RuntimeModelConfig `json:"models"`
}

type RuntimeModelConfig struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`

	Model    string `json:"model"`
	SourceID string `json:"source_id"`

	OpenAIAPIStyle           string `json:"openai_api_style,omitempty"`
	OpenAIAPIStyleDetectedAt int64  `json:"openai_api_style_detected_at,omitempty"`

	SystemPrompt    string   `json:"system_prompt,omitempty"`
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	OutputSchema    string   `json:"output_schema,omitempty"`
	TimeoutMs       int      `json:"timeout_ms,omitempty"`
}

type ResolvedRuntimeModel struct {
	Runtime RuntimeModelRuntimeConfig
	Model   RuntimeModelConfig
	Source  APISourceConfig
}

func defaultRuntimeCatalog() []RuntimeModelRuntimeConfig {
	return []RuntimeModelRuntimeConfig{
		{ID: RuntimeIDSDKOpenAI, Label: "OpenAI SDK", Kind: RuntimeKindSDK, Provider: "openai", Models: []RuntimeModelConfig{}},
		{ID: RuntimeIDSDKAnthropic, Label: "Anthropic SDK", Kind: RuntimeKindSDK, Provider: "anthropic", Models: []RuntimeModelConfig{}},
		{ID: RuntimeIDCodex, Label: "Codex CLI", Kind: RuntimeKindCLI, Provider: "openai", CLIToolID: "codex", Models: []RuntimeModelConfig{}},
		{ID: RuntimeIDClaude, Label: "Claude Code", Kind: RuntimeKindCLI, Provider: "anthropic", CLIToolID: "claude", Models: []RuntimeModelConfig{}},
		{ID: RuntimeIDIFLOW, Label: "iFlow CLI", Kind: RuntimeKindCLI, Provider: "iflow", CLIToolID: "iflow", Models: []RuntimeModelConfig{}},
		{ID: RuntimeIDOpenCode, Label: "OpenCode CLI", Kind: RuntimeKindCLI, Provider: "openai", CLIToolID: "opencode", Models: []RuntimeModelConfig{}},
	}
}

func defaultRuntimeByID() map[string]RuntimeModelRuntimeConfig {
	out := make(map[string]RuntimeModelRuntimeConfig, 6)
	for _, item := range defaultRuntimeCatalog() {
		out[item.ID] = item
	}
	return out
}

func runtimeIDForCLITool(toolID string) string {
	switch strings.TrimSpace(toolID) {
	case RuntimeIDCodex:
		return RuntimeIDCodex
	case RuntimeIDClaude:
		return RuntimeIDClaude
	case RuntimeIDIFLOW:
		return RuntimeIDIFLOW
	case RuntimeIDOpenCode:
		return RuntimeIDOpenCode
	default:
		return ""
	}
}

func NormalizeAPISources(sources *[]APISourceConfig) error {
	if sources == nil {
		return nil
	}
	if len(*sources) == 0 {
		*sources = nil
		return nil
	}
	seen := make(map[string]struct{}, len(*sources))
	out := make([]APISourceConfig, 0, len(*sources))
	for i := range *sources {
		item := (*sources)[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Label = strings.TrimSpace(item.Label)
		item.Provider = normalizeProvider(item.Provider)
		item.BaseURL = strings.TrimSpace(item.BaseURL)
		item.APIKey = strings.TrimSpace(item.APIKey)
		rawAuthMode := item.AuthMode
		item.AuthMode = normalizeAPISourceAuthMode(item.AuthMode)
		if item.ID == "" {
			item.ID = nextAPISourceID(item.Label, seen)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("api_sources[%d].id %q is duplicated", i, item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Label == "" {
			item.Label = item.ID
		}
		if item.BaseURL != "" {
			parsed, err := url.Parse(item.BaseURL)
			if err != nil || parsed == nil {
				return fmt.Errorf("api_sources[%d].base_url is invalid", i)
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				return fmt.Errorf("api_sources[%d].base_url must start with http:// or https://", i)
			}
		}
		if strings.TrimSpace(rawAuthMode) != "" && item.AuthMode == "" {
			return fmt.Errorf("api_sources[%d].auth_mode %q is not supported", i, strings.TrimSpace(rawAuthMode))
		}
		out = append(out, item)
	}
	*sources = out
	return nil
}

func nextAPISourceID(label string, seen map[string]struct{}) string {
	base := normalizeAPISourceGeneratedID(label)
	if base == "" {
		base = "source"
	}
	candidate := base
	for index := 2; ; index++ {
		if _, ok := seen[candidate]; !ok {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, index)
	}
}

func normalizeAPISourceGeneratedID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func normalizeAPISourceAuthMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case IFLOWAuthModeAPIKey:
		return IFLOWAuthModeAPIKey
	case IFLOWAuthModeBrowser:
		return IFLOWAuthModeBrowser
	default:
		return ""
	}
}

func NormalizeRuntimeModelSettings(settings **RuntimeModelSettings, sources []APISourceConfig) error {
	if settings == nil {
		return nil
	}
	catalog := defaultRuntimeByID()
	if *settings == nil {
		*settings = &RuntimeModelSettings{}
	}
	sourceByID := make(map[string]APISourceConfig, len(sources))
	for _, source := range sources {
		sourceByID[strings.TrimSpace(source.ID)] = source
	}
	seenRuntime := make(map[string]struct{}, len((*settings).Runtimes))
	seenByRuntime := make(map[string]map[string]struct{}, len((*settings).Runtimes))
	outByID := make(map[string]RuntimeModelRuntimeConfig, len(catalog))
	for i := range (*settings).Runtimes {
		runtime := (*settings).Runtimes[i]
		runtime.ID = strings.TrimSpace(runtime.ID)
		if runtime.ID == "" {
			return fmt.Errorf("runtime_model_settings.runtimes[%d].id is required", i)
		}
		if _, ok := seenRuntime[runtime.ID]; ok {
			return fmt.Errorf("runtime_model_settings.runtimes[%d].id %q is duplicated", i, runtime.ID)
		}
		base, ok := catalog[runtime.ID]
		if !ok {
			return fmt.Errorf("runtime_model_settings.runtimes[%d].id %q is not supported", i, runtime.ID)
		}
		seenRuntime[runtime.ID] = struct{}{}
		if strings.TrimSpace(runtime.Label) == "" {
			runtime.Label = base.Label
		}
		runtime.Kind = base.Kind
		runtime.Provider = base.Provider
		runtime.CLIToolID = base.CLIToolID
		runtime.DefaultModelID = normalizeModelIdentifier(runtime.DefaultModelID)
		seenModels := make(map[string]struct{}, len(runtime.Models))
		models := make([]RuntimeModelConfig, 0, len(runtime.Models))
		for j := range runtime.Models {
			model := runtime.Models[j]
			model.ID = normalizeModelIdentifier(model.ID)
			model.Label = strings.TrimSpace(model.Label)
			model.Provider = normalizeProvider(model.Provider)
			model.Model = normalizeModelIdentifier(model.Model)
			model.SourceID = strings.TrimSpace(model.SourceID)
			model.OpenAIAPIStyle = normalizeOpenAIAPIStyle(model.OpenAIAPIStyle)
			if model.OpenAIAPIStyle == "" {
				model.OpenAIAPIStyleDetectedAt = 0
			}
			model.SystemPrompt = strings.TrimSpace(model.SystemPrompt)
			model.OutputSchema = strings.TrimSpace(model.OutputSchema)
			if model.ID == "" {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].models[%d].id is required", i, j)
			}
			if _, ok := seenModels[model.ID]; ok {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].models[%d].id %q is duplicated", i, j, model.ID)
			}
			seenModels[model.ID] = struct{}{}
			if model.SourceID == "" {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].models[%d].source_id is required", i, j)
			}
			if _, ok := sourceByID[model.SourceID]; !ok {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].models[%d].source_id %q does not exist", i, j, model.SourceID)
			}
			if model.Provider == "" {
				model.Provider = normalizeProvider(base.Provider)
			}
			if model.Provider != "openai" && model.Provider != "anthropic" && model.Provider != "iflow" {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].models[%d].provider %q is not supported", i, j, model.Provider)
			}
			if !runtimeAllowsProvider(runtime.ID, model.Provider) {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].models[%d].provider %q is not allowed for runtime %q", i, j, model.Provider, runtime.ID)
			}
			if model.Model == "" {
				model.Model = model.ID
			}
			if model.Label == "" {
				model.Label = model.ID
			}
			models = append(models, model)
		}
		runtime.Models = models
		if runtime.DefaultModelID != "" {
			if _, ok := seenModels[runtime.DefaultModelID]; !ok {
				return fmt.Errorf("runtime_model_settings.runtimes[%d].default_model_id %q does not exist", i, runtime.DefaultModelID)
			}
		}
		if runtime.DefaultModelID == "" && len(runtime.Models) > 0 {
			runtime.DefaultModelID = runtime.Models[0].ID
		}
		seenByRuntime[runtime.ID] = seenModels
		outByID[runtime.ID] = runtime
	}
	ordered := make([]RuntimeModelRuntimeConfig, 0, len(catalog))
	for _, base := range defaultRuntimeCatalog() {
		runtime, ok := outByID[base.ID]
		if !ok {
			runtime = base
		}
		ordered = append(ordered, runtime)
	}
	(*settings).Runtimes = ordered
	return nil
}

func runtimeAllowsProvider(runtimeID, provider string) bool {
	provider = normalizeProvider(provider)
	switch strings.TrimSpace(runtimeID) {
	case RuntimeIDSDKOpenAI, RuntimeIDCodex:
		return provider == "openai"
	case RuntimeIDSDKAnthropic, RuntimeIDClaude:
		return provider == "anthropic"
	case RuntimeIDIFLOW:
		return provider == "iflow"
	case RuntimeIDOpenCode:
		return provider == "openai" || provider == "anthropic"
	default:
		return false
	}
}

func FindAPISourceByID(sources []APISourceConfig, sourceID string) (APISourceConfig, bool) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return APISourceConfig{}, false
	}
	for _, source := range sources {
		if strings.TrimSpace(source.ID) == sourceID {
			return source, true
		}
	}
	return APISourceConfig{}, false
}

func FindRuntimeConfigByID(settings *RuntimeModelSettings, runtimeID string) (RuntimeModelRuntimeConfig, bool) {
	if settings == nil {
		return RuntimeModelRuntimeConfig{}, false
	}
	runtimeID = strings.TrimSpace(runtimeID)
	for _, runtime := range settings.Runtimes {
		if strings.TrimSpace(runtime.ID) == runtimeID {
			return runtime, true
		}
	}
	return RuntimeModelRuntimeConfig{}, false
}

func ResolveRuntimeModelBinding(settings *RuntimeModelSettings, sources []APISourceConfig, runtimeID, modelID string) (*ResolvedRuntimeModel, bool) {
	runtime, ok := FindRuntimeConfigByID(settings, runtimeID)
	if !ok {
		return nil, false
	}
	modelID = normalizeModelIdentifier(modelID)
	if modelID == "" {
		modelID = normalizeModelIdentifier(runtime.DefaultModelID)
	}
	if modelID == "" {
		return nil, false
	}
	for _, model := range runtime.Models {
		if normalizeModelIdentifier(model.ID) != modelID {
			continue
		}
		source, ok := FindAPISourceByID(sources, model.SourceID)
		if !ok {
			return nil, false
		}
		return &ResolvedRuntimeModel{Runtime: runtime, Model: model, Source: source}, true
	}
	return nil, false
}

func ResolveSDKRuntimeModelByID(settings *RuntimeModelSettings, sources []APISourceConfig, modelID string) (*ResolvedRuntimeModel, bool) {
	for _, runtimeID := range []string{RuntimeIDSDKOpenAI, RuntimeIDSDKAnthropic} {
		resolved, ok := ResolveRuntimeModelBinding(settings, sources, runtimeID, modelID)
		if ok {
			return resolved, true
		}
	}
	return nil, false
}

func FindRuntimeModelByID(cfg Config, modelID string) (RuntimeModelRuntimeConfig, RuntimeModelConfig, APISourceConfig, bool) {
	if cfg.RuntimeModels == nil {
		return RuntimeModelRuntimeConfig{}, RuntimeModelConfig{}, APISourceConfig{}, false
	}
	for _, runtime := range cfg.RuntimeModels.Runtimes {
		resolved, ok := ResolveRuntimeModelBinding(cfg.RuntimeModels, cfg.APISources, runtime.ID, modelID)
		if ok {
			return resolved.Runtime, resolved.Model, resolved.Source, true
		}
	}
	return RuntimeModelRuntimeConfig{}, RuntimeModelConfig{}, APISourceConfig{}, false
}

func HydrateRuntimeSettings(cfg *Config) error {
	if cfg == nil {
		return errNilConfig
	}
	if err := NormalizeAPISources(&cfg.APISources); err != nil {
		return err
	}
	if cfg.RuntimeModels != nil {
		if err := NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
			return err
		}
	}
	if len(cfg.APISources) == 0 {
		cfg.APISources = deriveAPISourcesFromLegacy(cfg.LLM, cfg.CLITools)
		if err := NormalizeAPISources(&cfg.APISources); err != nil {
			return err
		}
	}
	if cfg.RuntimeModels == nil || len(cfg.RuntimeModels.Runtimes) == 0 {
		cfg.RuntimeModels = deriveRuntimeModelsFromLegacy(cfg.LLM, cfg.CLITools, cfg.APISources)
	}
	if err := NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
		return err
	}
	applyRuntimeSettingMirrors(cfg)
	ReconcileBasicSettingsWithRuntime(&cfg.Basic, *cfg)
	return nil
}

func deriveAPISourcesFromLegacy(llm *LLMSettings, tools []CLIToolConfig) []APISourceConfig {
	out := make([]APISourceConfig, 0)
	used := make(map[string]struct{})
	for _, source := range legacySourcesOrEmpty(llm) {
		id := strings.TrimSpace(source.ID)
		if id == "" {
			continue
		}
		if _, ok := used[id]; ok {
			continue
		}
		used[id] = struct{}{}
		out = append(out, APISourceConfig{
			ID:       id,
			Label:    firstNonEmptyTrimmed(source.Label, id),
			Provider: normalizeProvider(source.Provider),
			BaseURL:  strings.TrimSpace(source.BaseURL),
			APIKey:   strings.TrimSpace(source.APIKey),
		})
	}
	iflowTool := CLIToolConfig{IFlowBaseURL: defaultIFLOWBaseURL, IFlowAuthMode: IFLOWAuthModeBrowser}
	for _, tool := range tools {
		if !isIFLOWTool(tool) {
			continue
		}
		iflowTool = tool
		break
	}
	id := nextUniqueSourceID("iflow-default", used)
	used[id] = struct{}{}
	out = append(out, APISourceConfig{
		ID:       id,
		Label:    "iFlow 官方来源",
		Provider: "iflow",
		BaseURL:  firstNonEmptyTrimmed(iflowTool.IFlowBaseURL, defaultIFLOWBaseURL),
		APIKey:   strings.TrimSpace(iflowTool.IFlowAPIKey),
		AuthMode: firstNonEmptyTrimmed(iflowTool.IFlowAuthMode, IFLOWAuthModeBrowser),
	})
	return out
}

func deriveRuntimeModelsFromLegacy(llm *LLMSettings, tools []CLIToolConfig, sources []APISourceConfig) *RuntimeModelSettings {
	settings := &RuntimeModelSettings{Runtimes: defaultRuntimeCatalog()}
	indexByID := make(map[string]int, len(settings.Runtimes))
	for i := range settings.Runtimes {
		indexByID[settings.Runtimes[i].ID] = i
	}
	if llm != nil {
		for _, model := range llm.Models {
			provider := normalizeProvider(model.Provider)
			runtimeID := runtimeIDForSDKProvider(provider)
			if runtimeID != "" {
				idx := indexByID[runtimeID]
				settings.Runtimes[idx].Models = append(settings.Runtimes[idx].Models, runtimeModelFromLegacy(model))
			}
			for _, runtimeID := range runtimeIDsForCLIProvider(provider) {
				idx := indexByID[runtimeID]
				settings.Runtimes[idx].Models = append(settings.Runtimes[idx].Models, runtimeModelFromLegacy(model))
			}
		}
	}
	if iflowSource, ok := firstIFLOWSource(sources); ok {
		idx := indexByID[RuntimeIDIFLOW]
		models, defaultModel := deriveIFLOWRuntimeModels(tools, iflowSource.ID)
		settings.Runtimes[idx].Models = models
		settings.Runtimes[idx].DefaultModelID = defaultModel
	}
	for i := range settings.Runtimes {
		if settings.Runtimes[i].DefaultModelID != "" {
			continue
		}
		toolDefault := legacyCLIToolDefaultModelID(tools, settings.Runtimes[i].ID)
		if toolDefault != "" {
			settings.Runtimes[i].DefaultModelID = normalizeModelIdentifier(toolDefault)
		}
		if settings.Runtimes[i].DefaultModelID == "" && len(settings.Runtimes[i].Models) > 0 {
			settings.Runtimes[i].DefaultModelID = settings.Runtimes[i].Models[0].ID
		}
	}
	return settings
}

func legacySourcesOrEmpty(llm *LLMSettings) []LLMSourceConfig {
	if llm == nil {
		return nil
	}
	return llm.Sources
}

func nextUniqueSourceID(base string, used map[string]struct{}) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "source"
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; i < 1000; i++ {
		id := fmt.Sprintf("%s-%d", base, i)
		if _, ok := used[id]; !ok {
			return id
		}
	}
	return fmt.Sprintf("%s-%d", base, len(used)+1)
}

func runtimeModelFromLegacy(model LLMModelConfig) RuntimeModelConfig {
	return RuntimeModelConfig{
		ID:                       normalizeModelIdentifier(model.ID),
		Label:                    strings.TrimSpace(model.Label),
		Provider:                 normalizeProvider(model.Provider),
		Model:                    strings.TrimSpace(model.Model),
		SourceID:                 strings.TrimSpace(model.SourceID),
		OpenAIAPIStyle:           normalizeOpenAIAPIStyle(model.OpenAIAPIStyle),
		OpenAIAPIStyleDetectedAt: model.OpenAIAPIStyleDetectedAt,
		SystemPrompt:             strings.TrimSpace(model.SystemPrompt),
		MaxOutputTokens:          model.MaxOutputTokens,
		Temperature:              model.Temperature,
		OutputSchema:             strings.TrimSpace(model.OutputSchema),
		TimeoutMs:                model.TimeoutMs,
	}
}

func runtimeIDForSDKProvider(provider string) string {
	switch normalizeProvider(provider) {
	case "openai":
		return RuntimeIDSDKOpenAI
	case "anthropic":
		return RuntimeIDSDKAnthropic
	default:
		return ""
	}
}

func runtimeIDsForCLIProvider(provider string) []string {
	switch normalizeProvider(provider) {
	case "openai":
		return []string{RuntimeIDCodex, RuntimeIDOpenCode}
	case "anthropic":
		return []string{RuntimeIDClaude, RuntimeIDOpenCode}
	default:
		return nil
	}
}

func isIFLOWAPISource(source APISourceConfig) bool {
	return strings.TrimSpace(source.AuthMode) != "" || normalizeProvider(source.Provider) == ProviderIFLOW
}

func firstIFLOWSource(sources []APISourceConfig) (APISourceConfig, bool) {
	for _, source := range sources {
		if isIFLOWAPISource(source) {
			return source, true
		}
	}
	return APISourceConfig{}, false
}

func deriveIFLOWRuntimeModels(tools []CLIToolConfig, sourceID string) ([]RuntimeModelConfig, string) {
	tool := CLIToolConfig{IFlowModels: []string{defaultIFLOWModel}, IFlowDefaultModel: defaultIFLOWModel}
	for _, item := range tools {
		if !isIFLOWTool(item) {
			continue
		}
		tool = item
		break
	}
	models := make([]RuntimeModelConfig, 0, len(tool.IFlowModels))
	for _, model := range normalizeIFLOWModelList(tool.IFlowModels) {
		models = append(models, RuntimeModelConfig{
			ID:       normalizeModelIdentifier(model),
			Label:    strings.TrimSpace(model),
			Provider: "iflow",
			Model:    strings.TrimSpace(model),
			SourceID: strings.TrimSpace(sourceID),
		})
	}
	defaultModel := normalizeModelIdentifier(tool.IFlowDefaultModel)
	if defaultModel == "" && len(models) > 0 {
		defaultModel = models[0].ID
	}
	return models, defaultModel
}

func legacyCLIToolDefaultModelID(tools []CLIToolConfig, runtimeID string) string {
	for _, tool := range tools {
		if runtimeIDForCLITool(tool.ID) != runtimeID {
			continue
		}
		if runtimeID == RuntimeIDIFLOW {
			return strings.TrimSpace(tool.IFlowDefaultModel)
		}
		return strings.TrimSpace(tool.DefaultModelID)
	}
	return ""
}

func applyRuntimeSettingMirrors(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.LLM = DeriveLegacyLLMFromRuntimeSettings(cfg.APISources, cfg.RuntimeModels)
	mirrorCLIToolsFromRuntimeSettings(&cfg.CLITools, cfg.APISources, cfg.RuntimeModels)
}

func DeriveLegacyLLMFromRuntimeSettings(sources []APISourceConfig, settings *RuntimeModelSettings) *LLMSettings {
	if settings == nil {
		return nil
	}
	allowed := map[string]struct{}{}
	sourceProviderByID := map[string]string{}
	models := make([]LLMModelConfig, 0)
	for _, runtimeID := range []string{RuntimeIDSDKOpenAI, RuntimeIDSDKAnthropic} {
		runtime, ok := FindRuntimeConfigByID(settings, runtimeID)
		if !ok {
			continue
		}
		for _, model := range runtime.Models {
			sourceID := strings.TrimSpace(model.SourceID)
			allowed[sourceID] = struct{}{}
			provider := normalizeProvider(model.Provider)
			if existing, ok := sourceProviderByID[sourceID]; !ok {
				sourceProviderByID[sourceID] = provider
			} else if existing != provider {
				sourceProviderByID[sourceID] = ""
			}
			models = append(models, LLMModelConfig{
				ID:                       normalizeModelIdentifier(model.ID),
				Label:                    strings.TrimSpace(model.Label),
				Provider:                 provider,
				Model:                    strings.TrimSpace(model.Model),
				SourceID:                 sourceID,
				OpenAIAPIStyle:           normalizeOpenAIAPIStyle(model.OpenAIAPIStyle),
				OpenAIAPIStyleDetectedAt: model.OpenAIAPIStyleDetectedAt,
				SystemPrompt:             strings.TrimSpace(model.SystemPrompt),
				MaxOutputTokens:          model.MaxOutputTokens,
				Temperature:              model.Temperature,
				OutputSchema:             strings.TrimSpace(model.OutputSchema),
				TimeoutMs:                model.TimeoutMs,
			})
		}
	}
	if len(models) == 0 && len(allowed) == 0 {
		return nil
	}
	legacySources := make([]LLMSourceConfig, 0, len(allowed))
	for _, source := range sources {
		sourceID := strings.TrimSpace(source.ID)
		if _, ok := allowed[sourceID]; !ok {
			continue
		}
		legacySources = append(legacySources, LLMSourceConfig{
			ID:       sourceID,
			Label:    strings.TrimSpace(source.Label),
			Provider: sourceProviderByID[sourceID],
			BaseURL:  strings.TrimSpace(source.BaseURL),
			APIKey:   strings.TrimSpace(source.APIKey),
		})
	}
	sort.Slice(legacySources, func(i, j int) bool { return legacySources[i].ID < legacySources[j].ID })
	return &LLMSettings{Sources: legacySources, Models: models}
}

func mirrorCLIToolsFromRuntimeSettings(tools *[]CLIToolConfig, sources []APISourceConfig, settings *RuntimeModelSettings) {
	if tools == nil {
		return
	}
	if len(*tools) == 0 {
		*tools = defaultCLITools()
	}
	for i := range *tools {
		tool := &(*tools)[i]
		runtimeID := runtimeIDForCLITool(tool.ID)
		if runtimeID == "" {
			continue
		}
		runtime, ok := FindRuntimeConfigByID(settings, runtimeID)
		if !ok {
			continue
		}
		if runtimeID == RuntimeIDIFLOW {
			tool.IFlowModels = nil
			for _, model := range runtime.Models {
				tool.IFlowModels = append(tool.IFlowModels, strings.TrimSpace(model.Model))
			}
			tool.IFlowDefaultModel = strings.TrimSpace(runtime.DefaultModelID)
			if resolved, ok := ResolveRuntimeModelBinding(settings, sources, runtimeID, runtime.DefaultModelID); ok {
				tool.IFlowAuthMode = firstNonEmptyTrimmed(resolved.Source.AuthMode, IFLOWAuthModeBrowser)
				tool.IFlowBaseURL = firstNonEmptyTrimmed(resolved.Source.BaseURL, defaultIFLOWBaseURL)
				tool.IFlowAPIKey = strings.TrimSpace(resolved.Source.APIKey)
			} else if source, ok := firstIFLOWSource(sources); ok {
				tool.IFlowAuthMode = firstNonEmptyTrimmed(source.AuthMode, IFLOWAuthModeBrowser)
				tool.IFlowBaseURL = firstNonEmptyTrimmed(source.BaseURL, defaultIFLOWBaseURL)
				tool.IFlowAPIKey = strings.TrimSpace(source.APIKey)
			}
			continue
		}
		tool.DefaultModelID = strings.TrimSpace(runtime.DefaultModelID)
	}
}

func SyncSDKRuntimeSettingsFromLLM(cfg *Config, llm *LLMSettings) error {
	if cfg == nil {
		return errNilConfig
	}
	if err := NormalizeAPISources(&cfg.APISources); err != nil {
		return err
	}
	if cfg.RuntimeModels == nil {
		cfg.RuntimeModels = &RuntimeModelSettings{Runtimes: defaultRuntimeCatalog()}
	}
	if err := NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
		return err
	}
	if llm == nil {
		clearRuntimeModels(cfg.RuntimeModels, RuntimeIDSDKOpenAI, RuntimeIDSDKAnthropic)
		applyRuntimeSettingMirrors(cfg)
		return nil
	}
	if err := NormalizeLLMSettings(llm); err != nil {
		return err
	}
	mergedSources := mergeSDKAPISources(cfg.APISources, llm.Sources)
	cfg.APISources = mergedSources
	if err := NormalizeAPISources(&cfg.APISources); err != nil {
		return err
	}
	clearRuntimeModels(cfg.RuntimeModels, RuntimeIDSDKOpenAI, RuntimeIDSDKAnthropic)
	for _, model := range llm.Models {
		runtimeID := runtimeIDForSDKProvider(model.Provider)
		if runtimeID == "" {
			continue
		}
		appendRuntimeModel(cfg.RuntimeModels, runtimeID, runtimeModelFromLegacy(model), normalizeModelIdentifier(model.ID))
	}
	applyRuntimeSettingMirrors(cfg)
	return NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources)
}

func SyncCLIToolRuntimeSettings(cfg *Config, tools []CLIToolConfig) error {
	if cfg == nil {
		return errNilConfig
	}
	cfg.CLITools = append([]CLIToolConfig(nil), tools...)
	if err := NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		return err
	}
	if cfg.RuntimeModels == nil {
		cfg.RuntimeModels = &RuntimeModelSettings{Runtimes: defaultRuntimeCatalog()}
	}
	if err := NormalizeAPISources(&cfg.APISources); err != nil {
		return err
	}
	if err := NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
		return err
	}
	for _, tool := range cfg.CLITools {
		runtimeID := runtimeIDForCLITool(tool.ID)
		if runtimeID == "" {
			continue
		}
		if runtimeID == RuntimeIDIFLOW {
			sourceID := ensureIFLOWSource(cfg, tool)
			models, defaultModel := deriveIFLOWRuntimeModels([]CLIToolConfig{tool}, sourceID)
			replaceRuntimeModels(cfg.RuntimeModels, runtimeID, models, defaultModel)
			continue
		}
		if strings.TrimSpace(tool.DefaultModelID) != "" {
			setRuntimeDefaultModel(cfg.RuntimeModels, runtimeID, tool.DefaultModelID)
		}
	}
	applyRuntimeSettingMirrors(cfg)
	return NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources)
}

func mergeSDKAPISources(existing []APISourceConfig, llmSources []LLMSourceConfig) []APISourceConfig {
	out := append([]APISourceConfig(nil), existing...)
	index := make(map[string]int, len(out))
	for i := range out {
		index[strings.TrimSpace(out[i].ID)] = i
	}
	for _, source := range llmSources {
		provider := normalizeProvider(source.Provider)
		if provider != "openai" && provider != "anthropic" {
			continue
		}
		item := APISourceConfig{
			ID:       strings.TrimSpace(source.ID),
			Label:    firstNonEmptyTrimmed(source.Label, source.ID),
			Provider: provider,
			BaseURL:  strings.TrimSpace(source.BaseURL),
			APIKey:   strings.TrimSpace(source.APIKey),
		}
		if idx, ok := index[item.ID]; ok {
			out[idx] = item
			continue
		}
		index[item.ID] = len(out)
		out = append(out, item)
	}
	return out
}

func clearRuntimeModels(settings *RuntimeModelSettings, runtimeIDs ...string) {
	if settings == nil {
		return
	}
	allowed := make(map[string]struct{}, len(runtimeIDs))
	for _, runtimeID := range runtimeIDs {
		allowed[strings.TrimSpace(runtimeID)] = struct{}{}
	}
	for i := range settings.Runtimes {
		if _, ok := allowed[strings.TrimSpace(settings.Runtimes[i].ID)]; !ok {
			continue
		}
		settings.Runtimes[i].Models = nil
		settings.Runtimes[i].DefaultModelID = ""
	}
}

func appendRuntimeModel(settings *RuntimeModelSettings, runtimeID string, model RuntimeModelConfig, defaultModelID string) {
	if settings == nil {
		return
	}
	for i := range settings.Runtimes {
		if strings.TrimSpace(settings.Runtimes[i].ID) != strings.TrimSpace(runtimeID) {
			continue
		}
		settings.Runtimes[i].Models = append(settings.Runtimes[i].Models, model)
		if strings.TrimSpace(defaultModelID) != "" {
			settings.Runtimes[i].DefaultModelID = normalizeModelIdentifier(defaultModelID)
		}
		return
	}
}

func replaceRuntimeModels(settings *RuntimeModelSettings, runtimeID string, models []RuntimeModelConfig, defaultModelID string) {
	if settings == nil {
		return
	}
	for i := range settings.Runtimes {
		if strings.TrimSpace(settings.Runtimes[i].ID) != strings.TrimSpace(runtimeID) {
			continue
		}
		settings.Runtimes[i].Models = append([]RuntimeModelConfig(nil), models...)
		settings.Runtimes[i].DefaultModelID = normalizeModelIdentifier(defaultModelID)
		return
	}
}

func setRuntimeDefaultModel(settings *RuntimeModelSettings, runtimeID, modelID string) {
	if settings == nil {
		return
	}
	for i := range settings.Runtimes {
		if strings.TrimSpace(settings.Runtimes[i].ID) != strings.TrimSpace(runtimeID) {
			continue
		}
		settings.Runtimes[i].DefaultModelID = normalizeModelIdentifier(modelID)
		return
	}
}

func ensureIFLOWSource(cfg *Config, tool CLIToolConfig) string {
	for i := range cfg.APISources {
		if !isIFLOWAPISource(cfg.APISources[i]) {
			continue
		}
		cfg.APISources[i].AuthMode = firstNonEmptyTrimmed(tool.IFlowAuthMode, cfg.APISources[i].AuthMode, IFLOWAuthModeBrowser)
		cfg.APISources[i].BaseURL = firstNonEmptyTrimmed(tool.IFlowBaseURL, cfg.APISources[i].BaseURL, defaultIFLOWBaseURL)
		if strings.TrimSpace(tool.IFlowAPIKey) != "" {
			cfg.APISources[i].APIKey = strings.TrimSpace(tool.IFlowAPIKey)
		}
		return cfg.APISources[i].ID
	}
	id := "iflow-default"
	used := make(map[string]struct{}, len(cfg.APISources))
	for _, item := range cfg.APISources {
		used[strings.TrimSpace(item.ID)] = struct{}{}
	}
	id = nextUniqueSourceID(id, used)
	cfg.APISources = append(cfg.APISources, APISourceConfig{
		ID:       id,
		Label:    "iFlow 官方来源",
		Provider: "iflow",
		BaseURL:  firstNonEmptyTrimmed(tool.IFlowBaseURL, defaultIFLOWBaseURL),
		APIKey:   strings.TrimSpace(tool.IFlowAPIKey),
		AuthMode: firstNonEmptyTrimmed(tool.IFlowAuthMode, IFLOWAuthModeBrowser),
	})
	return id
}

func SetAPISources(cfg *Config, sources []APISourceConfig) error {
	if cfg == nil {
		return errNilConfig
	}
	cfg.APISources = append([]APISourceConfig(nil), sources...)
	return HydrateRuntimeSettings(cfg)
}

func SetRuntimeModelSettings(cfg *Config, settings *RuntimeModelSettings) error {
	if cfg == nil {
		return errNilConfig
	}
	cfg.RuntimeModels = cloneRuntimeSettings(settings)
	return HydrateRuntimeSettings(cfg)
}

func RuntimeModels(cfg Config) []RuntimeModelRuntimeConfig {
	if cfg.RuntimeModels == nil {
		return nil
	}
	out := make([]RuntimeModelRuntimeConfig, 0, len(cfg.RuntimeModels.Runtimes))
	for _, runtime := range cfg.RuntimeModels.Runtimes {
		item := runtime
		item.Models = append([]RuntimeModelConfig(nil), runtime.Models...)
		out = append(out, item)
	}
	return out
}

func cloneRuntimeSettings(settings *RuntimeModelSettings) *RuntimeModelSettings {
	if settings == nil {
		return nil
	}
	out := &RuntimeModelSettings{Runtimes: make([]RuntimeModelRuntimeConfig, 0, len(settings.Runtimes))}
	for _, runtime := range settings.Runtimes {
		item := runtime
		item.Models = append([]RuntimeModelConfig(nil), runtime.Models...)
		out.Runtimes = append(out.Runtimes, item)
	}
	return out
}
func splitRuntimeModelIdentifier(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if !strings.Contains(value, ":") {
		return "", normalizeModelIdentifier(value)
	}
	runtimeID, modelID, _ := strings.Cut(value, ":")
	return strings.TrimSpace(runtimeID), normalizeModelIdentifier(modelID)
}

func normalizeRuntimeModelIdentifier(value string) string {
	runtimeID, modelID := splitRuntimeModelIdentifier(value)
	if runtimeID == "" {
		return modelID
	}
	if modelID == "" {
		return ""
	}
	return strings.TrimSpace(runtimeID) + ":" + modelID
}
