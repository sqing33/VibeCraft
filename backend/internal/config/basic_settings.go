package config

import (
	"fmt"
	"strings"
)

type ThinkingTranslationRuntime struct {
	SourceID       string
	Provider       string
	BaseURL        string
	APIKey         string
	Model          string
	OpenAIAPIStyle string
}

// NormalizeBasicSettings 功能：规范化 basic settings，并在配置为空时清理空容器。
// 参数/返回：basic 为可空双指针；原地修改。
// 失败场景：无。
// 副作用：会 trim 并去重模型 ID。
func NormalizeBasicSettings(basic **BasicSettings) {
	if basic == nil || *basic == nil {
		return
	}
	if (*basic).ThinkingTranslation != nil {
		tt := (*basic).ThinkingTranslation
		tt.ModelID = normalizeModelIdentifier(tt.ModelID)
		if tt.ModelID == "" {
			tt.ModelID = normalizeModelIdentifier(tt.Model)
		}
		tt.TargetModelIDs = normalizeTargetModelIDList(tt.TargetModelIDs)
		if len(tt.TargetModelIDs) == 0 {
			(*basic).ThinkingTranslation = nil
		}
	}
	if (*basic).ThinkingTranslation == nil {
		*basic = nil
	}
}

// ValidateBasicSettings 功能：校验 basic settings 的结构与 LLM 引用关系。
// 参数/返回：basic 为当前设置；llm 为兼容镜像。返回 error 表示不合法。
// 失败场景：翻译模型或目标模型不存在时返回 error。
// 副作用：无。
func ValidateBasicSettings(basic *BasicSettings, llm *LLMSettings) error {
	copyValue := cloneBasicSettings(basic)
	NormalizeBasicSettings(&copyValue)
	if copyValue == nil || copyValue.ThinkingTranslation == nil {
		return nil
	}
	tt := copyValue.ThinkingTranslation
	if tt.ModelID == "" {
		return fmt.Errorf("basic.thinking_translation.model_id is required")
	}
	if len(tt.TargetModelIDs) == 0 {
		return fmt.Errorf("basic.thinking_translation.target_model_ids must not be empty")
	}
	if llm == nil {
		return fmt.Errorf("llm settings are required for thinking translation")
	}
	modelCfg, _, _, ok := FindLLMModelByID(llm, tt.ModelID)
	if !ok {
		return fmt.Errorf("basic.thinking_translation.model_id %q does not exist in llm models", tt.ModelID)
	}
	provider := normalizeProvider(modelCfg.Provider)
	if provider != ProviderOpenAI && provider != ProviderAnthropic {
		return fmt.Errorf("basic.thinking_translation.model_id %q references unsupported provider %q", tt.ModelID, strings.TrimSpace(modelCfg.Provider))
	}
	modelIDs := llmModelIDSet(llm)
	for _, modelID := range tt.TargetModelIDs {
		if _, ok := modelIDs[modelID]; !ok {
			return fmt.Errorf("basic.thinking_translation.target_model_ids contains unknown model %q", modelID)
		}
	}
	return nil
}

func ValidateBasicSettingsWithRuntime(basic *BasicSettings, cfg Config) error {
	copyValue := cloneBasicSettings(basic)
	NormalizeBasicSettings(&copyValue)
	if copyValue == nil || copyValue.ThinkingTranslation == nil {
		return nil
	}
	tt := copyValue.ThinkingTranslation
	if tt.ModelID == "" {
		return fmt.Errorf("basic.thinking_translation.model_id is required")
	}
	if len(tt.TargetModelIDs) == 0 {
		return fmt.Errorf("basic.thinking_translation.target_model_ids must not be empty")
	}
	runtime, modelCfg, _, ok := FindRuntimeModelByID(cfg, tt.ModelID)
	if !ok {
		return fmt.Errorf("basic.thinking_translation.model_id %q does not exist", tt.ModelID)
	}
	if runtime.Kind != RuntimeKindSDK {
		return fmt.Errorf("basic.thinking_translation.model_id %q must reference an SDK runtime model", tt.ModelID)
	}
	provider := normalizeProvider(modelCfg.Provider)
	if provider != ProviderOpenAI && provider != ProviderAnthropic {
		return fmt.Errorf("basic.thinking_translation.model_id %q references unsupported provider %q", tt.ModelID, strings.TrimSpace(modelCfg.Provider))
	}
	modelIDs := runtimeModelIDSet(cfg)
	for _, modelID := range tt.TargetModelIDs {
		if _, ok := modelIDs[modelID]; !ok {
			return fmt.Errorf("basic.thinking_translation.target_model_ids contains unknown model %q", modelID)
		}
	}
	return nil
}

// ReconcileBasicSettingsWithLLM 功能：在 LLM settings 变化后自动裁剪/清空失效的 basic settings 引用。
// 参数/返回：basic 为当前 basic settings 指针；llm 为最新 LLM settings；无返回值。
// 失败场景：无（失效配置会被自动清空而非报错）。
// 副作用：会原地修改 `thinking_translation`，必要时置空整个 basic settings。
func ReconcileBasicSettingsWithLLM(basic **BasicSettings, llm *LLMSettings) {
	NormalizeBasicSettings(basic)
	if basic == nil || *basic == nil || (*basic).ThinkingTranslation == nil {
		return
	}
	if llm == nil {
		*basic = nil
		return
	}
	tt := (*basic).ThinkingTranslation
	modelCfg, _, _, ok := FindLLMModelByID(llm, tt.ModelID)
	if !ok {
		*basic = nil
		return
	}
	provider := normalizeProvider(modelCfg.Provider)
	if provider != ProviderOpenAI && provider != ProviderAnthropic {
		*basic = nil
		return
	}
	modelIDs := llmModelIDSet(llm)
	filtered := make([]string, 0, len(tt.TargetModelIDs))
	for _, modelID := range tt.TargetModelIDs {
		if _, ok := modelIDs[modelID]; ok {
			filtered = append(filtered, modelID)
		}
	}
	tt.TargetModelIDs = filtered
	if len(tt.TargetModelIDs) == 0 {
		*basic = nil
		return
	}
	NormalizeBasicSettings(basic)
}

func ReconcileBasicSettingsWithRuntime(basic **BasicSettings, cfg Config) {
	NormalizeBasicSettings(basic)
	if basic == nil || *basic == nil || (*basic).ThinkingTranslation == nil {
		return
	}
	tt := (*basic).ThinkingTranslation
	runtime, modelCfg, _, ok := FindRuntimeModelByID(cfg, tt.ModelID)
	if !ok || runtime.Kind != RuntimeKindSDK {
		*basic = nil
		return
	}
	provider := normalizeProvider(modelCfg.Provider)
	if provider != ProviderOpenAI && provider != ProviderAnthropic {
		*basic = nil
		return
	}
	modelIDs := runtimeModelIDSet(cfg)
	filtered := make([]string, 0, len(tt.TargetModelIDs))
	for _, modelID := range tt.TargetModelIDs {
		if _, ok := modelIDs[normalizeTargetModelID(modelID)]; ok {
			filtered = append(filtered, normalizeTargetModelID(modelID))
		}
	}
	tt.TargetModelIDs = filtered
	if len(tt.TargetModelIDs) == 0 {
		*basic = nil
		return
	}
	NormalizeBasicSettings(basic)
}

// ResolveThinkingTranslation 功能：根据 basic settings 与目标模型 ID 生成当前 turn 可用的翻译运行时配置。
// 参数/返回：targetModelID 为当前 turn 的 primary model id；命中时返回运行时配置，否则返回 nil。
// 失败场景：basic settings 结构非法时返回 error。
// 副作用：无。
func ResolveThinkingTranslation(basic *BasicSettings, llm *LLMSettings, targetModelID string) (*ThinkingTranslationRuntime, error) {
	targetModelID = normalizeModelIdentifier(targetModelID)
	if targetModelID == "" {
		return nil, nil
	}
	copyValue := cloneBasicSettings(basic)
	NormalizeBasicSettings(&copyValue)
	if copyValue == nil || copyValue.ThinkingTranslation == nil {
		return nil, nil
	}
	if err := ValidateBasicSettings(copyValue, llm); err != nil {
		return nil, err
	}
	tt := copyValue.ThinkingTranslation
	if !containsNormalizedTarget(tt.TargetModelIDs, targetModelID) {
		return nil, nil
	}
	modelCfg, source, _, _ := FindLLMModelByID(llm, tt.ModelID)
	return &ThinkingTranslationRuntime{
		SourceID:       strings.TrimSpace(source.ID),
		Provider:       normalizeProvider(modelCfg.Provider),
		BaseURL:        strings.TrimSpace(source.BaseURL),
		APIKey:         strings.TrimSpace(source.APIKey),
		Model:          strings.TrimSpace(modelCfg.Model),
		OpenAIAPIStyle: modelCfg.OpenAIAPIStyle,
	}, nil
}

func ResolveThinkingTranslationWithRuntime(basic *BasicSettings, cfg Config, targetModelID string) (*ThinkingTranslationRuntime, error) {
	targetModelID = normalizeTargetModelID(targetModelID)
	if targetModelID == "" {
		return nil, nil
	}
	copyValue := cloneBasicSettings(basic)
	NormalizeBasicSettings(&copyValue)
	if copyValue == nil || copyValue.ThinkingTranslation == nil {
		return nil, nil
	}
	if err := ValidateBasicSettingsWithRuntime(copyValue, cfg); err != nil {
		return nil, err
	}
	tt := copyValue.ThinkingTranslation
	if !containsNormalizedTarget(tt.TargetModelIDs, targetModelID) {
		return nil, nil
	}
	_, modelCfg, source, _ := FindRuntimeModelByID(cfg, tt.ModelID)
	return &ThinkingTranslationRuntime{
		SourceID:       strings.TrimSpace(source.ID),
		Provider:       normalizeProvider(modelCfg.Provider),
		BaseURL:        strings.TrimSpace(source.BaseURL),
		APIKey:         strings.TrimSpace(source.APIKey),
		Model:          strings.TrimSpace(modelCfg.Model),
		OpenAIAPIStyle: modelCfg.OpenAIAPIStyle,
	}, nil
}

func cloneBasicSettings(basic *BasicSettings) *BasicSettings {
	if basic == nil {
		return nil
	}
	out := &BasicSettings{}
	if basic.ThinkingTranslation != nil {
		tt := *basic.ThinkingTranslation
		tt.TargetModelIDs = append([]string(nil), basic.ThinkingTranslation.TargetModelIDs...)
		out.ThinkingTranslation = &tt
	}
	return out
}

func llmSourceByID(llm *LLMSettings) map[string]LLMSourceConfig {
	if llm == nil {
		return nil
	}
	out := make(map[string]LLMSourceConfig, len(llm.Sources))
	for _, source := range llm.Sources {
		out[strings.TrimSpace(source.ID)] = source
	}
	return out
}

func llmModelIDSet(llm *LLMSettings) map[string]struct{} {
	if llm == nil {
		return nil
	}
	out := make(map[string]struct{}, len(llm.Models))
	for _, model := range llm.Models {
		id := normalizeModelIdentifier(model.ID)
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func runtimeModelIDSet(cfg Config) map[string]struct{} {
	out := map[string]struct{}{}
	for _, runtime := range RuntimeModels(cfg) {
		for _, model := range runtime.Models {
			id := normalizeModelIdentifier(model.ID)
			if id == "" {
				continue
			}
			out[id] = struct{}{}
			out[normalizeModelIdentifier(runtime.ID+":"+id)] = struct{}{}
		}
	}
	return out
}

func normalizeTargetModelIDList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeTargetModelID(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeTargetModelID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, ":") {
		return normalizeModelIdentifier(value)
	}
	return normalizeModelIdentifier(value)
}

func containsNormalizedTarget(values []string, target string) bool {
	target = normalizeTargetModelID(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if normalizeTargetModelID(value) == target {
			return true
		}
	}
	return false
}
