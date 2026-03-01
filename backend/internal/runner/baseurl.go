package runner

import (
	"net/url"
	"strings"
)

// NormalizeBaseURL 功能：按 provider 规范化 SDK base URL：
// - openai：确保以 `/v1` 结尾（若缺失则追加）
// - anthropic：若以 `/v1` 结尾则移除（Anthropic 不需要 `/v1`）
// 参数/返回：provider 为 "openai"/"anthropic"；raw 为用户配置；返回规范化后的 base URL（尽量保持可用）。
// 失败场景：raw 非法 URL 时回退返回原字符串（best-effort）。
// 副作用：无。
func NormalizeBaseURL(provider, raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	u, err := url.Parse(s)
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return s
	}

	u.Fragment = ""
	u.RawQuery = ""

	path := strings.TrimSuffix(strings.TrimSpace(u.Path), "/")
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		if path == "" {
			path = "/v1"
		} else if !strings.HasSuffix(path, "/v1") {
			path = path + "/v1"
		}
	case "anthropic":
		if strings.HasSuffix(path, "/v1") {
			path = strings.TrimSuffix(path, "/v1")
		}
		if path == "/" {
			path = ""
		}
	}

	u.Path = path
	out := u.String()
	out = strings.TrimSuffix(out, "/")
	return out
}

