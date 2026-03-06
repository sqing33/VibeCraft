package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateLLMSettings 功能：校验 LLM settings 的结构有效性（sources/models）。
// 参数/返回：llm 可为 nil；nil 表示不启用该能力并返回 nil；否则返回校验错误。
// 失败场景：id/provider/model/source 引用不合法，或模型标识在小写归一化后冲突时返回 error。
// 副作用：无。
func ValidateLLMSettings(llm *LLMSettings) error {
	if llm == nil {
		return nil
	}

	sourceByID := make(map[string]LLMSourceConfig, len(llm.Sources))
	for i := range llm.Sources {
		s := llm.Sources[i]
		id := strings.TrimSpace(s.ID)
		if id == "" {
			return fmt.Errorf("llm.sources[%d].id is required", i)
		}
		if _, ok := sourceByID[id]; ok {
			return fmt.Errorf("llm.sources[%d].id %q is duplicated", i, id)
		}

		provider := normalizeProvider(s.Provider)
		if provider != "" && provider != "openai" && provider != "anthropic" {
			return fmt.Errorf("llm.sources[%d].provider %q is not supported", i, strings.TrimSpace(s.Provider))
		}

		if u := strings.TrimSpace(s.BaseURL); u != "" {
			parsed, err := url.Parse(u)
			if err != nil || parsed == nil {
				return fmt.Errorf("llm.sources[%d].base_url is invalid", i)
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				return fmt.Errorf("llm.sources[%d].base_url must start with http:// or https://", i)
			}
		}

		sourceByID[id] = LLMSourceConfig{
			ID:       id,
			Label:    strings.TrimSpace(s.Label),
			Provider: provider,
			BaseURL:  strings.TrimSpace(s.BaseURL),
			APIKey:   strings.TrimSpace(s.APIKey),
		}
	}

	modelSeen := make(map[string]struct{}, len(llm.Models))
	for i := range llm.Models {
		m := llm.Models[i]
		id := normalizeModelIdentifier(m.ID)
		if id == "" {
			return fmt.Errorf("llm.models[%d].id is required", i)
		}
		if _, ok := modelSeen[id]; ok {
			return fmt.Errorf("llm.models[%d].id %q is duplicated", i, id)
		}
		modelSeen[id] = struct{}{}

		provider := normalizeProvider(m.Provider)
		if provider == "" {
			return fmt.Errorf("llm.models[%d].provider is required", i)
		}
		if provider != "openai" && provider != "anthropic" {
			return fmt.Errorf("llm.models[%d].provider %q is not supported", i, strings.TrimSpace(m.Provider))
		}

		model := normalizeModelIdentifier(m.Model)
		if model == "" {
			return fmt.Errorf("llm.models[%d].model is required", i)
		}

		style := normalizeOpenAIAPIStyle(m.OpenAIAPIStyle)
		if style != "" && provider != "openai" {
			return fmt.Errorf("llm.models[%d].openai_api_style is only supported for openai models", i)
		}
		if strings.TrimSpace(m.OpenAIAPIStyle) != "" && style == "" {
			return fmt.Errorf("llm.models[%d].openai_api_style %q is not supported", i, strings.TrimSpace(m.OpenAIAPIStyle))
		}

		sourceID := strings.TrimSpace(m.SourceID)
		if sourceID == "" {
			return fmt.Errorf("llm.models[%d].source_id is required", i)
		}
		src, ok := sourceByID[sourceID]
		if !ok {
			return fmt.Errorf("llm.models[%d].source_id %q does not exist", i, sourceID)
		}
		_ = src // source existence already validated; source may be used by different providers.
	}

	return nil
}

// NormalizeLLMSettings 功能：在校验通过后，规范化 LLM settings 的 provider 与模型标识字段。
// 参数/返回：llm 可为 nil；成功返回 nil。
// 失败场景：结构非法或归一化后出现冲突时返回 error。
// 副作用：会原地 trim 并小写化 llm.Sources.provider 与 llm.Models 的 id/provider/model/source_id 字段。
func NormalizeLLMSettings(llm *LLMSettings) error {
	if llm == nil {
		return nil
	}
	if err := ValidateLLMSettings(llm); err != nil {
		return err
	}

	for i := range llm.Sources {
		llm.Sources[i].ID = strings.TrimSpace(llm.Sources[i].ID)
		llm.Sources[i].Label = strings.TrimSpace(llm.Sources[i].Label)
		llm.Sources[i].Provider = normalizeProvider(llm.Sources[i].Provider)
		llm.Sources[i].BaseURL = strings.TrimSpace(llm.Sources[i].BaseURL)
		llm.Sources[i].APIKey = strings.TrimSpace(llm.Sources[i].APIKey)
	}

	for i := range llm.Models {
		llm.Models[i].ID = normalizeModelIdentifier(llm.Models[i].ID)
		llm.Models[i].Label = strings.TrimSpace(llm.Models[i].Label)
		llm.Models[i].Provider = normalizeProvider(llm.Models[i].Provider)
		llm.Models[i].Model = normalizeModelIdentifier(llm.Models[i].Model)
		llm.Models[i].SourceID = strings.TrimSpace(llm.Models[i].SourceID)
		llm.Models[i].OpenAIAPIStyle = normalizeOpenAIAPIStyle(llm.Models[i].OpenAIAPIStyle)
		if llm.Models[i].OpenAIAPIStyle == "" {
			llm.Models[i].OpenAIAPIStyleDetectedAt = 0
		}
	}

	return nil
}

func normalizeProvider(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeModelIdentifier(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeOpenAIAPIStyle(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "responses":
		return OpenAIAPIStyleResponses
	case "chat_completions":
		return OpenAIAPIStyleChatCompletions
	default:
		return ""
	}
}
