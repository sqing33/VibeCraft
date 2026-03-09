package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"

	"vibe-tree/backend/internal/openaicompat"
	"vibe-tree/backend/internal/runner"
)

const (
	defaultThinkingTranslationMinChars   = 80
	defaultThinkingTranslationForceChars = 220
	defaultThinkingTranslationIdle       = 900 * time.Millisecond
)

type ThinkingTranslationSpec struct {
	Provider       string
	Model          string
	BaseURL        string
	Env            map[string]string
	OpenAIAPIStyle string // 可选，指定 openai api style（"responses" 或 "chat_completions"），为空则自动探测
}

type ThinkingTranslatorFunc func(ctx context.Context, spec ThinkingTranslationSpec, text string) (string, error)

type thinkingTranslationRuntime struct {
	manager           *Manager
	sessionID         string
	turnID            string
	spec              *ThinkingTranslationSpec
	buffer            string
	bufferEntryID     string
	lastDelta         time.Time
	failed            bool
	translatedByEntry map[string]string
	entryOrder        []string
}

func newThinkingTranslationRuntime(manager *Manager, sessionID, turnID string, spec *ThinkingTranslationSpec) *thinkingTranslationRuntime {
	if manager == nil || spec == nil {
		return nil
	}
	return &thinkingTranslationRuntime{
		manager:   manager,
		sessionID: sessionID,
		turnID:    strings.TrimSpace(turnID),
		spec: &ThinkingTranslationSpec{
			Provider: strings.ToLower(strings.TrimSpace(spec.Provider)),
			Model:    strings.TrimSpace(spec.Model),
			BaseURL:  strings.TrimSpace(spec.BaseURL),
			Env:      cloneEnvMap(spec.Env),
		},
	}
}

func (t *thinkingTranslationRuntime) applied() bool {
	return t != nil && t.spec != nil
}

func (t *thinkingTranslationRuntime) failedState() bool {
	return t != nil && t.failed
}

func (t *thinkingTranslationRuntime) translatedText() string {
	if t == nil {
		return ""
	}
	var out strings.Builder
	for _, entryID := range t.entryOrder {
		segment := strings.TrimSpace(t.translatedByEntry[entryID])
		if segment == "" {
			continue
		}
		out.WriteString(segment)
	}
	return strings.TrimSpace(out.String())
}

func (t *thinkingTranslationRuntime) rememberEntry(entryID string) {
	if t == nil {
		return
	}
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		entryID = "thinking"
	}
	if t.translatedByEntry == nil {
		t.translatedByEntry = map[string]string{}
	}
	for _, existing := range t.entryOrder {
		if existing == entryID {
			return
		}
	}
	t.entryOrder = append(t.entryOrder, entryID)
}

func (t *thinkingTranslationRuntime) resetEntry(ctx context.Context, entryID string) {
	if t == nil || t.spec == nil || t.failed {
		return
	}
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return
	}
	t.rememberEntry(entryID)
	if t.translatedByEntry != nil {
		t.translatedByEntry[entryID] = ""
	}
	if t.manager != nil && strings.TrimSpace(t.turnID) != "" {
		t.manager.persistTurnTranslationWarn(ctx, t.turnID, entryID, "", true)
	}
}

func (t *thinkingTranslationRuntime) add(ctx context.Context, entryID, delta string) {
	if t == nil || t.spec == nil || t.failed {
		return
	}
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		entryID = "thinking"
	}
	delta = strings.TrimSpace(delta)
	if delta == "" {
		return
	}
	now := time.Now()
	if !t.lastDelta.IsZero() && now.Sub(t.lastDelta) >= t.manager.thinkingTranslationIdle && strings.TrimSpace(t.buffer) != "" {
		t.flush(ctx, true)
	}
	if entryID != "" && t.bufferEntryID != "" && entryID != t.bufferEntryID && strings.TrimSpace(t.buffer) != "" {
		t.flush(ctx, true)
	}
	if t.bufferEntryID == "" {
		t.bufferEntryID = entryID
	}
	t.rememberEntry(t.bufferEntryID)
	t.buffer += delta
	t.lastDelta = now
	t.flush(ctx, false)
}

func (t *thinkingTranslationRuntime) complete(ctx context.Context) {
	if t == nil || t.spec == nil || t.failed || strings.TrimSpace(t.buffer) == "" {
		return
	}
	t.flush(ctx, true)
}

func (t *thinkingTranslationRuntime) flush(ctx context.Context, force bool) {
	for {
		entryID := strings.TrimSpace(t.bufferEntryID)
		segment := t.nextSegment(force)
		if segment == "" {
			return
		}
		translated, err := t.manager.translateThinking(ctx, *t.spec, segment)
		if err != nil {
			t.failed = true
			payload := map[string]any{
				"session_id": t.sessionID,
				"error":      err.Error(),
			}
			if entryID != "" {
				payload["entry_id"] = entryID
				if t.manager != nil && strings.TrimSpace(t.turnID) != "" {
					t.manager.persistTurnTranslationFailedWarn(ctx, t.turnID, entryID)
				}
			}
			t.manager.broadcast("chat.turn.thinking.translation.failed", payload)
			return
		}
		translated = strings.TrimSpace(translated)
		if translated == "" {
			if strings.TrimSpace(t.buffer) == "" {
				t.bufferEntryID = ""
			}
			continue
		}
		t.rememberEntry(entryID)
		if t.translatedByEntry == nil {
			t.translatedByEntry = map[string]string{}
		}
		t.translatedByEntry[entryID] += translated
		if t.manager != nil && strings.TrimSpace(t.turnID) != "" {
			t.manager.persistTurnTranslationWarn(ctx, t.turnID, entryID, translated, false)
		}
		payload := map[string]any{
			"session_id": t.sessionID,
			"delta":      translated,
		}
		if entryID != "" {
			payload["entry_id"] = entryID
		}
		t.manager.broadcast("chat.turn.thinking.translation.delta", payload)
		if strings.TrimSpace(t.buffer) == "" {
			t.bufferEntryID = ""
		}
		if force {
			continue
		}
	}
}

func (t *thinkingTranslationRuntime) nextSegment(force bool) string {
	if t == nil {
		return ""
	}
	buffer := strings.TrimSpace(t.buffer)
	if buffer == "" {
		t.buffer = ""
		return ""
	}
	if force {
		t.buffer = ""
		return buffer
	}
	if utf8.RuneCountInString(buffer) >= t.manager.thinkingTranslationForceChars {
		t.buffer = ""
		return buffer
	}
	if utf8.RuneCountInString(buffer) < t.manager.thinkingTranslationMinChars {
		return ""
	}
	boundaryEnd := lastTranslationBoundaryEnd(buffer)
	if boundaryEnd <= 0 {
		return ""
	}
	segment := strings.TrimSpace(buffer[:boundaryEnd])
	t.buffer = strings.TrimLeft(buffer[boundaryEnd:], " \t\r\n")
	return segment
}

func lastTranslationBoundaryEnd(text string) int {
	idx := strings.LastIndexAny(text, "。！？.!?；;\n")
	if idx < 0 {
		return -1
	}
	r, size := utf8.DecodeRuneInString(text[idx:])
	if r == utf8.RuneError && size == 0 {
		return -1
	}
	return idx + size
}

func (m *Manager) translateThinking(ctx context.Context, spec ThinkingTranslationSpec, text string) (string, error) {
	if m == nil {
		return "", errors.New("chat manager not configured")
	}
	if m.thinkingTranslator != nil {
		return m.thinkingTranslator(ctx, spec, text)
	}
	sdk := runner.SDKSpec{
		Provider: spec.Provider,
		Model:    spec.Model,
		BaseURL:  spec.BaseURL,
	}
	return m.generatePlainTextWithLLM(
		ctx,
		sdk,
		spec.Env,
		spec.OpenAIAPIStyle,
		buildThinkingTranslationPrompt(text),
		"你是一个思考过程翻译器。请把用户提供的内容忠实翻译为简体中文，保留原有段落、列表、代码块、URL、命令、模型名和专有名词，不要总结，不要删减，不要补充解释，只输出译文。",
		1600,
		0.1,
	)
}

// TranslateText 将任意文本翻译为简体中文，复用思考翻译模型配置。
func (m *Manager) TranslateText(ctx context.Context, spec ThinkingTranslationSpec, text string) (string, error) {
	if m == nil {
		return "", errors.New("chat manager not configured")
	}
	sdk := runner.SDKSpec{
		Provider: spec.Provider,
		Model:    spec.Model,
		BaseURL:  spec.BaseURL,
	}
	system := "你是一个简介翻译器。请把用户提供的内容简洁地翻译为简体中文，保留专有名词、命令名、模型名，只输出译文，不要解释。"
	prompt := "请将以下内容翻译成简体中文：\n\n" + strings.TrimSpace(text)
	return m.generatePlainTextWithLLM(ctx, sdk, spec.Env, spec.OpenAIAPIStyle, prompt, system, 500, 0.3)
}

func buildThinkingTranslationPrompt(text string) string {
	return strings.TrimSpace("请将以下内容翻译成简体中文，并尽量保持原始结构与语气：\n\n" + strings.TrimSpace(text))
}

func (m *Manager) generatePlainTextWithLLM(ctx context.Context, sdk runner.SDKSpec, env map[string]string, apiStyle string, prompt, system string, maxTokens int, temperature float64) (string, error) {
	if m == nil {
		return "", errors.New("chat manager not configured")
	}
	provider := strings.ToLower(strings.TrimSpace(sdk.Provider))
	if provider == "" {
		return "", errors.New("provider is required")
	}
	if provider == "demo" {
		return "", errors.New("demo provider does not support llm text generation")
	}
	if strings.TrimSpace(sdk.Model) == "" {
		return "", errors.New("model is required")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is required")
	}
	if maxTokens <= 0 {
		maxTokens = 1200
	}

	switch provider {
	case "openai":
		request := openaicompat.TextRequest{
			Model:           strings.TrimSpace(sdk.Model),
			BaseURL:         strings.TrimSpace(sdk.BaseURL),
			APIKey:          strings.TrimSpace(env["OPENAI_API_KEY"]),
			Prompt:          prompt,
			Instructions:    strings.TrimSpace(system),
			MaxOutputTokens: maxTokens,
			Temperature:     &temperature,
		}
		style := openaicompat.NormalizeAPIStyle(apiStyle)
		if !style.Valid() {
			style = openaicompat.APIStyleResponses
		}
		out, _, err := openaicompat.CompleteText(ctx, style, request)
		return strings.TrimSpace(out), err
	case "anthropic":
		body := anthropic.MessageNewParams{
			Model:     anthropic.Model(strings.TrimSpace(sdk.Model)),
			MaxTokens: int64(maxTokens),
			Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
		}
		if strings.TrimSpace(system) != "" {
			body.System = []anthropic.TextBlockParam{{Text: strings.TrimSpace(system)}}
		}
		body.Temperature = anthropic.Float(temperature)
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
			return "", errors.New("anthropic text stream is nil")
		}
		defer stream.Close()
		var out strings.Builder
		for stream.Next() {
			ev := stream.Current()
			switch ev.Type {
			case "content_block_delta":
				de := ev.AsContentBlockDelta()
				if strings.TrimSpace(de.Delta.Type) != "text_delta" {
					continue
				}
				delta := de.Delta.AsTextDelta().Text
				if delta != "" {
					out.WriteString(delta)
				}
			}
		}
		if err := stream.Err(); err != nil {
			return "", err
		}
		return strings.TrimSpace(out.String()), nil
	default:
		return "", fmt.Errorf("unsupported thinking translation provider %q", provider)
	}
}

func cloneEnvMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
