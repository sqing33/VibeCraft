package expert

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/runner"
)

type Resolved struct {
	Spec           runner.RunSpec
	Timeout        time.Duration
	ManagedSource  string
	PrimaryModelID string
	Provider       string
	ProtocolFamily string
	Model          string
	RuntimeKind    string
	CLIFamily      string
	ToolID         string
	ExpertID       string
	HelperOnly     bool
}

type ResolveOptions struct {
	CLIToolID string
	ModelID   string
}

type Registry struct {
	mu            sync.RWMutex
	experts       map[string]config.ExpertConfig
	cliTools      map[string]config.CLIToolConfig
	llm           *config.LLMSettings
	apiSources    []config.APISourceConfig
	runtimeModels *config.RuntimeModelSettings
}

type PublicExpert struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Description      string   `json:"description,omitempty"`
	Category         string   `json:"category,omitempty"`
	Avatar           string   `json:"avatar,omitempty"`
	ManagedSource    string   `json:"managed_source,omitempty"`
	PrimaryModelID   string   `json:"primary_model_id,omitempty"`
	SecondaryModelID string   `json:"secondary_model_id,omitempty"`
	FallbackOn       []string `json:"fallback_on,omitempty"`
	EnabledSkills    []string `json:"enabled_skills,omitempty"`
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	RuntimeKind      string   `json:"runtime_kind,omitempty"`
	CLIFamily        string   `json:"cli_family,omitempty"`
	HelperOnly       bool     `json:"helper_only,omitempty"`
	TimeoutMs        int      `json:"timeout_ms"`
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
	tools := make(map[string]config.CLIToolConfig, len(cfg.CLITools))
	for _, item := range cfg.CLITools {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		tools[item.ID] = item
	}
	return &Registry{experts: m, cliTools: tools, llm: cfg.LLM, apiSources: append([]config.APISourceConfig(nil), cfg.APISources...), runtimeModels: cloneRuntimeModelSettings(cfg.RuntimeModels)}
}

// Reload 功能：使用新的运行配置刷新 registry（原子替换 experts map）。
// 参数/返回：cfg 为最新 Config；无返回值。
// 失败场景：无（非法条目在 Resolve 时返回错误）。
// 副作用：更新内存中的 experts 集合。
func (r *Registry) Reload(cfg config.Config) {
	if r == nil {
		return
	}
	m := make(map[string]config.ExpertConfig, len(cfg.Experts))
	for _, e := range cfg.Experts {
		if strings.TrimSpace(e.ID) == "" {
			continue
		}
		m[e.ID] = e
	}
	tools := make(map[string]config.CLIToolConfig, len(cfg.CLITools))
	for _, item := range cfg.CLITools {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		tools[item.ID] = item
	}
	r.mu.Lock()
	r.experts = m
	r.cliTools = tools
	r.llm = cfg.LLM
	r.apiSources = append([]config.APISourceConfig(nil), cfg.APISources...)
	r.runtimeModels = cloneRuntimeModelSettings(cfg.RuntimeModels)
	r.mu.Unlock()
}

// KnownIDs 功能：返回已注册的 expert_id 集合（用于 DAG 校验等）。
// 参数/返回：无入参；返回 set（map[string]struct{}）。
// 失败场景：无。
// 副作用：无。
func (r *Registry) KnownIDs() map[string]struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PublicExpert, 0, len(r.experts))
	for id, e := range r.experts {
		if e.Disabled {
			continue
		}
		label := strings.TrimSpace(e.Label)
		if label == "" {
			label = id
		}
		provider := strings.TrimSpace(e.Provider)
		model := strings.TrimSpace(e.Model)
		out = append(out, PublicExpert{
			ID:               id,
			Label:            label,
			Description:      strings.TrimSpace(e.Description),
			Category:         strings.TrimSpace(e.Category),
			Avatar:           strings.TrimSpace(e.Avatar),
			ManagedSource:    strings.TrimSpace(e.ManagedSource),
			PrimaryModelID:   strings.TrimSpace(e.PrimaryModelID),
			SecondaryModelID: strings.TrimSpace(e.SecondaryModelID),
			FallbackOn:       append([]string(nil), e.FallbackOn...),
			EnabledSkills:    append([]string(nil), e.EnabledSkills...),
			Provider:         provider,
			Model:            model,
			RuntimeKind:      strings.TrimSpace(e.RuntimeKind),
			CLIFamily:        strings.TrimSpace(e.CLIFamily),
			HelperOnly:       e.HelperOnly,
			TimeoutMs:        e.TimeoutMs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// Resolve 功能：将 expert_id + prompt 解析为 RunSpec（兼容默认解析选项）。
func (r *Registry) Resolve(expertID, prompt, cwd string) (Resolved, error) {
	return r.ResolveWithOptions(expertID, prompt, cwd, ResolveOptions{})
}

// ResolveWithOptions 功能：在 expert 基础上叠加 `cli_tool_id/model_id` 选择，生成最终 RunSpec。
func (r *Registry) ResolveWithOptions(expertID, prompt, cwd string, opts ResolveOptions) (Resolved, error) {
	if r == nil {
		return Resolved{}, fmt.Errorf("expert registry not initialized")
	}
	if strings.TrimSpace(expertID) == "" {
		return Resolved{}, fmt.Errorf("expert_id is required")
	}

	r.mu.RLock()
	e, ok := r.experts[expertID]
	if !ok && strings.TrimSpace(opts.CLIToolID) != "" {
		if matched, matchedID, found := selectCLIExpertByTool(r.experts, r.cliTools, strings.TrimSpace(opts.CLIToolID)); found {
			e = matched
			expertID = matchedID
			ok = true
		}
	}
	tools := make(map[string]config.CLIToolConfig, len(r.cliTools))
	for id, item := range r.cliTools {
		tools[id] = item
	}
	llm := r.llm
	apiSources := append([]config.APISourceConfig(nil), r.apiSources...)
	runtimeModels := cloneRuntimeModelSettings(r.runtimeModels)
	r.mu.RUnlock()
	if !ok {
		return Resolved{}, fmt.Errorf("unknown expert_id %q", expertID)
	}
	if e.Disabled {
		return Resolved{}, fmt.Errorf("expert %q is disabled", expertID)
	}

	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		return Resolved{}, fmt.Errorf("expert %q: provider is required", expertID)
	}

	model := strings.TrimSpace(e.Model)
	protocolFamily := provider
	toolID := strings.TrimSpace(opts.CLIToolID)
	selectedModelID := strings.TrimSpace(opts.ModelID)
	selectedTool := config.CLIToolConfig{}
	if provider == "cli" {
		var err error
		selectedTool, toolID, err = resolveCLIToolSelection(tools, e, toolID)
		if err != nil {
			return Resolved{}, err
		}
		if toolID != "" {
			protocolFamily = strings.TrimSpace(selectedTool.ProtocolFamily)
			if strings.TrimSpace(selectedTool.CLIFamily) != "" {
				e.CLIFamily = strings.TrimSpace(selectedTool.CLIFamily)
			}
			if strings.TrimSpace(selectedTool.CommandPath) != "" {
				if e.Env == nil {
					e.Env = map[string]string{}
				}
				e.Env["VIBECRAFT_CLI_COMMAND_PATH"] = strings.TrimSpace(selectedTool.CommandPath)
			}
			if selectedModelID == "" {
				runtimeID := runtimeModelRuntimeID(toolID, e.CLIFamily)
				if resolved, ok := config.ResolveRuntimeModelBinding(runtimeModels, apiSources, runtimeID, ""); ok {
					selectedModelID = strings.TrimSpace(resolved.Model.ID)
				} else if runner.NormalizeCLIFamily(selectedTool.CLIFamily) == "iflow" {
					selectedModelID = strings.TrimSpace(selectedTool.IFlowDefaultModel)
				} else {
					selectedModelID = strings.TrimSpace(selectedTool.DefaultModelID)
				}
			}
		}
		if selectedModelID == "" {
			selectedModelID = strings.TrimSpace(e.PrimaryModelID)
		}
		if strings.TrimSpace(selectedModelID) != "" {
			runtimeID := runtimeModelRuntimeID(toolID, e.CLIFamily)
			if resolvedRuntime, ok := config.ResolveRuntimeModelBinding(runtimeModels, apiSources, runtimeID, selectedModelID); ok {
				selectedProvider := configProtocol(resolvedRuntime.Model.Provider)
				if toolID != "" && !config.CLIToolSupportsProtocolFamily(selectedTool, selectedProvider) && runner.NormalizeCLIFamily(e.CLIFamily) != "iflow" {
					return Resolved{}, fmt.Errorf("expert %q: model %q is not compatible with cli tool %q", expertID, selectedModelID, toolID)
				}
				model = strings.TrimSpace(resolvedRuntime.Model.Model)
				e.PrimaryModelID = strings.TrimSpace(resolvedRuntime.Model.ID)
				protocolFamily = selectedProvider
				if e.Env == nil {
					e.Env = map[string]string{}
				}
				applyCLIRuntimeModelEnv(e.Env, *resolvedRuntime)
			} else if runner.NormalizeCLIFamily(e.CLIFamily) == "iflow" {
				model = strings.TrimSpace(selectedModelID)
				e.PrimaryModelID = strings.TrimSpace(selectedModelID)
			} else {
				selectedModel, ok := lookupLLMModel(llm, selectedModelID)
				if !ok {
					return Resolved{}, fmt.Errorf("expert %q: model %q does not exist", expertID, selectedModelID)
				}
				selectedProvider := configProtocol(selectedModel.Provider)
				if toolID != "" && !config.CLIToolSupportsProtocolFamily(selectedTool, selectedProvider) {
					return Resolved{}, fmt.Errorf("expert %q: model %q is not compatible with cli tool %q", expertID, selectedModelID, toolID)
				}
				model = strings.TrimSpace(selectedModel.Model)
				e.PrimaryModelID = strings.TrimSpace(selectedModel.ID)
				protocolFamily = selectedProvider
			}
		}
	}
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
	if provider == "cli" && runner.NormalizeCLIFamily(e.CLIFamily) != "iflow" {
		if resolvedRuntime, ok := config.ResolveRuntimeModelBinding(runtimeModels, apiSources, runtimeModelRuntimeID(toolID, e.CLIFamily), selectedModelID); ok {
			applyCLIRuntimeModelEnv(env, *resolvedRuntime)
		} else {
			applyCLIModelRuntimeEnv(env, llm, selectedModelID)
		}
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
			Timeout:        timeout,
			ManagedSource:  strings.TrimSpace(e.ManagedSource),
			PrimaryModelID: strings.TrimSpace(e.PrimaryModelID),
			Provider:       provider,
			ProtocolFamily: protocolFamily,
			Model:          model,
			RuntimeKind:    strings.TrimSpace(e.RuntimeKind),
			CLIFamily:      strings.TrimSpace(e.CLIFamily),
			ToolID:         toolID,
			ExpertID:       expertID,
			HelperOnly:     e.HelperOnly,
		}, nil
	case "cli":
		family := runner.NormalizeCLIFamily(e.CLIFamily)
		if family == "" {
			if strings.Contains(strings.ToLower(strings.TrimSpace(e.ID)), "claude") {
				family = "claude"
			} else {
				family = "codex"
			}
		}
		scriptPath, err := runner.CLIScriptPath(family)
		if err != nil {
			return Resolved{}, fmt.Errorf("expert %q: %w", expertID, err)
		}
		baseURL := strings.TrimSpace(e.BaseURL)
		if baseURL != "" {
			expanded, err := expandEnvTemplate(baseURL)
			if err != nil {
				return Resolved{}, fmt.Errorf("expert %q: base_url: %w", expertID, err)
			}
			baseURL = expanded
		}
		if strings.TrimSpace(e.OutputSchema) != "" && strings.ToLower(strings.TrimSpace(e.OutputSchema)) != "dag_v1" && strings.ToLower(strings.TrimSpace(e.OutputSchema)) != "expert_builder_v1" {
			return Resolved{}, fmt.Errorf("expert %q: output_schema %q is not supported", expertID, e.OutputSchema)
		}
		env["VIBECRAFT_PROMPT"] = finalPrompt
		if strings.TrimSpace(e.SystemPrompt) != "" {
			env["VIBECRAFT_SYSTEM_PROMPT"] = strings.TrimSpace(e.SystemPrompt)
		}
		env["VIBECRAFT_MODEL"] = model
		if strings.TrimSpace(protocolFamily) != "" {
			env["VIBECRAFT_PROTOCOL_FAMILY"] = strings.TrimSpace(protocolFamily)
		}
		if strings.TrimSpace(toolID) != "" {
			env["VIBECRAFT_CLI_TOOL_ID"] = strings.TrimSpace(toolID)
		}
		if strings.TrimSpace(e.PrimaryModelID) != "" {
			env["VIBECRAFT_MODEL_ID"] = strings.TrimSpace(e.PrimaryModelID)
		}
		env["VIBECRAFT_CLI_FAMILY"] = family
		if strings.TrimSpace(e.OutputSchema) != "" {
			env["VIBECRAFT_OUTPUT_SCHEMA"] = strings.TrimSpace(e.OutputSchema)
		}
		if baseURL != "" {
			env["VIBECRAFT_BASE_URL"] = baseURL
		}

		return Resolved{
			Spec: runner.RunSpec{
				Command: "bash",
				Args:    []string{scriptPath},
				Env:     env,
				Cwd:     cwd,
			},
			Timeout:        timeout,
			ManagedSource:  strings.TrimSpace(e.ManagedSource),
			PrimaryModelID: strings.TrimSpace(e.PrimaryModelID),
			Provider:       "cli",
			ProtocolFamily: protocolFamily,
			Model:          model,
			RuntimeKind:    strings.TrimSpace(e.RuntimeKind),
			CLIFamily:      family,
			ToolID:         toolID,
			ExpertID:       expertID,
			HelperOnly:     e.HelperOnly,
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
		if outputSchema != "" && strings.ToLower(outputSchema) != "dag_v1" && strings.ToLower(outputSchema) != "expert_builder_v1" {
			return Resolved{}, fmt.Errorf("expert %q: output_schema %q is not supported", expertID, outputSchema)
		}

		fallbacks := make([]runner.SDKFallback, 0, 1)
		if strings.TrimSpace(e.SecondaryProvider) != "" && strings.TrimSpace(e.SecondaryModel) != "" {
			fallbackEnv := make(map[string]string, len(e.SecondaryEnv))
			for k, v := range e.SecondaryEnv {
				expanded, err := expandEnvTemplate(v)
				if err != nil {
					return Resolved{}, fmt.Errorf("expert %q: secondary env %s: %w", expertID, k, err)
				}
				fallbackEnv[k] = expanded
			}
			secondaryBaseURL := strings.TrimSpace(e.SecondaryBaseURL)
			if secondaryBaseURL != "" {
				expanded, err := expandEnvTemplate(secondaryBaseURL)
				if err != nil {
					return Resolved{}, fmt.Errorf("expert %q: secondary_base_url: %w", expertID, err)
				}
				secondaryBaseURL = expanded
			}
			fallbacks = append(fallbacks, runner.SDKFallback{
				Env: fallbackEnv,
				SDK: runner.SDKSpec{
					Provider:        strings.TrimSpace(e.SecondaryProvider),
					Model:           strings.TrimSpace(e.SecondaryModel),
					LLMModelID:      strings.TrimSpace(e.SecondaryModelID),
					Prompt:          finalPrompt,
					Instructions:    strings.TrimSpace(e.SystemPrompt),
					BaseURL:         secondaryBaseURL,
					MaxOutputTokens: e.MaxOutputTokens,
					Temperature:     e.Temperature,
					OutputSchema:    outputSchema,
				},
			})
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
					LLMModelID:      strings.TrimSpace(e.PrimaryModelID),
					Prompt:          finalPrompt,
					Instructions:    strings.TrimSpace(e.SystemPrompt),
					BaseURL:         baseURL,
					MaxOutputTokens: e.MaxOutputTokens,
					Temperature:     e.Temperature,
					OutputSchema:    outputSchema,
				},
				SDKFallbacks: fallbacks,
			},
			Timeout:        timeout,
			ManagedSource:  strings.TrimSpace(e.ManagedSource),
			PrimaryModelID: strings.TrimSpace(e.PrimaryModelID),
			Provider:       provider,
			ProtocolFamily: protocolFamily,
			Model:          model,
			RuntimeKind:    strings.TrimSpace(e.RuntimeKind),
			CLIFamily:      strings.TrimSpace(e.CLIFamily),
			ToolID:         toolID,
			ExpertID:       expertID,
			HelperOnly:     e.HelperOnly,
		}, nil
	default:
		return Resolved{}, fmt.Errorf("expert %q: unsupported provider %q", expertID, provider)
	}
}

func selectCLIExpertByTool(experts map[string]config.ExpertConfig, tools map[string]config.CLIToolConfig, toolID string) (config.ExpertConfig, string, bool) {
	item, ok := tools[strings.TrimSpace(toolID)]
	if !ok {
		return config.ExpertConfig{}, "", false
	}
	family := runner.NormalizeCLIFamily(item.CLIFamily)
	for id, expert := range experts {
		if strings.TrimSpace(expert.Provider) != "cli" || expert.Disabled {
			continue
		}
		if strings.TrimSpace(id) == strings.TrimSpace(toolID) {
			return expert, id, true
		}
		if runner.NormalizeCLIFamily(expert.CLIFamily) == family {
			return expert, id, true
		}
	}
	return config.ExpertConfig{}, "", false
}

func resolveCLIToolSelection(tools map[string]config.CLIToolConfig, expert config.ExpertConfig, requested string) (config.CLIToolConfig, string, error) {
	if len(tools) == 0 {
		return config.CLIToolConfig{}, "", nil
	}
	if requested = strings.TrimSpace(requested); requested != "" {
		item, ok := tools[requested]
		if !ok {
			return config.CLIToolConfig{}, "", fmt.Errorf("cli_tool_id %q does not exist", requested)
		}
		if !item.Enabled {
			return config.CLIToolConfig{}, "", fmt.Errorf("cli_tool_id %q is disabled", requested)
		}
		return item, requested, nil
	}
	if item, ok := tools[strings.TrimSpace(expert.ID)]; ok {
		if !item.Enabled {
			return config.CLIToolConfig{}, "", fmt.Errorf("cli_tool_id %q is disabled", item.ID)
		}
		return item, strings.TrimSpace(expert.ID), nil
	}
	family := runner.NormalizeCLIFamily(expert.CLIFamily)
	for id, item := range tools {
		if runner.NormalizeCLIFamily(item.CLIFamily) == family && item.Enabled {
			return item, id, nil
		}
	}
	return config.CLIToolConfig{}, "", nil
}

func runtimeModelRuntimeID(toolID, cliFamily string) string {
	if normalized := strings.TrimSpace(toolID); normalized != "" {
		switch normalized {
		case config.RuntimeIDCodex:
			return config.RuntimeIDCodex
		case config.RuntimeIDClaude:
			return config.RuntimeIDClaude
		case config.RuntimeIDIFLOW:
			return config.RuntimeIDIFLOW
		case config.RuntimeIDOpenCode:
			return config.RuntimeIDOpenCode
		}
	}
	switch runner.NormalizeCLIFamily(cliFamily) {
	case "codex":
		return config.RuntimeIDCodex
	case "claude":
		return config.RuntimeIDClaude
	case "iflow":
		return config.RuntimeIDIFLOW
	case "opencode":
		return config.RuntimeIDOpenCode
	default:
		return ""
	}
}

func cloneRuntimeModelSettings(settings *config.RuntimeModelSettings) *config.RuntimeModelSettings {
	if settings == nil {
		return nil
	}
	out := &config.RuntimeModelSettings{Runtimes: make([]config.RuntimeModelRuntimeConfig, 0, len(settings.Runtimes))}
	for _, runtime := range settings.Runtimes {
		item := runtime
		item.Models = append([]config.RuntimeModelConfig(nil), runtime.Models...)
		out.Runtimes = append(out.Runtimes, item)
	}
	return out
}

func lookupLLMModel(llm *config.LLMSettings, modelID string) (config.LLMModelConfig, bool) {
	if llm == nil {
		return config.LLMModelConfig{}, false
	}
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	for _, item := range llm.Models {
		if strings.ToLower(strings.TrimSpace(item.ID)) == modelID {
			return item, true
		}
	}
	return config.LLMModelConfig{}, false
}

func applyCLIModelRuntimeEnv(env map[string]string, llm *config.LLMSettings, modelID string) {
	model, source, _, ok := config.FindLLMModelByID(llm, modelID)
	if !ok {
		return
	}
	applyCLIRuntimeModelEnv(env, config.ResolvedRuntimeModel{
		Model:  config.RuntimeModelConfig{Provider: model.Provider},
		Source: config.APISourceConfig{Provider: source.Provider, BaseURL: source.BaseURL, APIKey: source.APIKey},
	})
}

func applyCLIRuntimeModelEnv(env map[string]string, resolved config.ResolvedRuntimeModel) {
	provider := configProtocol(resolved.Model.Provider)
	switch provider {
	case "openai":
		delete(env, "OPENAI_API_KEY")
		delete(env, "OPENAI_BASE_URL")
		delete(env, "ANTHROPIC_API_KEY")
		delete(env, "ANTHROPIC_BASE_URL")
		if strings.TrimSpace(resolved.Source.APIKey) != "" {
			env["OPENAI_API_KEY"] = strings.TrimSpace(resolved.Source.APIKey)
		}
		if strings.TrimSpace(resolved.Source.BaseURL) != "" {
			env["OPENAI_BASE_URL"] = strings.TrimSpace(resolved.Source.BaseURL)
			env["VIBECRAFT_BASE_URL"] = strings.TrimSpace(resolved.Source.BaseURL)
		}
	case "anthropic":
		delete(env, "ANTHROPIC_API_KEY")
		delete(env, "ANTHROPIC_BASE_URL")
		delete(env, "OPENAI_API_KEY")
		delete(env, "OPENAI_BASE_URL")
		if strings.TrimSpace(resolved.Source.APIKey) != "" {
			env["ANTHROPIC_API_KEY"] = strings.TrimSpace(resolved.Source.APIKey)
		}
		if strings.TrimSpace(resolved.Source.BaseURL) != "" {
			env["ANTHROPIC_BASE_URL"] = strings.TrimSpace(resolved.Source.BaseURL)
			env["VIBECRAFT_BASE_URL"] = strings.TrimSpace(resolved.Source.BaseURL)
		}
	case "iflow":
		env["VIBECRAFT_IFLOW_AUTH_MODE"] = firstNonEmptyValue(strings.TrimSpace(resolved.Source.AuthMode), config.IFLOWAuthModeBrowser)
		env["VIBECRAFT_IFLOW_BASE_URL"] = firstNonEmptyValue(strings.TrimSpace(resolved.Source.BaseURL), iflowDefaultBaseURL())
		if env["VIBECRAFT_IFLOW_AUTH_MODE"] == config.IFLOWAuthModeAPIKey {
			env["VIBECRAFT_IFLOW_API_KEY"] = strings.TrimSpace(resolved.Source.APIKey)
		} else {
			delete(env, "VIBECRAFT_IFLOW_API_KEY")
		}
	}
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func iflowDefaultBaseURL() string {
	return firstNonEmptyTrimmed("https://apis.iflow.cn/v1")
}

func configProtocol(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
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
func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
