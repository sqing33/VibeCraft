package config

import "strings"

const (
	OpenAIAPIStyleResponses       = "responses"
	OpenAIAPIStyleChatCompletions = "chat_completions"
)

// FindLLMModelByID 功能：按模型 ID 查找 llm.models 条目及其 source。
// 参数/返回：llm 可为 nil；modelID 为归一化前后的任意输入；命中时返回 model/source/index/true。
// 失败场景：无（未命中返回 ok=false）。
// 副作用：无。
func FindLLMModelByID(llm *LLMSettings, modelID string) (LLMModelConfig, LLMSourceConfig, int, bool) {
	if llm == nil {
		return LLMModelConfig{}, LLMSourceConfig{}, -1, false
	}
	modelID = normalizeModelIdentifier(modelID)
	if modelID == "" {
		return LLMModelConfig{}, LLMSourceConfig{}, -1, false
	}
	sourceByID := make(map[string]LLMSourceConfig, len(llm.Sources))
	for _, source := range llm.Sources {
		sourceByID[strings.TrimSpace(source.ID)] = source
	}
	for i, model := range llm.Models {
		if normalizeModelIdentifier(model.ID) != modelID {
			continue
		}
		source, ok := sourceByID[strings.TrimSpace(model.SourceID)]
		if !ok {
			return LLMModelConfig{}, LLMSourceConfig{}, -1, false
		}
		return model, source, i, true
	}
	return LLMModelConfig{}, LLMSourceConfig{}, -1, false
}

// FindLLMModelByIdentity 功能：按 provider/source/model 组合查找 llm.models 条目及其 source。
// 参数/返回：provider/sourceID/modelName 为匹配条件；命中时返回 model/source/index/true。
// 失败场景：无（未命中返回 ok=false）。
// 副作用：无。
func FindLLMModelByIdentity(llm *LLMSettings, provider, sourceID, modelName string) (LLMModelConfig, LLMSourceConfig, int, bool) {
	if llm == nil {
		return LLMModelConfig{}, LLMSourceConfig{}, -1, false
	}
	provider = normalizeProvider(provider)
	sourceID = strings.TrimSpace(sourceID)
	modelName = normalizeModelIdentifier(modelName)
	if provider == "" || sourceID == "" || modelName == "" {
		return LLMModelConfig{}, LLMSourceConfig{}, -1, false
	}
	sourceByID := make(map[string]LLMSourceConfig, len(llm.Sources))
	for _, source := range llm.Sources {
		sourceByID[strings.TrimSpace(source.ID)] = source
	}
	for i, model := range llm.Models {
		if normalizeProvider(model.Provider) != provider {
			continue
		}
		if strings.TrimSpace(model.SourceID) != sourceID {
			continue
		}
		if normalizeModelIdentifier(model.Model) != modelName {
			continue
		}
		source, ok := sourceByID[sourceID]
		if !ok {
			return LLMModelConfig{}, LLMSourceConfig{}, -1, false
		}
		return model, source, i, true
	}
	return LLMModelConfig{}, LLMSourceConfig{}, -1, false
}

// PreserveOpenAIAPIStyles 功能：在整包更新 LLM settings 时保留未失效的 OpenAI API style 元数据。
// 参数/返回：existing 为旧配置，next 为新配置；会原地修改 next。
// 失败场景：无。
// 副作用：根据模型/source 变化原地保留或清空 `openai_api_style` 与时间戳。
func PreserveOpenAIAPIStyles(existing, next *LLMSettings) {
	if next == nil {
		return
	}
	if existing == nil {
		clearAllOpenAIAPIStyles(next)
		return
	}
	oldSourceByID := make(map[string]LLMSourceConfig, len(existing.Sources))
	newSourceByID := make(map[string]LLMSourceConfig, len(next.Sources))
	for _, source := range existing.Sources {
		oldSourceByID[strings.TrimSpace(source.ID)] = source
	}
	for _, source := range next.Sources {
		newSourceByID[strings.TrimSpace(source.ID)] = source
	}
	oldModelByID := make(map[string]LLMModelConfig, len(existing.Models))
	for _, model := range existing.Models {
		oldModelByID[normalizeModelIdentifier(model.ID)] = model
	}

	for i := range next.Models {
		model := &next.Models[i]
		model.OpenAIAPIStyle = normalizeOpenAIAPIStyle(model.OpenAIAPIStyle)
		if normalizeProvider(model.Provider) != "openai" {
			model.OpenAIAPIStyle = ""
			model.OpenAIAPIStyleDetectedAt = 0
			continue
		}
		oldModel, ok := oldModelByID[normalizeModelIdentifier(model.ID)]
		if !ok {
			model.OpenAIAPIStyle = ""
			model.OpenAIAPIStyleDetectedAt = 0
			continue
		}
		if normalizeProvider(oldModel.Provider) != "openai" || normalizeModelIdentifier(oldModel.Model) != normalizeModelIdentifier(model.Model) || strings.TrimSpace(oldModel.SourceID) != strings.TrimSpace(model.SourceID) {
			model.OpenAIAPIStyle = ""
			model.OpenAIAPIStyleDetectedAt = 0
			continue
		}
		oldSource, okOld := oldSourceByID[strings.TrimSpace(oldModel.SourceID)]
		newSource, okNew := newSourceByID[strings.TrimSpace(model.SourceID)]
		if !okOld || !okNew || !sameSourceConnection(oldSource, newSource) {
			model.OpenAIAPIStyle = ""
			model.OpenAIAPIStyleDetectedAt = 0
			continue
		}
		model.OpenAIAPIStyle = normalizeOpenAIAPIStyle(oldModel.OpenAIAPIStyle)
		if model.OpenAIAPIStyle == "" {
			model.OpenAIAPIStyleDetectedAt = 0
			continue
		}
		model.OpenAIAPIStyleDetectedAt = oldModel.OpenAIAPIStyleDetectedAt
	}
}

func clearAllOpenAIAPIStyles(llm *LLMSettings) {
	if llm == nil {
		return
	}
	for i := range llm.Models {
		llm.Models[i].OpenAIAPIStyle = ""
		llm.Models[i].OpenAIAPIStyleDetectedAt = 0
	}
}

func sameSourceConnection(left, right LLMSourceConfig) bool {
	return strings.TrimSpace(left.BaseURL) == strings.TrimSpace(right.BaseURL) &&
		strings.TrimSpace(left.APIKey) == strings.TrimSpace(right.APIKey)
}
