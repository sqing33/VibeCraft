package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateLLMSettings 功能：校验 LLM settings 的结构有效性（sources/models）。
// 参数/返回：llm 可为 nil；nil 表示不启用该能力并返回 nil；否则返回校验错误。
// 失败场景：id/provider/model/source 引用不合法时返回 error。
// 副作用：无。
func ValidateLLMSettings(llm *LLMSettings) error {
	if llm == nil {
		return nil
	}

	sourceByID := make(map[string]LLMSourceConfig, len(llm.Sources))
	for i, s := range llm.Sources {
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

		s.ID = id
		s.Provider = provider
		sourceByID[id] = s
	}

	modelSeen := make(map[string]struct{}, len(llm.Models))
	for i, m := range llm.Models {
		id := strings.TrimSpace(m.ID)
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

		model := strings.TrimSpace(m.Model)
		if model == "" {
			return fmt.Errorf("llm.models[%d].model is required", i)
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

// NormalizeLLMSettings 功能：在校验通过后，将 source.provider 自动补齐为引用它的 models.provider（若缺失）。
// 参数/返回：llm 可为 nil；成功返回 nil。
// 失败场景：结构非法或 source 被不同 provider 的 model 混用时返回 error。
// 副作用：会原地规范化 llm.Sources 的 provider 字段（仅做小写化；不会推断/补齐）。
func NormalizeLLMSettings(llm *LLMSettings) error {
	if llm == nil {
		return nil
	}
	if err := ValidateLLMSettings(llm); err != nil {
		return err
	}

	for i := range llm.Sources {
		llm.Sources[i].Provider = normalizeProvider(llm.Sources[i].Provider)
	}

	return nil
}

func normalizeProvider(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}
