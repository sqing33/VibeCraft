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
// 副作用：会规范化 `model_id`，并清理历史遗留字段。
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
		tt.SourceID = ""
		tt.Model = ""
		tt.TargetModelIDs = nil
		if tt.ModelID == "" {
			(*basic).ThinkingTranslation = nil
		}
	}
	if (*basic).ThinkingTranslation == nil {
		*basic = nil
	}
}

// ValidateBasicSettings 功能：校验 basic settings 的结构与 LLM 引用关系。
// 参数/返回：basic 为当前设置；llm 为兼容镜像。返回 error 表示不合法。
// 失败场景：翻译模型不存在或 provider 不受支持时返回 error。
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
	return nil
}

// ValidateBasicSettingsWithRuntime 功能：校验 basic settings 与 runtime model settings 的引用关系。
// 参数/返回：basic 为当前设置；cfg 为完整配置；返回 error 表示不合法。
// 失败场景：翻译模型不存在、不是 SDK runtime，或 provider 不受支持时返回 error。
// 副作用：无。
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
	return nil
}

// ReconcileBasicSettingsWithLLM 功能：在 LLM settings 变化后自动修复或清空失效的 basic settings 引用。
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
	NormalizeBasicSettings(basic)
}

// ReconcileBasicSettingsWithRuntime 功能：在 runtime model settings 变化后自动修复或清空失效的 basic settings 引用。
// 参数/返回：basic 为当前 basic settings 指针；cfg 为完整配置；无返回值。
// 失败场景：无（失效配置会被自动清空而非报错）。
// 副作用：会原地修改 `thinking_translation`，必要时置空整个 basic settings。
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
	NormalizeBasicSettings(basic)
}

// ResolveThinkingTranslation 功能：根据 basic settings 生成当前 turn 可用的翻译运行时配置。
// 参数/返回：basic 为基本设置；llm 为 LLM 配置；命中时返回运行时配置，否则返回 nil。
// 失败场景：basic settings 结构非法时返回 error。
// 副作用：无。
func ResolveThinkingTranslation(basic *BasicSettings, llm *LLMSettings) (*ThinkingTranslationRuntime, error) {
	copyValue := cloneBasicSettings(basic)
	NormalizeBasicSettings(&copyValue)
	if copyValue == nil || copyValue.ThinkingTranslation == nil {
		return nil, nil
	}
	if err := ValidateBasicSettings(copyValue, llm); err != nil {
		return nil, err
	}
	tt := copyValue.ThinkingTranslation
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

// ResolveThinkingTranslationWithRuntime 功能：根据 runtime model settings 生成当前 turn 可用的翻译运行时配置。
// 参数/返回：basic 为基本设置；cfg 为完整配置；命中时返回运行时配置，否则返回 nil。
// 失败场景：basic settings 结构非法时返回 error。
// 副作用：无。
func ResolveThinkingTranslationWithRuntime(basic *BasicSettings, cfg Config) (*ThinkingTranslationRuntime, error) {
	copyValue := cloneBasicSettings(basic)
	NormalizeBasicSettings(&copyValue)
	if copyValue == nil || copyValue.ThinkingTranslation == nil {
		return nil, nil
	}
	if err := ValidateBasicSettingsWithRuntime(copyValue, cfg); err != nil {
		return nil, err
	}
	tt := copyValue.ThinkingTranslation
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
