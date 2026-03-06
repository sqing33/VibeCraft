package expertbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/skillcatalog"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Draft struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	Avatar           string   `json:"avatar,omitempty"`
	PrimaryModelID   string   `json:"primary_model_id"`
	SecondaryModelID string   `json:"secondary_model_id,omitempty"`
	SystemPrompt     string   `json:"system_prompt"`
	PromptTemplate   string   `json:"prompt_template,omitempty"`
	OutputFormat     string   `json:"output_format,omitempty"`
	MaxOutputTokens  int      `json:"max_output_tokens,omitempty"`
	TimeoutMs        int      `json:"timeout_ms,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	EnabledSkills    []string `json:"enabled_skills,omitempty"`
	FallbackOn       []string `json:"fallback_on,omitempty"`
}

type Envelope struct {
	AssistantMessage string `json:"assistant_message"`
	Expert           Draft  `json:"expert"`
}

type Result struct {
	AssistantMessage string   `json:"assistant_message"`
	Draft            Draft    `json:"draft"`
	Warnings         []string `json:"warnings,omitempty"`
	RawJSON          string   `json:"raw_json,omitempty"`
}

type Service struct {
	runner runner.SDKRunner
}

func NewService() Service {
	return Service{runner: runner.NewSDKRunner()}
}

// Generate 功能：基于 builder expert 与对话历史生成结构化专家草稿。
// 参数/返回：spec 为 builder expert 解析结果；llm/skills 为当前可选资源目录；返回助手说明与专家草稿。
// 失败场景：builder 非 SDK expert、模型输出非法 JSON 或引用不存在模型时返回 error。
// 副作用：可能发起外部 SDK 请求。
func (s Service) Generate(ctx context.Context, spec runner.RunSpec, llm *config.LLMSettings, skills []skillcatalog.Entry, messages []Message) (Result, error) {
	if spec.SDK == nil {
		return Result{}, fmt.Errorf("builder expert must be sdk provider")
	}
	if len(messages) == 0 {
		return Result{}, fmt.Errorf("messages are required")
	}
	models := availableModels(llm)
	if len(models) == 0 {
		return Result{}, fmt.Errorf("llm settings must contain at least one model before generating experts")
	}
	if strings.EqualFold(strings.TrimSpace(spec.SDK.Provider), "demo") {
		return s.demoResult(models, skills, messages), nil
	}

	sdk := *spec.SDK
	sdk.OutputSchema = "expert_builder_v1"
	sdk.Instructions = composeInstructions(spec.SDK.Instructions)
	sdk.Prompt = composePrompt(models, skills, messages)

	handle, err := s.runner.StartOneshot(ctx, runner.RunSpec{SDK: &sdk, Env: spec.Env})
	if err != nil {
		return Result{}, err
	}
	defer handle.Close()
	out, readErr := io.ReadAll(handle.Output())
	_, waitErr := handle.Wait()
	if readErr != nil {
		return Result{}, readErr
	}
	if waitErr != nil {
		return Result{}, waitErr
	}
	jsonText, err := extractJSON(string(out))
	if err != nil {
		return Result{}, err
	}
	var env Envelope
	if err := json.Unmarshal([]byte(jsonText), &env); err != nil {
		return Result{}, fmt.Errorf("decode builder output: %w", err)
	}
	warnings, err := validateDraft(env.Expert, models, skills)
	if err != nil {
		return Result{}, err
	}
	draft := normalizeDraft(env.Expert, models)
	return Result{
		AssistantMessage: strings.TrimSpace(env.AssistantMessage),
		Draft:            draft,
		Warnings:         warnings,
		RawJSON:          jsonText,
	}, nil
}

func composeInstructions(existing string) string {
	base := strings.TrimSpace(existing)
	if base != "" {
		base += "\n\n"
	}
	return base + `You are using the expert-creator skill.

Your task is to design a reusable expert for vibe-tree settings. Return ONLY a JSON object matching the requested schema.

Rules:
- The expert must be narrow and professional, not a generic assistant.
- primary_model_id and secondary_model_id must come from the provided model catalog.
- enabled_skills must use the provided skill ids when relevant; prefer a minimal set.
- system_prompt must explain role, priorities, and boundaries.
- assistant_message should briefly explain why the generated expert is designed this way.
- Use secondary_model_id only when fallback is valuable.
- fallback_on should describe request-failure conditions such as request_error, timeout, rate_limit, provider_5xx, network_error.
- Keep id in kebab-case and stable.
`
}

func composePrompt(models []config.LLMModelConfig, skills []skillcatalog.Entry, messages []Message) string {
	var b strings.Builder
	b.WriteString("Available models:\n")
	for _, model := range models {
		_, _ = fmt.Fprintf(&b, "- id=%s provider=%s label=%s model=%s\n", strings.TrimSpace(model.ID), strings.TrimSpace(model.Provider), strings.TrimSpace(model.Label), strings.TrimSpace(model.Model))
	}
	b.WriteString("\nAvailable skills:\n")
	if len(skills) == 0 {
		b.WriteString("- (none discovered)\n")
	} else {
		for _, skill := range skills {
			_, _ = fmt.Fprintf(&b, "- id=%s description=%s\n", skill.ID, strings.TrimSpace(skill.Description))
		}
	}
	b.WriteString("\nConversation:\n")
	for _, msg := range messages {
		role := strings.ToUpper(strings.TrimSpace(msg.Role))
		if role == "" {
			role = "USER"
		}
		_, _ = fmt.Fprintf(&b, "%s: %s\n", role, strings.TrimSpace(msg.Content))
	}
	b.WriteString("\nGenerate the next expert draft now.\n")
	return b.String()
}

func availableModels(llm *config.LLMSettings) []config.LLMModelConfig {
	if llm == nil {
		return nil
	}
	out := make([]config.LLMModelConfig, 0, len(llm.Models))
	for _, model := range llm.Models {
		out = append(out, model)
	}
	return out
}

var jsonEnvelopePattern = regexp.MustCompile(`(?s)\{.*\}`)

func extractJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	match := jsonEnvelopePattern.FindString(raw)
	if strings.TrimSpace(match) == "" {
		return "", fmt.Errorf("builder did not return JSON")
	}
	return match, nil
}

func validateDraft(draft Draft, models []config.LLMModelConfig, skills []skillcatalog.Entry) ([]string, error) {
	modelIDs := make(map[string]struct{}, len(models))
	for _, model := range models {
		modelIDs[strings.TrimSpace(model.ID)] = struct{}{}
	}
	primary := strings.TrimSpace(draft.PrimaryModelID)
	if primary == "" {
		return nil, fmt.Errorf("draft.primary_model_id is required")
	}
	if _, ok := modelIDs[primary]; !ok {
		return nil, fmt.Errorf("draft.primary_model_id %q does not exist", primary)
	}
	secondary := strings.TrimSpace(draft.SecondaryModelID)
	if secondary != "" {
		if _, ok := modelIDs[secondary]; !ok {
			return nil, fmt.Errorf("draft.secondary_model_id %q does not exist", secondary)
		}
	}
	if strings.TrimSpace(draft.Label) == "" || strings.TrimSpace(draft.Description) == "" || strings.TrimSpace(draft.SystemPrompt) == "" {
		return nil, fmt.Errorf("draft must contain label, description, and system_prompt")
	}
	skillIDs := make(map[string]struct{}, len(skills))
	for _, skill := range skills {
		skillIDs[strings.TrimSpace(skill.ID)] = struct{}{}
	}
	warnings := make([]string, 0)
	for _, skill := range draft.EnabledSkills {
		if _, ok := skillIDs[strings.TrimSpace(skill)]; ok || len(skillIDs) == 0 {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("skill %q 未在当前目录中发现，将按元数据保存", skill))
	}
	return warnings, nil
}

func normalizeDraft(draft Draft, models []config.LLMModelConfig) Draft {
	draft.ID = slugify(draft.ID)
	if draft.ID == "" {
		draft.ID = slugify(draft.Label)
	}
	if draft.Label == "" {
		draft.Label = strings.TrimSpace(draft.ID)
	}
	draft.Description = strings.TrimSpace(draft.Description)
	draft.Category = strings.TrimSpace(draft.Category)
	draft.Avatar = strings.TrimSpace(draft.Avatar)
	draft.PrimaryModelID = strings.TrimSpace(draft.PrimaryModelID)
	draft.SecondaryModelID = strings.TrimSpace(draft.SecondaryModelID)
	draft.SystemPrompt = strings.TrimSpace(draft.SystemPrompt)
	draft.PromptTemplate = strings.TrimSpace(draft.PromptTemplate)
	draft.OutputFormat = strings.TrimSpace(draft.OutputFormat)
	draft.EnabledSkills = uniqueTrimmed(draft.EnabledSkills)
	draft.FallbackOn = uniqueTrimmed(draft.FallbackOn)
	if draft.MaxOutputTokens <= 0 {
		draft.MaxOutputTokens = 4000
	}
	if draft.TimeoutMs <= 0 {
		draft.TimeoutMs = 45 * 1000
	}
	if draft.Temperature == nil {
		v := 0.4
		draft.Temperature = &v
	}
	if draft.Category == "" {
		draft.Category = inferCategory(draft.Label + " " + draft.Description)
	}
	if draft.SecondaryModelID != "" && len(draft.FallbackOn) == 0 {
		draft.FallbackOn = []string{"request_error"}
	}
	if draft.PrimaryModelID == "" && len(models) > 0 {
		draft.PrimaryModelID = strings.TrimSpace(models[0].ID)
	}
	return draft
}

func (s Service) demoResult(models []config.LLMModelConfig, skills []skillcatalog.Entry, messages []Message) Result {
	last := strings.TrimSpace(messages[len(messages)-1].Content)
	primary := strings.TrimSpace(models[0].ID)
	secondary := ""
	if len(models) > 1 {
		secondary = strings.TrimSpace(models[1].ID)
	}
	selectedSkills := make([]string, 0, 1)
	for _, skill := range skills {
		if strings.Contains(strings.ToLower(skill.ID), "ui") || strings.Contains(strings.ToLower(skill.Description), "ui") {
			selectedSkills = append(selectedSkills, skill.ID)
			break
		}
	}
	draft := normalizeDraft(Draft{
		ID:               slugify(last),
		Label:            titleFromSlug(slugify(last)),
		Description:      fmt.Sprintf("根据需求“%s”生成的专家草稿。", shorten(last, 48)),
		Category:         inferCategory(last),
		Avatar:           "✨",
		PrimaryModelID:   primary,
		SecondaryModelID: secondary,
		SystemPrompt:     fmt.Sprintf("你是一名专注于%s任务的专家。优先关注用户目标、可执行建议与输出结构化结果。", inferCategory(last)),
		PromptTemplate:   "{{prompt}}",
		OutputFormat:     "目标 -> 方案 -> 风险 -> 下一步",
		EnabledSkills:    selectedSkills,
		FallbackOn:       []string{"request_error"},
		MaxOutputTokens:  4000,
		TimeoutMs:        45000,
	}, models)
	return Result{
		AssistantMessage: "已根据你的描述生成一个可发布的 demo 专家草稿。你可以继续补充要求，我会继续细化它。",
		Draft:            draft,
		Warnings:         nil,
		RawJSON:          "",
	}
}

func inferCategory(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "ui") || strings.Contains(lower, "设计") || strings.Contains(lower, "ux"):
		return "design"
	case strings.Contains(lower, "规划") || strings.Contains(lower, "plan"):
		return "planning"
	case strings.Contains(lower, "运维") || strings.Contains(lower, "deploy") || strings.Contains(lower, "ops"):
		return "ops"
	default:
		return "general"
	}
}

func shorten(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max]) + "…"
}

func uniqueTrimmed(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func slugify(text string) string {
	text = strings.TrimSpace(strings.ToLower(text))
	var b strings.Builder
	lastDash := false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func titleFromSlug(slug string) string {
	parts := strings.Split(strings.Trim(slug, "-"), "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
