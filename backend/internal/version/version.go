package version

import "strings"

// Commit 功能：用于注入构建时 git commit（默认 dev）。
// 参数/返回：由 ldflags 注入的全局变量；用于 API/日志展示。
// 失败场景：无。
// 副作用：无。
var Commit = "dev"

// BuiltAt 功能：用于注入构建时间（可选）。
// 参数/返回：由 ldflags 注入的全局变量；用于 API/日志展示。
// 失败场景：无。
// 副作用：无。
var BuiltAt = ""

type Info struct {
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at,omitempty"`
}

// Current 功能：返回当前版本信息（对空值做规范化）。
// 参数/返回：无入参；返回 Info。
// 失败场景：无。
// 副作用：无。
func Current() Info {
	commit := strings.TrimSpace(Commit)
	if commit == "" {
		commit = "dev"
	}
	return Info{
		Commit:  commit,
		BuiltAt: strings.TrimSpace(BuiltAt),
	}
}
