package expert

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/runner"
)

type Resolved struct {
	Spec    runner.RunSpec
	Timeout time.Duration
}

type Registry struct {
	experts map[string]config.ExpertConfig
}

// NewRegistry 功能：从运行配置构建 expert 注册表（按 id 去重，后者覆盖前者）。
// 参数/返回：cfg 为最终运行 Config；返回 Registry。
// 失败场景：无（非法条目会在 Resolve 时返回可读错误）。
// 副作用：无。
func NewRegistry(cfg config.Config) *Registry {
	m := make(map[string]config.ExpertConfig, len(cfg.Experts))
	for _, e := range cfg.Experts {
		if strings.TrimSpace(e.ID) == "" {
			continue
		}
		m[e.ID] = e
	}
	return &Registry{experts: m}
}

// KnownIDs 功能：返回已注册的 expert_id 集合（用于 DAG 校验等）。
// 参数/返回：无入参；返回 set（map[string]struct{}）。
// 失败场景：无。
// 副作用：无。
func (r *Registry) KnownIDs() map[string]struct{} {
	out := make(map[string]struct{}, len(r.experts))
	for id := range r.experts {
		out[id] = struct{}{}
	}
	return out
}

// Resolve 功能：将 expert_id + prompt 解析为可执行的 RunSpec（支持 `{{prompt}}` 与 `${ENV}` 模板替换）。
// 参数/返回：expertID 为选择的专家；prompt 为 node 的 prompt；cwd 为工作目录；返回 Resolved（含 RunSpec 与超时）。
// 失败场景：expert 不存在、command 缺失、run_mode 不支持或 env 模板缺失时返回 error。
// 副作用：读取当前进程环境变量（用于 `${VAR}` 注入）。
func (r *Registry) Resolve(expertID, prompt, cwd string) (Resolved, error) {
	if r == nil {
		return Resolved{}, fmt.Errorf("expert registry not initialized")
	}
	if strings.TrimSpace(expertID) == "" {
		return Resolved{}, fmt.Errorf("expert_id is required")
	}

	e, ok := r.experts[expertID]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown expert_id %q", expertID)
	}

	runMode := strings.TrimSpace(e.RunMode)
	if runMode == "" {
		runMode = "oneshot"
	}
	if runMode != "oneshot" {
		return Resolved{}, fmt.Errorf("unsupported run_mode %q (expert=%s)", runMode, expertID)
	}

	cmd := strings.TrimSpace(e.Command)
	if cmd == "" {
		return Resolved{}, fmt.Errorf("expert %q: command is required", expertID)
	}

	args := make([]string, 0, len(e.Args))
	for _, a := range e.Args {
		args = append(args, strings.ReplaceAll(a, "{{prompt}}", prompt))
	}

	env := make(map[string]string, len(e.Env))
	for k, v := range e.Env {
		expanded, err := expandEnvTemplate(v)
		if err != nil {
			return Resolved{}, fmt.Errorf("expert %q: env %s: %w", expertID, k, err)
		}
		env[k] = expanded
	}

	timeout := time.Duration(e.TimeoutMs) * time.Millisecond
	if e.TimeoutMs <= 0 {
		timeout = 0
	}

	return Resolved{
		Spec: runner.RunSpec{
			Command: cmd,
			Args:    args,
			Env:     env,
			Cwd:     cwd,
		},
		Timeout: timeout,
	}, nil
}

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expandEnvTemplate(s string) (string, error) {
	missing := make([]string, 0)

	out := envPattern.ReplaceAllStringFunc(s, func(m string) string {
		sub := envPattern.FindStringSubmatch(m)
		if len(sub) != 2 {
			return ""
		}
		key := sub[1]
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		missing = append(missing, key)
		return ""
	})

	if len(missing) > 0 {
		sort.Strings(missing)
		missing = uniqueStrings(missing)
		return "", fmt.Errorf("missing env vars: %s", strings.Join(missing, ","))
	}
	return out, nil
}

func uniqueStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	out := make([]string, 0, len(in))
	prev := ""
	for _, s := range in {
		if s == prev {
			continue
		}
		out = append(out, s)
		prev = s
	}
	return out
}
