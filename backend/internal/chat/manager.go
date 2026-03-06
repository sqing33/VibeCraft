package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go"
	openai_option "github.com/openai/openai-go/option"
	openai_responses "github.com/openai/openai-go/responses"
	openai_shared "github.com/openai/openai-go/shared"

	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

const (
	defaultContextWindow = int64(128_000)
	defaultSoftRatio     = 0.82
	defaultForceRatio    = 0.92
	defaultHardRatio     = 0.97
	defaultKeepRecent    = 12
)

type Options struct {
	SoftRatio     float64
	ForceRatio    float64
	HardRatio     float64
	KeepRecent    int
	ContextWindow int64
}

type Manager struct {
	store *store.Store
	hub   *ws.Hub

	openaiClient    openai.Client
	anthropicClient anthropic.Client

	softRatio     float64
	forceRatio    float64
	hardRatio     float64
	keepRecent    int
	contextWindow int64
}

type TurnParams struct {
	Session     store.ChatSession
	ExpertID    string
	UserInput   string
	ModelInput  string
	Attachments []UploadedAttachment
	SDK         runner.SDKSpec
	Env         map[string]string
	Fallbacks   []runner.SDKFallback
}

type TurnResult struct {
	UserMessage       store.ChatMessage `json:"user_message"`
	AssistantMessage  store.ChatMessage `json:"assistant_message"`
	ReasoningText     *string           `json:"reasoning_text,omitempty"`
	ModelInput        *string           `json:"model_input,omitempty"`
	ContextMode       *string           `json:"context_mode,omitempty"`
	CachedInputTokens *int64            `json:"cached_input_tokens,omitempty"`
}

type providerCallMeta struct {
	ModelInput        string
	ContextMode       string
	TokenIn           *int64
	TokenOut          *int64
	CachedInputTokens *int64
}

func NewManager(st *store.Store, hub *ws.Hub, opts Options) *Manager {
	soft := opts.SoftRatio
	if soft <= 0 || soft >= 1 {
		soft = defaultSoftRatio
	}
	force := opts.ForceRatio
	if force <= 0 || force >= 1 {
		force = defaultForceRatio
	}
	hard := opts.HardRatio
	if hard <= 0 || hard >= 1 {
		hard = defaultHardRatio
	}
	if force < soft {
		force = soft
	}
	if hard < force {
		hard = force
	}
	keepRecent := opts.KeepRecent
	if keepRecent < 2 {
		keepRecent = defaultKeepRecent
	}
	contextWindow := opts.ContextWindow
	if contextWindow <= 0 {
		contextWindow = defaultContextWindow
	}

	return &Manager{
		store:           st,
		hub:             hub,
		openaiClient:    openai.NewClient(),
		anthropicClient: anthropic.NewClient(),
		softRatio:       soft,
		forceRatio:      force,
		hardRatio:       hard,
		keepRecent:      keepRecent,
		contextWindow:   contextWindow,
	}
}

func (m *Manager) RunTurn(ctx context.Context, params TurnParams) (TurnResult, error) {
	if m == nil || m.store == nil {
		return TurnResult{}, fmt.Errorf("chat manager not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(params.Session.ID) == "" {
		return TurnResult{}, fmt.Errorf("%w: session_id is required", store.ErrValidation)
	}
	if params.Session.Status != "active" {
		return TurnResult{}, fmt.Errorf("%w: session is not active", store.ErrValidation)
	}
	expertID := strings.TrimSpace(params.ExpertID)
	if expertID == "" {
		expertID = strings.TrimSpace(params.Session.ExpertID)
	}
	if expertID == "" {
		return TurnResult{}, fmt.Errorf("%w: expert_id is required", store.ErrValidation)
	}
	hasAttachments := len(params.Attachments) > 0
	userText := strings.TrimSpace(params.UserInput)
	if userText == "" && !hasAttachments {
		return TurnResult{}, fmt.Errorf("%w: input is required", store.ErrValidation)
	}
	modelInput := strings.TrimSpace(params.ModelInput)
	if modelInput == "" {
		modelInput = userText
		if modelInput == "" && hasAttachments {
			modelInput = attachmentOnlyModelPrompt
		}
	}

	provider := strings.ToLower(strings.TrimSpace(params.SDK.Provider))
	model := strings.TrimSpace(params.SDK.Model)

	userMsg, err := m.store.AppendChatMessage(ctx, store.AppendChatMessageParams{
		SessionID:   params.Session.ID,
		Role:        "user",
		ContentText: userText,
		ExpertID:    pointerString(expertID),
		Provider:    pointerString(provider),
		Model:       pointerString(model),
	})
	if err != nil {
		return TurnResult{}, err
	}
	if hasAttachments {
		attachments, err := m.persistTurnAttachments(ctx, userMsg, params.Attachments)
		if err != nil {
			return TurnResult{}, err
		}
		userMsg.Attachments = attachments
	}
	m.broadcast("chat.turn.started", map[string]any{
		"session_id":      params.Session.ID,
		"user_message_id": userMsg.ID,
		"turn":            userMsg.Turn,
		"expert_id":       expertID,
		"provider":        provider,
		"model":           model,
	})

	if err := m.ensureCompaction(ctx, params.Session, modelInput, params.SDK, params.Env); err != nil {
		return TurnResult{}, err
	}

	var anchor store.ChatAnchor
	if strings.EqualFold(provider, params.Session.Provider) && strings.EqualFold(model, params.Session.Model) {
		anchor, _ = m.store.GetChatAnchor(ctx, params.Session.ID)
	}

	usedSDK, respText, reasoningText, anchorUpdate, callMeta, err := m.callProviderWithFallbacks(ctx, params.Session, userMsg, params.SDK, params.Env, modelInput, anchor, params.Fallbacks)
	if err != nil {
		return TurnResult{}, err
	}
	provider = strings.ToLower(strings.TrimSpace(usedSDK.Provider))
	model = strings.TrimSpace(usedSDK.Model)

	assistantMsg, err := m.store.AppendChatMessage(ctx, store.AppendChatMessageParams{
		SessionID:         params.Session.ID,
		Role:              "assistant",
		ContentText:       respText,
		ExpertID:          pointerString(expertID),
		Provider:          pointerString(provider),
		Model:             pointerString(model),
		TokenIn:           callMeta.TokenIn,
		TokenOut:          callMeta.TokenOut,
		ProviderMessageID: anchorUpdate.ProviderMessageID,
	})
	if err != nil {
		return TurnResult{}, err
	}
	if anchorUpdate.PreviousResponse != nil || anchorUpdate.ContainerID != nil || anchorUpdate.ProviderMessageID != nil {
		_ = m.store.UpsertChatAnchor(ctx, store.UpsertChatAnchorParams{
			SessionID:         params.Session.ID,
			Provider:          provider,
			PreviousResponse:  anchorUpdate.PreviousResponse,
			ContainerID:       anchorUpdate.ContainerID,
			ProviderMessageID: anchorUpdate.ProviderMessageID,
		})
	}
	_, _ = m.store.UpdateChatSessionDefaults(ctx, store.UpdateChatSessionDefaultsParams{
		SessionID: params.Session.ID,
		ExpertID:  expertID,
		Provider:  provider,
		Model:     model,
	})

	m.broadcast("chat.turn.completed", map[string]any{
		"session_id":          params.Session.ID,
		"user_message_id":     userMsg.ID,
		"message":             assistantMsg,
		"reasoning_text":      reasoningText,
		"model_input":         callMeta.ModelInput,
		"context_mode":        callMeta.ContextMode,
		"token_in":            callMeta.TokenIn,
		"token_out":           callMeta.TokenOut,
		"cached_input_tokens": callMeta.CachedInputTokens,
	})

	return TurnResult{
		UserMessage:       userMsg,
		AssistantMessage:  assistantMsg,
		ReasoningText:     pointerString(reasoningText),
		ModelInput:        pointerString(callMeta.ModelInput),
		ContextMode:       pointerString(callMeta.ContextMode),
		CachedInputTokens: callMeta.CachedInputTokens,
	}, nil
}

func (m *Manager) callProviderWithFallbacks(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, fallbacks []runner.SDKFallback) (runner.SDKSpec, string, string, store.ChatAnchor, providerCallMeta, error) {
	respText, reasoningText, anchorUpdate, callMeta, err := m.callProviderWithAnchorRetry(ctx, sess, currentUser, sdk, env, modelInput, anchor)
	if err == nil {
		return sdk, respText, reasoningText, anchorUpdate, callMeta, nil
	}
	lastErr := err
	for _, fallback := range fallbacks {
		respText, reasoningText, anchorUpdate, callMeta, err = m.callProviderWithAnchorRetry(ctx, sess, currentUser, fallback.SDK, fallback.Env, modelInput, store.ChatAnchor{})
		if err == nil {
			return fallback.SDK, respText, reasoningText, anchorUpdate, callMeta, nil
		}
		lastErr = err
	}
	return runner.SDKSpec{}, "", "", store.ChatAnchor{}, providerCallMeta{}, lastErr
}

func (m *Manager) callProviderWithAnchorRetry(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor) (string, string, store.ChatAnchor, providerCallMeta, error) {
	provider := strings.ToLower(strings.TrimSpace(sdk.Provider))
	respText, reasoningText, anchorUpdate, callMeta, err := m.callProvider(ctx, sess, currentUser, sdk, env, modelInput, anchor)
	if err == nil {
		return respText, reasoningText, anchorUpdate, callMeta, nil
	}
	if (provider == "openai" && anchor.PreviousResponse != nil && strings.TrimSpace(*anchor.PreviousResponse) != "") ||
		(provider == "anthropic" && anchor.ContainerID != nil && strings.TrimSpace(*anchor.ContainerID) != "") {
		return m.callProvider(ctx, sess, currentUser, sdk, env, modelInput, store.ChatAnchor{})
	}
	return "", "", store.ChatAnchor{}, providerCallMeta{}, err
}

func (m *Manager) CompactSession(ctx context.Context, sessionID string, sdk runner.SDKSpec, env map[string]string) (store.ChatSession, *store.ChatCompaction, error) {
	if m == nil || m.store == nil {
		return store.ChatSession{}, nil, fmt.Errorf("chat manager not configured")
	}
	sess, err := m.store.GetChatSession(ctx, sessionID)
	if err != nil {
		return store.ChatSession{}, nil, err
	}
	messages, err := m.store.ListChatMessages(ctx, sessionID, 1000)
	if err != nil {
		return store.ChatSession{}, nil, err
	}
	messages = filterModelContextMessages(messages)
	hasAttachments, err := m.store.SessionHasAttachments(ctx, sessionID)
	if err != nil {
		return store.ChatSession{}, nil, err
	}
	if hasAttachments {
		return sess, nil, nil
	}
	rec, err := m.compactMessages(ctx, sess, messages, m.keepRecent, sdk, env)
	if err != nil {
		return store.ChatSession{}, nil, err
	}
	sess, err = m.store.GetChatSession(ctx, sessionID)
	if err != nil {
		return store.ChatSession{}, nil, err
	}
	if rec != nil {
		m.broadcast("chat.session.compacted", rec)
	}
	return sess, rec, nil
}

func (m *Manager) ensureCompaction(ctx context.Context, sess store.ChatSession, input string, sdk runner.SDKSpec, env map[string]string) error {
	hasAttachments, err := m.store.SessionHasAttachments(ctx, sess.ID)
	if err != nil {
		return err
	}
	if hasAttachments {
		return nil
	}
	messages, err := m.store.ListChatMessages(ctx, sess.ID, 1000)
	if err != nil {
		return err
	}
	messages = filterModelContextMessages(messages)

	ratio := m.usageRatio(sess, messages, input, m.keepRecent)
	if ratio < m.softRatio {
		return nil
	}
	_, err = m.compactMessages(ctx, sess, messages, m.keepRecent, sdk, env)
	if err != nil {
		return err
	}

	sess, err = m.store.GetChatSession(ctx, sess.ID)
	if err != nil {
		return err
	}
	messages, err = m.store.ListChatMessages(ctx, sess.ID, 1000)
	if err != nil {
		return err
	}
	messages = filterModelContextMessages(messages)
	ratio = m.usageRatio(sess, messages, input, m.keepRecent)
	if ratio >= m.forceRatio {
		forceKeep := m.keepRecent / 2
		if forceKeep < 4 {
			forceKeep = 4
		}
		_, err = m.compactMessages(ctx, sess, messages, forceKeep, sdk, env)
		if err != nil {
			return err
		}
		sess, err = m.store.GetChatSession(ctx, sess.ID)
		if err != nil {
			return err
		}
		messages, err = m.store.ListChatMessages(ctx, sess.ID, 1000)
		if err != nil {
			return err
		}
		messages = filterModelContextMessages(messages)
		ratio = m.usageRatio(sess, messages, input, forceKeep)
	}
	if ratio >= m.hardRatio {
		return fmt.Errorf("context overflow after compaction (ratio=%.2f)", ratio)
	}
	return nil
}

func (m *Manager) compactMessages(ctx context.Context, sess store.ChatSession, messages []store.ChatMessage, keepRecent int, sdk runner.SDKSpec, env map[string]string) (*store.ChatCompaction, error) {
	messages = filterModelContextMessages(messages)
	if keepRecent < 2 {
		keepRecent = 2
	}
	if len(messages) <= keepRecent {
		return nil, nil
	}
	pivot := len(messages) - keepRecent
	old := messages[:pivot]
	if len(old) == 0 {
		return nil, nil
	}
	before := estimateTokens(renderConversation(sess.Summary, messages, "", keepRecent+len(old)))
	delta, mergedSummary := m.generateCompactionSummary(ctx, sess.Summary, old, keepRecent, sdk, env)
	if strings.TrimSpace(mergedSummary) == "" || strings.TrimSpace(delta) == "" {
		return nil, nil
	}
	if _, err := m.store.UpdateChatSummary(ctx, sess.ID, mergedSummary); err != nil {
		return nil, err
	}
	recent := messages[pivot:]
	after := estimateTokens(renderConversation(pointerString(mergedSummary), recent, "", keepRecent))
	rec, err := m.store.CreateChatCompaction(ctx, store.CreateChatCompactionParams{
		SessionID:    sess.ID,
		FromTurn:     old[0].Turn,
		ToTurn:       old[len(old)-1].Turn,
		BeforeTokens: before,
		AfterTokens:  after,
		SummaryDelta: delta,
	})
	if err != nil {
		return nil, err
	}
	m.broadcast("chat.session.compacted", rec)
	return &rec, nil
}

func summarizeMessages(messages []store.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	const maxLines = 20
	lines := make([]string, 0, maxLines)
	for _, msg := range messages {
		text := strings.TrimSpace(msg.ContentText)
		if text == "" {
			continue
		}
		text = strings.ReplaceAll(text, "\n", " ")
		if len(text) > 180 {
			text = text[:180] + "..."
		}
		role := strings.ToUpper(msg.Role)
		lines = append(lines, role+": "+text)
		if len(lines) >= maxLines {
			break
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "Compacted context:\n- " + strings.Join(lines, "\n- ")
}

func (m *Manager) generateCompactionSummary(ctx context.Context, existingSummary *string, old []store.ChatMessage, keepRecent int, sdk runner.SDKSpec, env map[string]string) (summaryDelta string, mergedSummary string) {
	if ctx == nil {
		ctx = context.Background()
	}

	if m != nil {
		if out, err := m.summarizeWithLLM(ctx, sdk, env, buildCompactionPrompt(existingSummary, old)); err == nil {
			if v := strings.TrimSpace(out); v != "" {
				// LLM path: treat output as the full new summary.
				return v, v
			}
		}
	}

	// Fallback path: deterministic local summary delta appended to existing summary.
	delta := summarizeMessages(old)
	if strings.TrimSpace(delta) == "" {
		return "", ""
	}
	merged := strings.TrimSpace(strings.Join([]string{stringOrEmpty(existingSummary), delta}, "\n\n"))
	return delta, merged
}

func buildCompactionPrompt(existingSummary *string, old []store.ChatMessage) string {
	var b strings.Builder
	b.WriteString("你是一个对话记录压缩器。请把以下“历史对话片段”压缩为可持续续聊的会话摘要。\n")
	b.WriteString("要求：\n")
	b.WriteString("- 使用简体中文\n")
	b.WriteString("- 只输出摘要正文（不要解释你的做法）\n")
	b.WriteString("- 尽量保留：关键事实、关键约束、重要决策、未决问题与待办\n")
	b.WriteString("\n")

	if s := strings.TrimSpace(stringOrEmpty(existingSummary)); s != "" {
		b.WriteString("已有会话摘要（可重写/更新）：\n")
		b.WriteString(s)
		b.WriteString("\n\n")
	}

	b.WriteString("历史对话片段（按时间顺序）：\n")
	for _, msg := range old {
		text := strings.TrimSpace(msg.ContentText)
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%d %s: %s\n", msg.Turn, strings.ToUpper(msg.Role), text))
	}
	b.WriteString("\n输出格式建议（可按需增删小节）：\n")
	b.WriteString("目标:\n- ...\n\n已知事实:\n- ...\n\n约束/偏好:\n- ...\n\n关键结论/决策:\n- ...\n\n未决问题:\n- ...\n\n待办:\n- ...\n")
	return strings.TrimSpace(b.String())
}

func (m *Manager) summarizeWithLLM(ctx context.Context, sdk runner.SDKSpec, env map[string]string, prompt string) (string, error) {
	if m == nil {
		return "", errors.New("chat manager not configured")
	}
	provider := strings.ToLower(strings.TrimSpace(sdk.Provider))
	if provider == "" {
		return "", errors.New("summarizer provider is required")
	}
	if provider == "demo" {
		return "", errors.New("demo provider does not support summarization")
	}
	if strings.TrimSpace(sdk.Model) == "" {
		return "", errors.New("summarizer model is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("summarizer prompt is required")
	}

	system := strings.TrimSpace(sdk.Instructions)
	if system == "" {
		system = "你是一个对话摘要生成器。"
	}
	temp := sdk.Temperature
	if temp == nil {
		v := 0.1
		temp = &v
	}
	maxTokens := sdk.MaxOutputTokens
	if maxTokens <= 0 || maxTokens > 1200 {
		maxTokens = 1200
	}

	switch provider {
	case "openai":
		body := openai_responses.ResponseNewParams{
			Model: openai_shared.ResponsesModel(strings.TrimSpace(sdk.Model)),
			Input: openai_responses.ResponseNewParamsInputUnion{
				OfString: openai.String(prompt),
			},
			Instructions:    openai.String(system),
			MaxOutputTokens: openai.Int(int64(maxTokens)),
			Temperature:     openai.Float(*temp),
		}

		opts := make([]openai_option.RequestOption, 0, 4)
		if v := strings.TrimSpace(env["OPENAI_API_KEY"]); v != "" {
			opts = append(opts, openai_option.WithAPIKey(v))
		}
		if v := strings.TrimSpace(env["OPENAI_ORG_ID"]); v != "" {
			opts = append(opts, openai_option.WithOrganization(v))
		}
		if v := strings.TrimSpace(env["OPENAI_PROJECT_ID"]); v != "" {
			opts = append(opts, openai_option.WithProject(v))
		}
		if baseURL := strings.TrimSpace(sdk.BaseURL); baseURL != "" {
			opts = append(opts, openai_option.WithBaseURL(runner.NormalizeBaseURL("openai", baseURL)))
		} else if baseURL := strings.TrimSpace(env["OPENAI_BASE_URL"]); baseURL != "" {
			opts = append(opts, openai_option.WithBaseURL(runner.NormalizeBaseURL("openai", baseURL)))
		}

		stream := m.openaiClient.Responses.NewStreaming(ctx, body, opts...)
		if stream == nil {
			return "", errors.New("openai summary stream is nil")
		}
		defer stream.Close()

		var text strings.Builder
		for stream.Next() {
			ev := stream.Current()
			switch ev.Type {
			case "response.output_text.delta":
				delta := ev.AsResponseOutputTextDelta().Delta
				if delta == "" {
					continue
				}
				text.WriteString(delta)
			case "error":
				msg := strings.TrimSpace(ev.AsError().Message)
				if msg != "" {
					return "", errors.New(msg)
				}
			}
		}
		if err := stream.Err(); err != nil {
			return "", err
		}
		return strings.TrimSpace(text.String()), nil
	case "anthropic":
		body := anthropic.MessageNewParams{
			Model:     anthropic.Model(strings.TrimSpace(sdk.Model)),
			MaxTokens: int64(maxTokens),
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
			},
			System: []anthropic.TextBlockParam{{Text: system}},
		}
		body.Temperature = anthropic.Float(*temp)

		opts := make([]anthropic_option.RequestOption, 0, 2)
		if v := strings.TrimSpace(env["ANTHROPIC_API_KEY"]); v != "" {
			opts = append(opts, anthropic_option.WithAPIKey(v))
		}
		if baseURL := strings.TrimSpace(sdk.BaseURL); baseURL != "" {
			opts = append(opts, anthropic_option.WithBaseURL(runner.NormalizeBaseURL("anthropic", baseURL)))
		} else if baseURL := strings.TrimSpace(env["ANTHROPIC_BASE_URL"]); baseURL != "" {
			opts = append(opts, anthropic_option.WithBaseURL(runner.NormalizeBaseURL("anthropic", baseURL)))
		}

		stream := m.anthropicClient.Messages.NewStreaming(ctx, body, opts...)
		if stream == nil {
			return "", errors.New("anthropic summary stream is nil")
		}
		defer stream.Close()

		var text strings.Builder
		for stream.Next() {
			ev := stream.Current()
			switch ev.Type {
			case "content_block_delta":
				de := ev.AsContentBlockDelta()
				if strings.TrimSpace(de.Delta.Type) != "text_delta" {
					continue
				}
				delta := de.Delta.AsTextDelta().Text
				if delta == "" {
					continue
				}
				text.WriteString(delta)
			}
		}
		if err := stream.Err(); err != nil {
			return "", err
		}
		return strings.TrimSpace(text.String()), nil
	default:
		return "", fmt.Errorf("unsupported summarizer provider %q", provider)
	}
}

func (m *Manager) usageRatio(sess store.ChatSession, messages []store.ChatMessage, input string, keepRecent int) float64 {
	context := renderConversation(sess.Summary, messages, input, keepRecent)
	toks := estimateTokens(context)
	if m.contextWindow <= 0 {
		return 0
	}
	return float64(toks) / float64(m.contextWindow)
}

func renderConversation(summary *string, messages []store.ChatMessage, input string, keepRecent int) string {
	messages = filterModelContextMessages(messages)
	parts := make([]string, 0, 4)
	if s := strings.TrimSpace(stringOrEmpty(summary)); s != "" {
		parts = append(parts, "Session summary:\n"+s)
	}
	if keepRecent <= 0 {
		keepRecent = len(messages)
	}
	if len(messages) > keepRecent {
		messages = messages[len(messages)-keepRecent:]
	}
	if len(messages) > 0 {
		var b strings.Builder
		b.WriteString("Recent conversation:\n")
		for _, msg := range messages {
			b.WriteString(strings.ToUpper(msg.Role))
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(msg.ContentText))
			b.WriteString("\n")
		}
		parts = append(parts, b.String())
	}
	if strings.TrimSpace(input) != "" {
		parts = append(parts, "Current user input:\n"+strings.TrimSpace(input))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func filterModelContextMessages(messages []store.ChatMessage) []store.ChatMessage {
	if len(messages) == 0 {
		return messages
	}
	filtered := make([]store.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Provider != nil && strings.EqualFold(strings.TrimSpace(*msg.Provider), store.ForkContextProvider) {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}

func estimateTokens(s string) int64 {
	if s == "" {
		return 0
	}
	n := len([]byte(s))
	if n <= 0 {
		return 0
	}
	return int64((n + 3) / 4)
}

func (m *Manager) callProvider(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor) (string, string, store.ChatAnchor, providerCallMeta, error) {
	provider := strings.ToLower(strings.TrimSpace(sdk.Provider))
	sdk.Provider = provider

	switch provider {
	case "openai":
		return m.callOpenAI(ctx, sess, currentUser, sdk, env, modelInput, anchor)
	case "anthropic":
		return m.callAnthropic(ctx, sess, currentUser, sdk, env, modelInput, anchor)
	case "demo":
		return m.callDemo(sess, modelInput), "", store.ChatAnchor{}, providerCallMeta{
			ModelInput:  strings.TrimSpace(modelInput),
			ContextMode: "demo",
		}, nil
	default:
		return "", "", store.ChatAnchor{}, providerCallMeta{}, fmt.Errorf("unsupported provider %q", provider)
	}
}

func (m *Manager) callDemo(sess store.ChatSession, modelInput string) string {
	out := "demo: " + strings.TrimSpace(modelInput)
	if out == "demo:" {
		out = "demo: ok"
	}
	m.broadcast("chat.turn.delta", map[string]any{
		"session_id": sess.ID,
		"delta":      out,
	})
	return out
}

func (m *Manager) callOpenAI(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor) (string, string, store.ChatAnchor, providerCallMeta, error) {
	body := openai_responses.ResponseNewParams{
		Model: openai_shared.ResponsesModel(strings.TrimSpace(sdk.Model)),
	}
	if supportsOpenAIReasoning(sdk.Model) {
		body.Reasoning = openai_shared.ReasoningParam{
			Effort:  openai_shared.ReasoningEffortMedium,
			Summary: openai_shared.ReasoningSummaryAuto,
		}
	}
	contextMode := "anchor"
	debugInput := strings.TrimSpace(modelInput)
	instructions := strings.TrimSpace(sdk.Instructions)
	currentHasAttachments := len(currentUser.Attachments) > 0
	if anchor.PreviousResponse == nil || strings.TrimSpace(*anchor.PreviousResponse) == "" {
		contextMode = "reconstructed"
		msgs, err := m.store.ListChatMessages(ctx, sess.ID, 1000)
		if err == nil && anyMessageHasAttachments(msgs) {
			msgs = applyCurrentModelInput(msgs, currentUser, modelInput)
			items, debug, err := buildOpenAIReconstructedInput(sess.Summary, msgs, m.keepRecent)
			if err != nil {
				return "", "", store.ChatAnchor{}, providerCallMeta{}, err
			}
			body.Input = openai_responses.ResponseNewParamsInputUnion{OfInputItemList: items}
			debugInput = debug
			instructions = joinProviderInstructions(instructions, sess.Summary)
		} else {
			inputText := strings.TrimSpace(modelInput)
			if err == nil {
				inputText = renderConversation(sess.Summary, msgs, modelInput, m.keepRecent)
			}
			body.Input = openai_responses.ResponseNewParamsInputUnion{OfString: openai.String(inputText)}
			debugInput = inputText
		}
	} else if currentHasAttachments {
		content, err := buildOpenAIMessageContent(modelInput, currentUser.Attachments)
		if err != nil {
			return "", "", store.ChatAnchor{}, providerCallMeta{}, err
		}
		body.Input = openai_responses.ResponseNewParamsInputUnion{OfInputItemList: openai_responses.ResponseInputParam{
			openai_responses.ResponseInputItemParamOfMessage(content, openai_responses.EasyInputMessageRoleUser),
		}}
		debugInput = buildCurrentInputDebug(modelInput, currentUser.Attachments)
	} else {
		body.Input = openai_responses.ResponseNewParamsInputUnion{OfString: openai.String(strings.TrimSpace(modelInput))}
	}
	body.Store = openai.Bool(true)

	if instructions != "" {
		body.Instructions = openai.String(instructions)
	}
	if sdk.MaxOutputTokens > 0 {
		body.MaxOutputTokens = openai.Int(int64(sdk.MaxOutputTokens))
	}
	if sdk.Temperature != nil {
		body.Temperature = openai.Float(*sdk.Temperature)
	}
	if anchor.PreviousResponse != nil && strings.TrimSpace(*anchor.PreviousResponse) != "" {
		body.PreviousResponseID = openai.String(strings.TrimSpace(*anchor.PreviousResponse))
	}

	opts := make([]openai_option.RequestOption, 0, 4)
	if v := strings.TrimSpace(env["OPENAI_API_KEY"]); v != "" {
		opts = append(opts, openai_option.WithAPIKey(v))
	}
	if v := strings.TrimSpace(env["OPENAI_ORG_ID"]); v != "" {
		opts = append(opts, openai_option.WithOrganization(v))
	}
	if v := strings.TrimSpace(env["OPENAI_PROJECT_ID"]); v != "" {
		opts = append(opts, openai_option.WithProject(v))
	}
	if baseURL := strings.TrimSpace(sdk.BaseURL); baseURL != "" {
		opts = append(opts, openai_option.WithBaseURL(runner.NormalizeBaseURL("openai", baseURL)))
	} else if baseURL := strings.TrimSpace(env["OPENAI_BASE_URL"]); baseURL != "" {
		opts = append(opts, openai_option.WithBaseURL(runner.NormalizeBaseURL("openai", baseURL)))
	}

	stream := m.openaiClient.Responses.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, errors.New("openai stream is nil")
	}
	defer stream.Close()

	var text strings.Builder
	var reasoning strings.Builder
	var responseID string
	var sawReasoningDelta bool
	var tokenIn int64
	var tokenOut int64
	var cachedInputTokens int64
	var usageSeen bool
	for stream.Next() {
		ev := stream.Current()
		switch ev.Type {
		case "response.created":
			responseID = strings.TrimSpace(ev.AsResponseCreated().Response.ID)
		case "response.completed":
			id := strings.TrimSpace(ev.AsResponseCompleted().Response.ID)
			if id != "" {
				responseID = id
			}
			usage := ev.AsResponseCompleted().Response.Usage
			tokenIn = usage.InputTokens
			tokenOut = usage.OutputTokens
			cachedInputTokens = usage.InputTokensDetails.CachedTokens
			usageSeen = true
		case "response.output_text.delta":
			delta := ev.AsResponseOutputTextDelta().Delta
			if delta == "" {
				continue
			}
			text.WriteString(delta)
			m.broadcast("chat.turn.delta", map[string]any{
				"session_id": sess.ID,
				"delta":      delta,
			})
		case "response.reasoning_summary_text.delta":
			delta := ev.AsResponseReasoningSummaryTextDelta().Delta
			if delta == "" {
				continue
			}
			sawReasoningDelta = true
			reasoning.WriteString(delta)
			m.broadcast("chat.turn.thinking.delta", map[string]any{
				"session_id": sess.ID,
				"delta":      delta,
			})
		case "response.reasoning_summary_text.done":
			done := ev.AsResponseReasoningSummaryTextDone().Text
			if done == "" || sawReasoningDelta {
				continue
			}
			reasoning.WriteString(done)
			m.broadcast("chat.turn.thinking.delta", map[string]any{
				"session_id": sess.ID,
				"delta":      done,
			})
		case "error":
			msg := strings.TrimSpace(ev.AsError().Message)
			if msg != "" {
				return "", "", store.ChatAnchor{}, providerCallMeta{}, errors.New(msg)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, err
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		out = "(empty response)"
	}
	var prevID *string
	if responseID != "" {
		prevID = pointerString(responseID)
	}
	anchorOut := store.ChatAnchor{
		SessionID:         sess.ID,
		Provider:          "openai",
		PreviousResponse:  prevID,
		ProviderMessageID: prevID,
	}
	meta := providerCallMeta{
		ModelInput:  debugInput,
		ContextMode: contextMode,
	}
	if usageSeen {
		meta.TokenIn = pointerInt64(tokenIn)
		meta.TokenOut = pointerInt64(tokenOut)
		meta.CachedInputTokens = pointerInt64(cachedInputTokens)
	}
	return out, strings.TrimSpace(reasoning.String()), anchorOut, meta, nil
}

func (m *Manager) callAnthropic(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor) (string, string, store.ChatAnchor, providerCallMeta, error) {
	maxTokens := sdk.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	debugInput := strings.TrimSpace(modelInput)
	contextMode := "anchor"
	systemText := strings.TrimSpace(sdk.Instructions)
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(strings.TrimSpace(modelInput))),
	}
	currentHasAttachments := len(currentUser.Attachments) > 0
	if anchor.ContainerID == nil || strings.TrimSpace(*anchor.ContainerID) == "" {
		contextMode = "reconstructed"
		msgs, err := m.store.ListChatMessages(ctx, sess.ID, 1000)
		if err == nil && anyMessageHasAttachments(msgs) {
			msgs = applyCurrentModelInput(msgs, currentUser, modelInput)
			builtMessages, debug, err := buildAnthropicReconstructedMessages(sess.Summary, msgs, m.keepRecent)
			if err != nil {
				return "", "", store.ChatAnchor{}, providerCallMeta{}, err
			}
			messages = builtMessages
			debugInput = debug
			systemText = joinProviderInstructions(systemText, sess.Summary)
		} else {
			inputText := strings.TrimSpace(modelInput)
			if err == nil {
				inputText = renderConversation(sess.Summary, msgs, modelInput, m.keepRecent)
			}
			messages = []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(inputText)),
			}
			debugInput = inputText
		}
	} else if currentHasAttachments {
		blocks, err := buildAnthropicMessageBlocks(modelInput, currentUser.Attachments)
		if err != nil {
			return "", "", store.ChatAnchor{}, providerCallMeta{}, err
		}
		messages = []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)}
		debugInput = buildCurrentInputDebug(modelInput, currentUser.Attachments)
	}

	body := anthropic.MessageNewParams{
		Model:     anthropic.Model(strings.TrimSpace(sdk.Model)),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
	}
	if systemText != "" {
		body.System = []anthropic.TextBlockParam{{Text: systemText}}
	}
	if sdk.Temperature != nil {
		body.Temperature = anthropic.Float(*sdk.Temperature)
	}
	if anchor.ContainerID != nil && strings.TrimSpace(*anchor.ContainerID) != "" {
		body.Container = anthropic.String(strings.TrimSpace(*anchor.ContainerID))
	}
	if supportsAnthropicThinking(sdk.Model) {
		if budget := defaultAnthropicThinkingBudget(maxTokens); budget > 0 {
			body.Thinking = anthropic.ThinkingConfigParamOfEnabled(budget)
		}
	}

	opts := make([]anthropic_option.RequestOption, 0, 2)
	if v := strings.TrimSpace(env["ANTHROPIC_API_KEY"]); v != "" {
		opts = append(opts, anthropic_option.WithAPIKey(v))
	}
	if baseURL := strings.TrimSpace(sdk.BaseURL); baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(runner.NormalizeBaseURL("anthropic", baseURL)))
	} else if baseURL := strings.TrimSpace(env["ANTHROPIC_BASE_URL"]); baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(runner.NormalizeBaseURL("anthropic", baseURL)))
	}

	stream := m.anthropicClient.Messages.NewStreaming(ctx, body, opts...)
	if stream == nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, errors.New("anthropic stream is nil")
	}
	defer stream.Close()

	var text strings.Builder
	var thinking strings.Builder
	var containerID string
	var providerMsgID string
	var tokenIn int64
	var tokenOut int64
	var cachedInputTokens int64
	var usageSeen bool
	for stream.Next() {
		ev := stream.Current()
		switch ev.Type {
		case "message_start":
			me := ev.AsMessageStart().Message
			providerMsgID = strings.TrimSpace(me.ID)
			containerID = strings.TrimSpace(me.Container.ID)
		case "message_delta":
			usage := ev.AsMessageDelta().Usage
			tokenIn = usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
			tokenOut = usage.OutputTokens
			cachedInputTokens = usage.CacheReadInputTokens
			usageSeen = true
		case "content_block_start":
			block := ev.AsContentBlockStart().ContentBlock
			if strings.TrimSpace(block.Type) != "thinking" {
				continue
			}
			delta := block.AsThinking().Thinking
			if delta == "" {
				continue
			}
			thinking.WriteString(delta)
			m.broadcast("chat.turn.thinking.delta", map[string]any{
				"session_id": sess.ID,
				"delta":      delta,
			})
		case "content_block_delta":
			de := ev.AsContentBlockDelta()
			switch strings.TrimSpace(de.Delta.Type) {
			case "text_delta":
				delta := de.Delta.AsTextDelta().Text
				if delta == "" {
					continue
				}
				text.WriteString(delta)
				m.broadcast("chat.turn.delta", map[string]any{
					"session_id": sess.ID,
					"delta":      delta,
				})
			case "input_json_delta":
				delta := de.Delta.AsInputJSONDelta().PartialJSON
				if delta == "" {
					continue
				}
				text.WriteString(delta)
				m.broadcast("chat.turn.delta", map[string]any{
					"session_id": sess.ID,
					"delta":      delta,
				})
			case "thinking_delta":
				delta := de.Delta.AsThinkingDelta().Thinking
				if delta == "" {
					continue
				}
				thinking.WriteString(delta)
				m.broadcast("chat.turn.thinking.delta", map[string]any{
					"session_id": sess.ID,
					"delta":      delta,
				})
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, err
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		out = "(empty response)"
	}
	var cid *string
	if containerID != "" {
		cid = pointerString(containerID)
	}
	var pid *string
	if providerMsgID != "" {
		pid = pointerString(providerMsgID)
	}
	anchorOut := store.ChatAnchor{
		SessionID:         sess.ID,
		Provider:          "anthropic",
		ContainerID:       cid,
		ProviderMessageID: pid,
	}
	meta := providerCallMeta{
		ModelInput:  debugInput,
		ContextMode: contextMode,
	}
	if usageSeen {
		meta.TokenIn = pointerInt64(tokenIn)
		meta.TokenOut = pointerInt64(tokenOut)
		meta.CachedInputTokens = pointerInt64(cachedInputTokens)
	}
	return out, strings.TrimSpace(thinking.String()), anchorOut, meta, nil
}

func (m *Manager) broadcast(typ string, payload any) {
	if m == nil || m.hub == nil {
		return
	}
	env := ws.Envelope{
		Type:    typ,
		Ts:      time.Now().UnixMilli(),
		Payload: payload,
	}
	if b, err := json.Marshal(env); err == nil {
		m.hub.Broadcast(b)
	}
}

func pointerString(v string) *string {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return &s
}

func pointerInt64(v int64) *int64 {
	return &v
}

func stringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func supportsOpenAIReasoning(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "o1") ||
		strings.HasPrefix(m, "o3") ||
		strings.HasPrefix(m, "o4") ||
		strings.HasPrefix(m, "gpt-5")
}

func supportsAnthropicThinking(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "claude-3-7") ||
		strings.Contains(m, "claude-4") ||
		strings.Contains(m, "sonnet-4") ||
		strings.Contains(m, "opus-4") ||
		strings.Contains(m, "haiku-4")
}

func defaultAnthropicThinkingBudget(maxTokens int) int64 {
	if maxTokens < 1025 {
		return 0
	}
	budget := int64(maxTokens / 3)
	if budget < 1024 {
		budget = 1024
	}
	if budget >= int64(maxTokens) {
		budget = int64(maxTokens) - 1
	}
	if budget < 1024 {
		return 0
	}
	return budget
}
