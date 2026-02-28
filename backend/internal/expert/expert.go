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

type PublicExpert struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	TimeoutMs int    `json:"timeout_ms"`
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

// ListPublic 功能：列出可安全暴露给 UI 的 experts 信息（不包含 command/args/env 等敏感字段）。
// 参数/返回：无入参；返回按 id 排序的 PublicExpert 列表。
// 失败场景：无。
// 副作用：无。
func (r *Registry) ListPublic() []PublicExpert {
	if r == nil {
		return nil
	}
	out := make([]PublicExpert, 0, len(r.experts))
	for id, e := range r.experts {
		label := strings.TrimSpace(e.Label)
		if label == "" {
			label = id
		}
		provider := strings.TrimSpace(e.Provider)
		model := strings.TrimSpace(e.Model)
		out = append(out, PublicExpert{
			ID:        id,
			Label:     label,
			Provider:  provider,
			Model:     model,
			TimeoutMs: e.TimeoutMs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// Resolve 功能：将 expert_id + prompt 解析为 SDK 驱动的 RunSpec（支持 `{{prompt}}/{{workspace}}` 与 `${ENV}` 模板替换）。
// 参数/返回：expertID 为选择的专家；prompt 为 node 的 prompt；cwd 为工作目录；返回 Resolved（含 RunSpec 与超时）。
// 失败场景：expert 不存在、provider/model 缺失或 env 模板缺失时返回 error。
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

	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		return Resolved{}, fmt.Errorf("expert %q: provider is required", expertID)
	}

	model := strings.TrimSpace(e.Model)
	if provider != "demo" && provider != "process" && model == "" {
		return Resolved{}, fmt.Errorf("expert %q: model is required (provider=%s)", expertID, provider)
	}

	finalPrompt := prompt
	if strings.TrimSpace(e.PromptTemplate) != "" {
		finalPrompt = strings.ReplaceAll(e.PromptTemplate, "{{prompt}}", prompt)
		finalPrompt = strings.ReplaceAll(finalPrompt, "{{workspace}}", cwd)
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

	switch provider {
	case "process":
		if strings.TrimSpace(e.BaseURL) != "" || strings.TrimSpace(e.OutputSchema) != "" || strings.TrimSpace(e.SystemPrompt) != "" || strings.TrimSpace(e.Model) != "" {
			return Resolved{}, fmt.Errorf("expert %q: provider=process does not support model/system_prompt/base_url/output_schema", expertID)
		}
		cmd := strings.TrimSpace(e.Command)
		if cmd == "" {
			return Resolved{}, fmt.Errorf("expert %q: command is required (provider=process)", expertID)
		}

		args := make([]string, 0, len(e.Args))
		for _, a := range e.Args {
			a = strings.ReplaceAll(a, "{{prompt}}", finalPrompt)
			a = strings.ReplaceAll(a, "{{workspace}}", cwd)
			args = append(args, a)
		}

		if strings.TrimSpace(e.RunMode) != "" && strings.TrimSpace(e.RunMode) != "oneshot" {
			return Resolved{}, fmt.Errorf("expert %q: unsupported run_mode %q (provider=process)", expertID, strings.TrimSpace(e.RunMode))
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
	case "openai", "anthropic", "demo":
		// LLM/Demo 走 SDK 驱动；禁止 legacy CLI 字段悄悄生效。
		if strings.TrimSpace(e.Command) != "" || len(e.Args) > 0 || strings.TrimSpace(e.RunMode) != "" {
			return Resolved{}, fmt.Errorf("expert %q: legacy CLI fields (run_mode/command/args) are not supported when provider=%s", expertID, provider)
		}

		baseURL := strings.TrimSpace(e.BaseURL)
		if baseURL != "" {
			expanded, err := expandEnvTemplate(baseURL)
			if err != nil {
				return Resolved{}, fmt.Errorf("expert %q: base_url: %w", expertID, err)
			}
			baseURL = expanded
		}

		outputSchema := strings.TrimSpace(e.OutputSchema)
		if outputSchema != "" && strings.ToLower(outputSchema) != "dag_v1" {
			return Resolved{}, fmt.Errorf("expert %q: output_schema %q is not supported", expertID, outputSchema)
		}

		return Resolved{
			Spec: runner.RunSpec{
				Command: "sdk:" + provider,
				Args:    []string{"model=" + model},
				Env:     env,
				Cwd:     cwd,
				SDK: &runner.SDKSpec{
					Provider:        provider,
					Model:           model,
					Prompt:          finalPrompt,
					Instructions:    strings.TrimSpace(e.SystemPrompt),
					BaseURL:         baseURL,
					MaxOutputTokens: e.MaxOutputTokens,
					Temperature:     e.Temperature,
					OutputSchema:    outputSchema,
				},
			},
			Timeout: timeout,
		}, nil
	default:
		return Resolved{}, fmt.Errorf("expert %q: unsupported provider %q", expertID, provider)
	}
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
