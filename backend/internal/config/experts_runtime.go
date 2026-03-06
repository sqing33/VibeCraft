package config

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ManagedSourceBuiltin       = "builtin"
	ManagedSourceLLMModel      = "llm-model"
	ManagedSourceExpertProfile = "expert-profile"
)

var supportedFallbackOn = map[string]struct{}{
	"request_error": {},
	"timeout":       {},
	"rate_limit":    {},
	"provider_5xx":  {},
	"network_error": {},
}

// RebuildExperts 功能：重建运行时 experts，保留 builtin / custom experts，并根据 llm settings 生成 llm-model experts。
// 参数/返回：cfg 为完整配置；成功时原地更新 cfg.Experts。
// 失败场景：模型引用缺失、expert id 冲突或结构非法时返回 error。
// 副作用：修改 cfg.Experts 中的 provider/model/base_url/env 等派生字段。
func RebuildExperts(cfg *Config) error {
	if cfg == nil {
		return errNilConfig
	}
	if err := NormalizeLLMSettings(cfg.LLM); err != nil {
		return err
	}

	existingByID := make(map[string]ExpertConfig, len(cfg.Experts))
	for _, e := range cfg.Experts {
		existingByID[strings.TrimSpace(e.ID)] = e
	}

	outByID := make(map[string]ExpertConfig, len(cfg.Experts)+len(llmModels(cfg.LLM)))
	order := make([]string, 0, len(cfg.Experts)+len(llmModels(cfg.LLM)))
	put := func(e ExpertConfig) {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			return
		}
		e.ID = id
		if _, ok := outByID[id]; !ok {
			order = append(order, id)
		}
		outByID[id] = e
	}

	for _, raw := range cfg.Experts {
		if strings.EqualFold(inferManagedSource(raw), ManagedSourceLLMModel) {
			continue
		}
		hydrated, err := hydrateExpert(raw, cfg.LLM)
		if err != nil {
			return err
		}
		put(hydrated)
	}

	if cfg.LLM != nil {
		sourceByID := make(map[string]LLMSourceConfig, len(cfg.LLM.Sources))
		for _, s := range cfg.LLM.Sources {
			sourceByID[strings.TrimSpace(s.ID)] = s
		}
		for _, m := range cfg.LLM.Models {
			id := strings.TrimSpace(m.ID)
			base := existingByID[id]
			base.ID = id
			base.ManagedSource = ManagedSourceLLMModel
			base.PrimaryModelID = id
			base.SecondaryModelID = ""
			base.SecondaryProvider = ""
			base.SecondaryModel = ""
			base.SecondaryBaseURL = ""
			base.SecondaryEnv = nil
			base.FallbackOn = nil
			if strings.TrimSpace(base.Label) == "" {
				base.Label = strings.TrimSpace(m.Label)
			}
			if strings.TrimSpace(base.Label) == "" {
				base.Label = id
			}
			base.Description = strings.TrimSpace(base.Description)
			base.Provider = normalizeProvider(m.Provider)
			base.Model = strings.TrimSpace(m.Model)
			base.BaseURL = strings.TrimSpace(sourceByID[strings.TrimSpace(m.SourceID)].BaseURL)
			base.Env = mergeProviderEnv(base.Env, base.Provider, sourceByID[strings.TrimSpace(m.SourceID)].APIKey)
			if base.SystemPrompt == "" {
				base.SystemPrompt = strings.TrimSpace(m.SystemPrompt)
			}
			if base.MaxOutputTokens <= 0 {
				base.MaxOutputTokens = m.MaxOutputTokens
			}
			if base.Temperature == nil && m.Temperature != nil {
				v := *m.Temperature
				base.Temperature = &v
			}
			if strings.TrimSpace(base.OutputSchema) == "" {
				base.OutputSchema = strings.TrimSpace(m.OutputSchema)
			}
			if base.TimeoutMs <= 0 {
				base.TimeoutMs = m.TimeoutMs
			}
			if base.TimeoutMs <= 0 {
				base.TimeoutMs = 30 * 60 * 1000
			}
			put(normalizeExpert(base))
		}
	}

	sort.Strings(order)
	out := make([]ExpertConfig, 0, len(order))
	for _, id := range order {
		out = append(out, outByID[id])
	}
	if err := validateExpertList(out); err != nil {
		return err
	}
	cfg.Experts = out
	return nil
}

func llmModels(llm *LLMSettings) []LLMModelConfig {
	if llm == nil {
		return nil
	}
	return llm.Models
}

func hydrateExpert(raw ExpertConfig, llm *LLMSettings) (ExpertConfig, error) {
	e := normalizeExpert(raw)
	if e.ManagedSource == ManagedSourceExpertProfile && strings.TrimSpace(e.PrimaryModelID) == "" && strings.TrimSpace(e.Provider) == "" {
		return ExpertConfig{}, fmt.Errorf("expert %q: primary_model_id is required", e.ID)
	}
	if strings.TrimSpace(e.PrimaryModelID) == "" || llm == nil {
		return e, nil
	}
	primaryModel, primarySource, err := lookupModel(llm, e.PrimaryModelID)
	if err != nil {
		return ExpertConfig{}, fmt.Errorf("expert %q: %w", e.ID, err)
	}
	e.ManagedSource = ManagedSourceExpertProfile
	e.Provider = normalizeProvider(primaryModel.Provider)
	e.Model = strings.TrimSpace(primaryModel.Model)
	e.BaseURL = strings.TrimSpace(primarySource.BaseURL)
	e.Env = mergeProviderEnv(e.Env, e.Provider, primarySource.APIKey)
	if strings.TrimSpace(e.SystemPrompt) == "" {
		e.SystemPrompt = strings.TrimSpace(primaryModel.SystemPrompt)
	}
	if e.MaxOutputTokens <= 0 {
		e.MaxOutputTokens = primaryModel.MaxOutputTokens
	}
	if e.Temperature == nil && primaryModel.Temperature != nil {
		v := *primaryModel.Temperature
		e.Temperature = &v
	}
	if strings.TrimSpace(e.OutputSchema) == "" {
		e.OutputSchema = strings.TrimSpace(primaryModel.OutputSchema)
	}
	if e.TimeoutMs <= 0 {
		e.TimeoutMs = primaryModel.TimeoutMs
	}
	if e.TimeoutMs <= 0 {
		e.TimeoutMs = 30 * 60 * 1000
	}

	if strings.TrimSpace(e.SecondaryModelID) != "" {
		secondaryModel, secondarySource, err := lookupModel(llm, e.SecondaryModelID)
		if err != nil {
			return ExpertConfig{}, fmt.Errorf("expert %q: %w", e.ID, err)
		}
		e.SecondaryProvider = normalizeProvider(secondaryModel.Provider)
		e.SecondaryModel = strings.TrimSpace(secondaryModel.Model)
		e.SecondaryBaseURL = strings.TrimSpace(secondarySource.BaseURL)
		e.SecondaryEnv = mergeProviderEnv(e.SecondaryEnv, e.SecondaryProvider, secondarySource.APIKey)
		if len(e.FallbackOn) == 0 {
			e.FallbackOn = []string{"request_error"}
		}
	} else {
		e.SecondaryProvider = ""
		e.SecondaryModel = ""
		e.SecondaryBaseURL = ""
		e.SecondaryEnv = nil
		e.FallbackOn = normalizeFallbackOn(nil)
	}
	return normalizeExpert(e), nil
}

func normalizeExpert(e ExpertConfig) ExpertConfig {
	e.ID = strings.TrimSpace(e.ID)
	e.Label = strings.TrimSpace(e.Label)
	e.Description = strings.TrimSpace(e.Description)
	e.Category = strings.TrimSpace(e.Category)
	e.Avatar = strings.TrimSpace(e.Avatar)
	e.ManagedSource = inferManagedSource(e)
	e.PrimaryModelID = strings.TrimSpace(e.PrimaryModelID)
	e.SecondaryModelID = strings.TrimSpace(e.SecondaryModelID)
	e.Provider = normalizeProvider(e.Provider)
	e.Model = strings.TrimSpace(e.Model)
	e.BaseURL = strings.TrimSpace(e.BaseURL)
	e.SecondaryProvider = normalizeProvider(e.SecondaryProvider)
	e.SecondaryModel = strings.TrimSpace(e.SecondaryModel)
	e.SecondaryBaseURL = strings.TrimSpace(e.SecondaryBaseURL)
	e.SystemPrompt = strings.TrimSpace(e.SystemPrompt)
	e.PromptTemplate = strings.TrimSpace(e.PromptTemplate)
	e.OutputSchema = strings.TrimSpace(e.OutputSchema)
	e.OutputFormat = strings.TrimSpace(e.OutputFormat)
	e.BuilderExpertID = strings.TrimSpace(e.BuilderExpertID)
	e.BuilderSessionID = strings.TrimSpace(e.BuilderSessionID)
	e.BuilderSnapshotID = strings.TrimSpace(e.BuilderSnapshotID)
	e.GeneratedBy = strings.TrimSpace(e.GeneratedBy)
	e.FallbackOn = normalizeFallbackOn(e.FallbackOn)
	e.EnabledSkills = uniqueTrimmed(e.EnabledSkills)
	if e.Label == "" {
		e.Label = e.ID
	}
	if e.Env == nil {
		e.Env = map[string]string{}
	}
	if e.SecondaryEnv == nil && e.SecondaryModel != "" {
		e.SecondaryEnv = map[string]string{}
	}
	return e
}

func inferManagedSource(e ExpertConfig) string {
	s := strings.TrimSpace(strings.ToLower(e.ManagedSource))
	switch s {
	case ManagedSourceBuiltin, ManagedSourceLLMModel, ManagedSourceExpertProfile:
		return s
	}
	if strings.TrimSpace(e.PrimaryModelID) != "" || strings.TrimSpace(e.SecondaryModelID) != "" || len(e.EnabledSkills) > 0 || strings.TrimSpace(e.Description) != "" || strings.TrimSpace(e.Category) != "" {
		return ManagedSourceExpertProfile
	}
	if strings.TrimSpace(e.ID) == "master" || strings.TrimSpace(e.ID) == "bash" || strings.TrimSpace(e.ID) == "demo" || strings.TrimSpace(e.ID) == "codex" || strings.TrimSpace(e.ID) == "claudecode" {
		return ManagedSourceBuiltin
	}
	if p := normalizeProvider(e.Provider); p == "process" || p == "demo" {
		return ManagedSourceBuiltin
	}
	return ManagedSourceBuiltin
}

func normalizeFallbackOn(values []string) []string {
	return uniqueTrimmed(values)
}

func uniqueTrimmed(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func lookupModel(llm *LLMSettings, modelID string) (LLMModelConfig, LLMSourceConfig, error) {
	if llm == nil {
		return LLMModelConfig{}, LLMSourceConfig{}, fmt.Errorf("llm settings are required")
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return LLMModelConfig{}, LLMSourceConfig{}, fmt.Errorf("model id is required")
	}
	sourceByID := make(map[string]LLMSourceConfig, len(llm.Sources))
	for _, s := range llm.Sources {
		sourceByID[strings.TrimSpace(s.ID)] = s
	}
	for _, m := range llm.Models {
		if strings.TrimSpace(m.ID) != modelID {
			continue
		}
		src, ok := sourceByID[strings.TrimSpace(m.SourceID)]
		if !ok {
			return LLMModelConfig{}, LLMSourceConfig{}, fmt.Errorf("model %q references missing source %q", modelID, m.SourceID)
		}
		return m, src, nil
	}
	return LLMModelConfig{}, LLMSourceConfig{}, fmt.Errorf("model %q does not exist", modelID)
}

func mergeProviderEnv(base map[string]string, provider, apiKey string) map[string]string {
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		if strings.EqualFold(k, "OPENAI_API_KEY") || strings.EqualFold(k, "ANTHROPIC_API_KEY") {
			continue
		}
		out[k] = v
	}
	key := strings.TrimSpace(apiKey)
	switch normalizeProvider(provider) {
	case "openai":
		if key != "" {
			out["OPENAI_API_KEY"] = key
		}
	case "anthropic":
		if key != "" {
			out["ANTHROPIC_API_KEY"] = key
		}
	}
	return out
}

func validateExpertList(experts []ExpertConfig) error {
	seen := make(map[string]struct{}, len(experts))
	for i, e := range experts {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			return fmt.Errorf("experts[%d].id is required", i)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("experts[%d].id %q is duplicated", i, id)
		}
		seen[id] = struct{}{}
		provider := normalizeProvider(e.Provider)
		switch provider {
		case "", "openai", "anthropic", "demo", "process":
		default:
			return fmt.Errorf("experts[%d].provider %q is not supported", i, e.Provider)
		}
		if provider != "" && provider != "demo" && provider != "process" && strings.TrimSpace(e.Model) == "" {
			return fmt.Errorf("experts[%d].model is required", i)
		}
		for _, fallback := range e.FallbackOn {
			if _, ok := supportedFallbackOn[fallback]; !ok {
				return fmt.Errorf("experts[%d].fallback_on contains unsupported value %q", i, fallback)
			}
		}
	}
	return nil
}
