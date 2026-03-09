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
// 参数/返回：basic 为 `*BasicSettings` 指针；原地 trim/dedupe 后无返回值。
// 失败场景：无。
// 副作用：会原地修改字段，并在配置为空时置 nil。
func NormalizeBasicSettings(basic **BasicSettings) {
	if basic == nil || *basic == nil {
		return
	}
	if (*basic).ThinkingTranslation != nil {
		tt := (*basic).ThinkingTranslation
		tt.ModelID = strings.TrimSpace(tt.ModelID)
		tt.SourceID = ""
		tt.Model = ""
		tt.TargetModelIDs = normalizeModelIDList(tt.TargetModelIDs)
		if tt.ModelID == "" && len(tt.TargetModelIDs) == 0 {
			(*basic).ThinkingTranslation = nil
		}
	}
	if (*basic).ThinkingTranslation == nil {
		*basic = nil
	}
}

// ValidateBasicSettings 功能：校验 basic settings 的结构与 LLM 引用关系。
// 参数/返回：basic 可为 nil；llm 为当前 LLM settings；成功返回 nil。
// 失败场景：model_id/target_model_ids 不合法时返回 error。
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
	modelCfg, source, _, ok := FindLLMModelByID(llm, tt.ModelID)
	if !ok {
		return fmt.Errorf("basic.thinking_translation.model_id %q does not exist in llm models", tt.ModelID)
	}
	_ = modelCfg
	provider := normalizeProvider(source.Provider)
	if provider != "openai" && provider != "anthropic" {
		return fmt.Errorf("basic.thinking_translation.model_id %q references unsupported provider %q", tt.ModelID, strings.TrimSpace(source.Provider))
	}
	modelIDs := llmModelIDSet(llm)
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
	_, source, _, ok := FindLLMModelByID(llm, tt.ModelID)
	if !ok {
		*basic = nil
		return
	}
	provider := normalizeProvider(source.Provider)
	if provider != "openai" && provider != "anthropic" {
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
	hit := false
	for _, modelID := range tt.TargetModelIDs {
		if modelID == targetModelID {
			hit = true
			break
		}
	}
	if !hit {
		return nil, nil
	}
	modelCfg, source, _, _ := FindLLMModelByID(llm, tt.ModelID)
	return &ThinkingTranslationRuntime{
		SourceID:       strings.TrimSpace(source.ID),
		Provider:       normalizeProvider(source.Provider),
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

func normalizeModelIDList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeModelIdentifier(value)
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
