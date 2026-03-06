package config

import (
	"errors"
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
	return RebuildExperts(cfg)
}
