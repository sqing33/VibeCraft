package config

import (
	"errors"
	"strings"
)

var errNilConfig = errors.New("config is nil")

// MirrorLLMToExperts 功能：将 cfg.LLM 的 models/sources 映射为可执行的 experts 条目（写入/覆盖 cfg.Experts）。
// 参数/返回：cfg 必填；成功返回 nil。
// 失败场景：llm settings 校验失败时返回 error。
// 副作用：原地修改 cfg.Experts。
func MirrorLLMToExperts(cfg *Config) error {
	if cfg == nil {
		return errNilConfig
	}
	if cfg.LLM == nil {
		return nil
	}
	if err := NormalizeLLMSettings(cfg.LLM); err != nil {
		return err
	}

	sourceByID := make(map[string]LLMSourceConfig, len(cfg.LLM.Sources))
	for _, s := range cfg.LLM.Sources {
		sourceByID[strings.TrimSpace(s.ID)] = s
	}

	modelIDs := make(map[string]struct{}, len(cfg.LLM.Models))
	for _, m := range cfg.LLM.Models {
		modelIDs[strings.TrimSpace(m.ID)] = struct{}{}
	}

	existingByID := make(map[string]ExpertConfig, len(cfg.Experts))
	for _, e := range cfg.Experts {
		existingByID[strings.TrimSpace(e.ID)] = e
	}

	out := make([]ExpertConfig, 0, len(cfg.Experts)+len(cfg.LLM.Models))
	for _, e := range cfg.Experts {
		if _, ok := modelIDs[strings.TrimSpace(e.ID)]; ok {
			continue
		}
		out = append(out, e)
	}

	for _, m := range cfg.LLM.Models {
		id := strings.TrimSpace(m.ID)
		base := existingByID[id]
		base.ID = id

		if label := strings.TrimSpace(m.Label); label != "" {
			base.Label = label
		}

		provider := normalizeProvider(m.Provider)
		base.Provider = provider
		base.Model = strings.TrimSpace(m.Model)

		src := sourceByID[strings.TrimSpace(m.SourceID)]
		base.BaseURL = strings.TrimSpace(src.BaseURL)

		// LLM provider 禁止 legacy CLI 字段悄悄生效：镜像时统一清空。
		base.RunMode = ""
		base.Command = ""
		base.Args = nil

		if base.Env == nil {
			base.Env = map[string]string{}
		}
		switch provider {
		case "openai":
			base.Env["OPENAI_API_KEY"] = strings.TrimSpace(src.APIKey)
		case "anthropic":
			base.Env["ANTHROPIC_API_KEY"] = strings.TrimSpace(src.APIKey)
		}

		// 约束：对新建 LLM expert，提供一个与默认值一致的超时兜底。
		if base.TimeoutMs <= 0 {
			base.TimeoutMs = 30 * 60 * 1000
		}

		out = append(out, base)
	}

	cfg.Experts = out
	return nil
}
