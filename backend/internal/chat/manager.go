package chat

import (
	"bufio"
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

	"vibe-tree/backend/internal/cliruntime"
	"vibe-tree/backend/internal/openaicompat"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

const (
	defaultContextWindow          = int64(128_000)
	defaultSoftRatio              = 0.82
	defaultForceRatio             = 0.92
	defaultHardRatio              = 0.97
	defaultKeepRecent             = 12
	defaultCodexRuntimeIdleTTL    = 10 * time.Minute
	defaultCodexRuntimeReapTick   = time.Minute
	defaultOpenCodeBlankStepLimit = 40
)

type Options struct {
	SoftRatio                     float64
	ForceRatio                    float64
	HardRatio                     float64
	KeepRecent                    int
	ContextWindow                 int64
	ThinkingTranslationMinChars   int
	ThinkingTranslationForceChars int
	ThinkingTranslationIdle       time.Duration
	ThinkingTranslator            ThinkingTranslatorFunc
	Runner                        runner.Runner
}

type Manager struct {
	store *store.Store
	hub   *ws.Hub

	runtimeRunner   runner.Runner
	openaiClient    openai.Client
	anthropicClient anthropic.Client

	softRatio     float64
	forceRatio    float64
	hardRatio     float64
	keepRecent    int
	contextWindow int64

	thinkingTranslationMinChars   int
	thinkingTranslationForceChars int
	thinkingTranslationIdle       time.Duration
	thinkingTranslator            ThinkingTranslatorFunc
	codexRuntimePool              *codexRuntimePool
}

type TurnParams struct {
	Session             store.ChatSession
	ExpertID            string
	CLIToolID           *string
	ModelID             *string
	ReasoningEffort     *string
	UserInput           string
	ModelInput          string
	Attachments         []UploadedAttachment
	Spec                runner.RunSpec
	Provider            string
	Model               string
	SDK                 runner.SDKSpec
	Env                 map[string]string
	Fallbacks           []runner.SDKFallback
	ThinkingTranslation *ThinkingTranslationSpec
}

type TurnResult struct {
	UserMessage                store.ChatMessage `json:"user_message"`
	AssistantMessage           store.ChatMessage `json:"assistant_message"`
	ReasoningText              *string           `json:"reasoning_text,omitempty"`
	TranslatedReasoningText    *string           `json:"translated_reasoning_text,omitempty"`
	ModelInput                 *string           `json:"model_input,omitempty"`
	ContextMode                *string           `json:"context_mode,omitempty"`
	CachedInputTokens          *int64            `json:"cached_input_tokens,omitempty"`
	ThinkingTranslationApplied bool              `json:"thinking_translation_applied"`
	ThinkingTranslationFailed  bool              `json:"thinking_translation_failed"`
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
	thinkingTranslationMinChars := opts.ThinkingTranslationMinChars
	if thinkingTranslationMinChars <= 0 {
		thinkingTranslationMinChars = defaultThinkingTranslationMinChars
	}
	thinkingTranslationForceChars := opts.ThinkingTranslationForceChars
	if thinkingTranslationForceChars <= 0 {
		thinkingTranslationForceChars = defaultThinkingTranslationForceChars
	}
	if thinkingTranslationForceChars < thinkingTranslationMinChars {
		thinkingTranslationForceChars = thinkingTranslationMinChars
	}
	thinkingTranslationIdle := opts.ThinkingTranslationIdle
	if thinkingTranslationIdle <= 0 {
		thinkingTranslationIdle = defaultThinkingTranslationIdle
	}

	return &Manager{
		store:                         st,
		hub:                           hub,
		runtimeRunner:                 opts.Runner,
		openaiClient:                  openai.NewClient(),
		anthropicClient:               anthropic.NewClient(),
		softRatio:                     soft,
		forceRatio:                    force,
		hardRatio:                     hard,
		keepRecent:                    keepRecent,
		contextWindow:                 contextWindow,
		thinkingTranslationMinChars:   thinkingTranslationMinChars,
		thinkingTranslationForceChars: thinkingTranslationForceChars,
		thinkingTranslationIdle:       thinkingTranslationIdle,
		thinkingTranslator:            opts.ThinkingTranslator,
		codexRuntimePool:              newCodexRuntimePool(defaultCodexRuntimeIdleTTL, defaultCodexRuntimeReapTick),
	}
}

// Close 功能：释放 chat manager 持有的暖运行时资源。
// 参数/返回：无入参；成功返回 nil。
// 失败场景：底层 Codex app-server 关闭失败时返回 error。
// 副作用：关闭所有仍在内存池中的 Codex app-server 子进程。
func (m *Manager) Close() error {
	if m == nil || m.codexRuntimePool == nil {
		return nil
	}
	return m.codexRuntimePool.Close()
}

// ReleaseSessionRuntime 功能：主动释放指定 chat session 的暖运行时。
// 参数/返回：sessionID 为 chat session id；成功返回 nil。
// 失败场景：底层 Codex app-server 关闭失败时返回 error。
// 副作用：从内存池中移除并关闭该 session 绑定的 Codex app-server 子进程。
func (m *Manager) ReleaseSessionRuntime(sessionID string) error {
	if m == nil || m.codexRuntimePool == nil {
		return nil
	}
	return m.codexRuntimePool.Invalidate(sessionID)
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

	spec := params.Spec
	if spec.SDK == nil && strings.TrimSpace(params.SDK.Provider) != "" {
		sdkCopy := params.SDK
		spec = runner.RunSpec{
			Command:      "sdk:" + strings.TrimSpace(sdkCopy.Provider),
			Args:         []string{"model=" + strings.TrimSpace(sdkCopy.Model)},
			Env:          cloneEnvMap(params.Env),
			Cwd:          params.Session.WorkspacePath,
			SDK:          &sdkCopy,
			SDKFallbacks: append([]runner.SDKFallback(nil), params.Fallbacks...),
		}
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" && spec.SDK != nil {
		provider = strings.ToLower(strings.TrimSpace(spec.SDK.Provider))
	}
	model := strings.TrimSpace(params.Model)
	if model == "" && spec.SDK != nil {
		model = strings.TrimSpace(spec.SDK.Model)
	}
	reasoningEffort := resolvedTurnOptionPointer(params.ReasoningEffort, "", params.Session.ReasoningEffort)

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
	turnTimeline, err := m.startTurnTimeline(ctx, params.Session, userMsg, expertID, provider, model, modelInput)
	if err != nil {
		return TurnResult{}, err
	}
	turnCompleted := false
	defer func() {
		if turnCompleted || err == nil {
			return
		}
		_ = m.failTurnTimeline(ctxNoCancel(ctx), turnTimeline, err)
	}()
	m.broadcast("chat.turn.started", map[string]any{
		"session_id":      params.Session.ID,
		"user_message_id": userMsg.ID,
		"turn":            userMsg.Turn,
		"expert_id":       expertID,
		"provider":        provider,
		"model":           model,
	})

	if spec.SDK == nil {
		result, runErr := m.runCLITurn(ctx, params.Session, turnTimeline, userMsg, modelInput, spec, expertID, provider, model, params.CLIToolID, params.ModelID, reasoningEffort, params.ThinkingTranslation)
		if runErr == nil {
			turnCompleted = true
		}
		err = runErr
		return result, err
	}
	if err := m.ensureCompaction(ctx, params.Session, modelInput, *spec.SDK, spec.Env); err != nil {
		return TurnResult{}, err
	}

	var anchor store.ChatAnchor
	if strings.EqualFold(provider, params.Session.Provider) && strings.EqualFold(model, params.Session.Model) {
		anchor, _ = m.store.GetChatAnchor(ctx, params.Session.ID)
	}
	translationRuntime := newThinkingTranslationRuntime(m, params.Session.ID, turnTimeline.ID, params.ThinkingTranslation)

	usedSDK, respText, reasoningText, anchorUpdate, callMeta, err := m.callProviderWithFallbacks(ctx, params.Session, userMsg, *spec.SDK, spec.Env, modelInput, anchor, spec.SDKFallbacks, translationRuntime)
	if err != nil {
		return TurnResult{}, err
	}
	provider = strings.ToLower(strings.TrimSpace(usedSDK.Provider))
	model = strings.TrimSpace(usedSDK.Model)
	translatedReasoningText := ""
	translationApplied := translationRuntime.applied()
	translationFailed := translationRuntime.failedState()
	if translationRuntime != nil {
		translatedReasoningText = translationRuntime.translatedText()
	}

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
		CLIToolID: params.CLIToolID,
		ModelID:   params.ModelID,
		Provider:  provider,
		Model:     model,
	})
	if err := m.completeTurnEntry(ctx, turnTimeline.ID, "answer", "answer", respText, nil); err != nil {
		return TurnResult{}, err
	}
	if strings.TrimSpace(reasoningText) != "" {
		if err := m.completeTurnEntry(ctx, turnTimeline.ID, "thinking:1", "thinking", reasoningText, nil); err != nil {
			return TurnResult{}, err
		}
		if strings.TrimSpace(translatedReasoningText) != "" || translationFailed {
			if err := m.replaceTurnTranslation(ctx, turnTimeline.ID, "thinking:1", translatedReasoningText, translationFailed); err != nil {
				return TurnResult{}, err
			}
		}
	}
	if err := m.completeTurnTimeline(ctx, turnTimeline, assistantMsg, callMeta.ContextMode, callMeta, translationApplied, translationFailed); err != nil {
		return TurnResult{}, err
	}
	turnCompleted = true

	m.broadcast("chat.turn.completed", map[string]any{
		"session_id":                   params.Session.ID,
		"user_message_id":              userMsg.ID,
		"message":                      assistantMsg,
		"reasoning_text":               reasoningText,
		"translated_reasoning_text":    translatedReasoningText,
		"thinking_translation_applied": translationApplied,
		"thinking_translation_failed":  translationFailed,
		"model_input":                  callMeta.ModelInput,
		"context_mode":                 callMeta.ContextMode,
		"token_in":                     callMeta.TokenIn,
		"token_out":                    callMeta.TokenOut,
		"cached_input_tokens":          callMeta.CachedInputTokens,
	})

	return TurnResult{
		UserMessage:                userMsg,
		AssistantMessage:           assistantMsg,
		ReasoningText:              pointerString(reasoningText),
		TranslatedReasoningText:    pointerString(translatedReasoningText),
		ModelInput:                 pointerString(callMeta.ModelInput),
		ContextMode:                pointerString(callMeta.ContextMode),
		CachedInputTokens:          callMeta.CachedInputTokens,
		ThinkingTranslationApplied: translationApplied,
		ThinkingTranslationFailed:  translationFailed,
	}, nil
}

func (m *Manager) callProviderWithFallbacks(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, fallbacks []runner.SDKFallback, translationRuntime *thinkingTranslationRuntime) (runner.SDKSpec, string, string, store.ChatAnchor, providerCallMeta, error) {
	respText, reasoningText, anchorUpdate, callMeta, err := m.callProviderWithAnchorRetry(ctx, sess, currentUser, sdk, env, modelInput, anchor, translationRuntime)
	if err == nil {
		return sdk, respText, reasoningText, anchorUpdate, callMeta, nil
	}
	lastErr := err
	for _, fallback := range fallbacks {
		respText, reasoningText, anchorUpdate, callMeta, err = m.callProviderWithAnchorRetry(ctx, sess, currentUser, fallback.SDK, fallback.Env, modelInput, store.ChatAnchor{}, translationRuntime)
		if err == nil {
			return fallback.SDK, respText, reasoningText, anchorUpdate, callMeta, nil
		}
		lastErr = err
	}
	return runner.SDKSpec{}, "", "", store.ChatAnchor{}, providerCallMeta{}, lastErr
}

func (m *Manager) callProviderWithAnchorRetry(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, translationRuntime *thinkingTranslationRuntime) (string, string, store.ChatAnchor, providerCallMeta, error) {
	provider := strings.ToLower(strings.TrimSpace(sdk.Provider))
	respText, reasoningText, anchorUpdate, callMeta, err := m.callProvider(ctx, sess, currentUser, sdk, env, modelInput, anchor, translationRuntime)
	if err == nil {
		return respText, reasoningText, anchorUpdate, callMeta, nil
	}
	if (provider == "openai" && anchor.PreviousResponse != nil && strings.TrimSpace(*anchor.PreviousResponse) != "") ||
		(provider == "anthropic" && anchor.ContainerID != nil && strings.TrimSpace(*anchor.ContainerID) != "") {
		return m.callProvider(ctx, sess, currentUser, sdk, env, modelInput, store.ChatAnchor{}, translationRuntime)
	}
	return "", "", store.ChatAnchor{}, providerCallMeta{}, err
}

func (m *Manager) runLegacyCLITurn(ctx context.Context, sess store.ChatSession, turn store.ChatTurn, userMsg store.ChatMessage, modelInput string, spec runner.RunSpec, expertID, provider, model string, cliToolID, modelID, reasoningEffort *string, thinkingTranslation *ThinkingTranslationSpec) (TurnResult, error) {
	if m.runtimeRunner == nil {
		return TurnResult{}, fmt.Errorf("chat runtime runner not configured")
	}
	if strings.TrimSpace(spec.Command) == "" {
		return TurnResult{}, fmt.Errorf("%w: chat expert is not executable", store.ErrValidation)
	}

	artifactDir, err := cliruntime.ChatTurnArtifactDir(sess.ID, userMsg.ID)
	if err != nil {
		artifactDir = ""
	}
	attemptResume := strings.TrimSpace(pointerStringValue(sess.CLISessionID))
	translatedReasoningText := ""
	thinkingTranslationApplied := false
	thinkingTranslationFailed := false

	runOnce := func(prompt string, contextMode string, resumeSessionID string) (finalText string, reasoningText string, nextSessionID string, err error) {
		runSpec := spec
		if runSpec.Env == nil {
			runSpec.Env = map[string]string{}
		}
		runSpec.Env["VIBE_TREE_PROMPT"] = prompt
		if resumeSessionID != "" {
			runSpec.Env["VIBE_TREE_RESUME_SESSION_ID"] = resumeSessionID
		} else {
			delete(runSpec.Env, "VIBE_TREE_RESUME_SESSION_ID")
		}
		preparedRunSpec, prepErr := prepareIFLOWRunSpec(sess, runSpec, expertID)
		if prepErr != nil {
			return "", "", "", prepErr
		}
		runSpec = preparedRunSpec
		if artifactDir != "" {
			runSpec = cliruntime.PrepareRunSpec(runSpec, artifactDir)
		}
		if strings.TrimSpace(runSpec.Cwd) == "" {
			runSpec.Cwd = sess.WorkspacePath
		}
		handle, err := m.runtimeRunner.StartOneshot(ctx, runSpec)
		if err != nil {
			return "", "", "", err
		}
		output := handle.Output()
		toolID := strings.TrimSpace(runSpec.Env["VIBE_TREE_CLI_FAMILY"])
		parser := newCLIStreamParser(toolID)
		assistantBuf := strings.Builder{}
		thinkingBuf := strings.Builder{}
		opencodeBlankSteps := 0
		opencodeLoopGuardText := ""
		sawOpenCodeContent := false
		translationRuntime := newThinkingTranslationRuntime(m, sess.ID, turn.ID, thinkingTranslation)
		feedEmitter := newCodexTurnFeedEmitter(m, turn.ID, sess.ID, userMsg.ID, translationRuntime)
		done := make(chan struct{})
		go func() {
			defer close(done)
			if output == nil {
				return
			}
			defer output.Close()
			scanner := bufio.NewScanner(output)
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 4*1024*1024)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				events := parser.Parse(line)
				for _, event := range events {
					switch event.Type {
					case "session":
						if strings.TrimSpace(event.SessionID) != "" {
							nextSessionID = strings.TrimSpace(event.SessionID)
						}
					case "assistant_delta":
						if event.Delta == "" {
							continue
						}
						sawOpenCodeContent = true
						opencodeBlankSteps = 0
						assistantBuf.WriteString(event.Delta)
						feedEmitter.emit(ctx, chatTurnEventPayload{EntryID: "answer", Kind: "answer", Op: "append", Status: "streaming", Delta: event.Delta})
						m.broadcast("chat.turn.delta", map[string]any{"session_id": sess.ID, "delta": event.Delta})
					case "thinking_delta":
						if event.Delta == "" {
							continue
						}
						sawOpenCodeContent = true
						opencodeBlankSteps = 0
						thinkingBuf.WriteString(event.Delta)
						feedEmitter.emit(ctx, chatTurnEventPayload{EntryID: "thinking:1", Kind: "thinking", Op: "append", Status: "streaming", Delta: event.Delta})
						m.broadcast("chat.turn.thinking.delta", map[string]any{"session_id": sess.ID, "delta": event.Delta})
						if translationRuntime != nil {
							translationRuntime.add(ctx, "thinking:1", event.Delta)
						}
					case "progress_delta":
						if event.Delta == "" {
							continue
						}
						if toolID == "opencode" && !sawOpenCodeContent {
							delta := strings.TrimSpace(event.Delta)
							if strings.HasPrefix(delta, "[step]") {
								opencodeBlankSteps += 1
							} else if delta != "" {
								opencodeBlankSteps = 0
							}
							if opencodeBlankSteps >= defaultOpenCodeBlankStepLimit && opencodeLoopGuardText == "" {
								opencodeLoopGuardText = "OpenCode 在当前模型上持续执行空步骤且未产生文本输出，已提前终止。请改用 gpt-5.4 或其他已验证支持 agentic tool-calling 的模型。"
								_ = handle.Cancel(1500 * time.Millisecond)
							}
						}
						feedEmitter.emit(ctx, chatTurnEventPayload{EntryID: "progress:legacy", Kind: "progress", Op: "upsert", Status: "streaming", Content: event.Delta})
						m.broadcast("chat.turn.thinking.delta", map[string]any{"session_id": sess.ID, "delta": event.Delta})
					case "final":
						if strings.TrimSpace(event.Text) != "" {
							finalText = strings.TrimSpace(event.Text)
						}
						if strings.TrimSpace(event.Thinking) != "" && thinkingBuf.Len() == 0 {
							thinkingBuf.WriteString(strings.TrimSpace(event.Thinking))
						}
					}
				}
			}
		}()
		exitRes, waitErr := handle.Wait()
		<-done
		if translationRuntime != nil {
			translationRuntime.complete(ctx)
			translatedReasoningText = translationRuntime.translatedText()
			thinkingTranslationApplied = translationRuntime.applied()
			thinkingTranslationFailed = translationRuntime.failedState()
		} else {
			translatedReasoningText = ""
			thinkingTranslationApplied = false
			thinkingTranslationFailed = false
		}
		if strings.TrimSpace(finalText) == "" && assistantBuf.Len() > 0 {
			finalText = strings.TrimSpace(assistantBuf.String())
		}
		if strings.TrimSpace(finalText) == "" && strings.TrimSpace(opencodeLoopGuardText) != "" {
			finalText = strings.TrimSpace(opencodeLoopGuardText)
		}
		reasoningText = strings.TrimSpace(thinkingBuf.String())
		if artifactDir != "" {
			if artifactText, err := cliruntime.ReadFinalMessage(artifactDir); err == nil && strings.TrimSpace(artifactText) != "" {
				finalText = strings.TrimSpace(artifactText)
			}
			if cliSession, err := cliruntime.ReadSession(artifactDir); err == nil && strings.TrimSpace(cliSession.SessionID) != "" {
				nextSessionID = strings.TrimSpace(cliSession.SessionID)
			}
		}
		if strings.TrimSpace(finalText) == "" {
			if summary := cliruntime.SummaryText(artifactDir); summary != nil {
				finalText = strings.TrimSpace(*summary)
			}
		}
		if waitErr != nil || exitRes.ExitCode != 0 {
			if strings.TrimSpace(resumeSessionID) != "" {
				if waitErr != nil {
					return "", reasoningText, nextSessionID, waitErr
				}
				return "", reasoningText, nextSessionID, fmt.Errorf("cli runtime exited with code %d", exitRes.ExitCode)
			}
			if strings.TrimSpace(finalText) == "" {
				if waitErr != nil {
					return "", reasoningText, nextSessionID, waitErr
				}
				return "", reasoningText, nextSessionID, fmt.Errorf("cli runtime exited with code %d", exitRes.ExitCode)
			}
		}
		if strings.TrimSpace(finalText) == "" {
			finalText = "(empty response)"
		}
		return finalText, reasoningText, firstNonEmptyTrimmed(nextSessionID, resumeSessionID), nil
	}

	var (
		finalText     string
		reasoningText string
		cliSessionID  string
		contextMode   string
	)
	if attemptResume != "" {
		prompt := buildCLIIncrementalPrompt(userMsg, modelInput)
		finalText, reasoningText, cliSessionID, err = runOnce(prompt, "cli_resume", attemptResume)
		if err == nil {
			contextMode = "cli_resume"
		}
	}
	if err != nil || contextMode == "" {
		if err := m.ensureCompaction(ctx, sess, modelInput, runner.SDKSpec{}, nil); err != nil {
			return TurnResult{}, err
		}
		messages, msgErr := m.store.ListChatMessages(ctx, sess.ID, 1000)
		if msgErr != nil {
			return TurnResult{}, msgErr
		}
		prompt, _ := buildCLITurnPrompt(sess, messages, userMsg, modelInput, m.keepRecent)
		finalText, reasoningText, cliSessionID, err = runOnce(prompt, "cli_reconstructed", "")
		if err != nil {
			return TurnResult{}, err
		}
		contextMode = "cli_reconstructed"
	}

	assistantMsg, err := m.store.AppendChatMessage(ctx, store.AppendChatMessageParams{
		SessionID:   sess.ID,
		Role:        "assistant",
		ContentText: finalText,
		ExpertID:    pointerString(expertID),
		Provider:    pointerString(provider),
		Model:       pointerString(model),
	})
	if err != nil {
		return TurnResult{}, err
	}
	_, _ = m.store.UpdateChatSessionDefaults(ctx, store.UpdateChatSessionDefaultsParams{
		SessionID:       sess.ID,
		ExpertID:        expertID,
		CLIToolID:       resolvedTurnOptionPointer(cliToolID, spec.Env["VIBE_TREE_CLI_TOOL_ID"], sess.CLIToolID),
		ModelID:         resolvedTurnOptionPointer(modelID, spec.Env["VIBE_TREE_MODEL_ID"], sess.ModelID),
		ReasoningEffort: reasoningEffort,
		CLISessionID:    pointerOrNilString(cliSessionID),
		Provider:        provider,
		Model:           model,
	})
	if err := m.completeTurnEntry(ctx, turn.ID, "answer", "answer", finalText, nil); err != nil {
		return TurnResult{}, err
	}
	if strings.TrimSpace(reasoningText) != "" {
		if err := m.completeTurnEntry(ctx, turn.ID, "thinking:1", "thinking", reasoningText, nil); err != nil {
			return TurnResult{}, err
		}
		if strings.TrimSpace(translatedReasoningText) != "" || thinkingTranslationFailed {
			if err := m.replaceTurnTranslation(ctx, turn.ID, "thinking:1", translatedReasoningText, thinkingTranslationFailed); err != nil {
				return TurnResult{}, err
			}
		}
	}
	if err := m.completeTurnTimeline(ctx, turn, assistantMsg, contextMode, providerCallMeta{ModelInput: modelInput, ContextMode: contextMode}, thinkingTranslationApplied, thinkingTranslationFailed); err != nil {
		return TurnResult{}, err
	}
	m.broadcast("chat.turn.completed", map[string]any{
		"session_id":                   sess.ID,
		"user_message_id":              userMsg.ID,
		"message":                      assistantMsg,
		"reasoning_text":               reasoningText,
		"translated_reasoning_text":    translatedReasoningText,
		"thinking_translation_applied": thinkingTranslationApplied,
		"thinking_translation_failed":  thinkingTranslationFailed,
		"model_input":                  modelInput,
		"context_mode":                 contextMode,
		"token_in":                     nil,
		"token_out":                    nil,
		"cached_input_tokens":          nil,
	})
	return TurnResult{UserMessage: userMsg, AssistantMessage: assistantMsg, ReasoningText: pointerOrNilString(reasoningText), TranslatedReasoningText: pointerOrNilString(translatedReasoningText), ModelInput: pointerString(modelInput), ContextMode: pointerString(contextMode), CachedInputTokens: nil, ThinkingTranslationApplied: thinkingTranslationApplied, ThinkingTranslationFailed: thinkingTranslationFailed}, nil
}

type cliStreamEvent struct {
	Type      string
	Delta     string
	SessionID string
	Text      string
	Thinking  string
}

type cliStreamParser struct {
	toolID            string
	opencodePartKinds map[string]string
	opencodePartTexts map[string]string
}

func newCLIStreamParser(toolID string) *cliStreamParser {
	return &cliStreamParser{
		toolID:            strings.TrimSpace(toolID),
		opencodePartKinds: map[string]string{},
		opencodePartTexts: map[string]string{},
	}
}

func (p *cliStreamParser) Parse(line string) []cliStreamEvent {
	if p == nil {
		return nil
	}
	switch strings.TrimSpace(p.toolID) {
	case "claude":
		return parseClaudeCLIStreamEvents(line)
	case "iflow":
		return parseIFLOWCLIStreamEvents(line)
	case "opencode":
		return p.parseOpenCodeCLIStreamEvents(line)
	default:
		return parseCodexCLIStreamEvents(line)
	}
}

func parseCLIStreamEvents(toolID, line string) []cliStreamEvent {
	return newCLIStreamParser(toolID).Parse(line)
}

func parseIFLOWCLIStreamEvents(line string) []cliStreamEvent {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	return []cliStreamEvent{{Type: "assistant_delta", Delta: line + "\n"}}
}

func parseCodexCLIStreamEvents(line string) []cliStreamEvent {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil
	}
	out := make([]cliStreamEvent, 0, 2)
	if sid := firstNonEmptyTrimmed(stringValue(raw["session_id"]), stringValue(raw["thread_id"])); sid != "" {
		out = append(out, cliStreamEvent{Type: "session", SessionID: sid})
	}
	typeName := strings.TrimSpace(stringValue(raw["type"]))
	if typeName == "thread.started" || typeName == "sessionConfigured" {
		if sid := firstNonEmptyTrimmed(stringValue(raw["thread_id"]), nestedString(raw, "thread", "id"), nestedString(raw, "params", "sessionId")); sid != "" {
			out = append(out, cliStreamEvent{Type: "session", SessionID: sid})
		}
	}
	if typeName == "item.completed" {
		item, _ := raw["item"].(map[string]any)
		itemType := strings.TrimSpace(stringValue(item["type"]))
		textVal := stringifyEventText(item["text"])
		switch itemType {
		case "agent_message":
			if textVal != "" {
				out = append(out, cliStreamEvent{Type: "assistant_delta", Delta: textVal})
			}
		case "reasoning":
			if textVal != "" {
				out = append(out, cliStreamEvent{Type: "thinking_delta", Delta: textVal})
			}
		case "command_execution":
			msg := firstNonEmptyTrimmed(stringValue(item["command"]), textVal, "[tool] command execution")
			out = append(out, cliStreamEvent{Type: "progress_delta", Delta: msg})
		}
	}
	return out
}

func (p *cliStreamParser) parseOpenCodeCLIStreamEvents(line string) []cliStreamEvent {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil
	}
	out := make([]cliStreamEvent, 0, 3)
	if sid := firstNonEmptyTrimmed(
		stringValue(raw["sessionID"]),
		stringValue(raw["session_id"]),
		nestedString(raw, "properties", "sessionID"),
		nestedString(raw, "properties", "part", "sessionID"),
		nestedString(raw, "properties", "info", "id"),
		nestedString(raw, "part", "sessionID"),
	); sid != "" {
		out = append(out, cliStreamEvent{Type: "session", SessionID: sid})
	}
	typeName := strings.TrimSpace(stringValue(raw["type"]))
	switch typeName {
	case "message.part.updated":
		properties, _ := raw["properties"].(map[string]any)
		part, _ := properties["part"].(map[string]any)
		out = append(out, p.openCodePartEvents(part)...)
	case "message.part.delta":
		properties, _ := raw["properties"].(map[string]any)
		partID := strings.TrimSpace(stringValue(properties["partID"]))
		field := strings.TrimSpace(stringValue(properties["field"]))
		delta := stringValue(properties["delta"])
		if delta == "" {
			return out
		}
		if field != "" && field != "text" && !strings.HasSuffix(field, ".text") {
			return out
		}
		if partID != "" {
			p.opencodePartTexts[partID] = p.opencodePartTexts[partID] + delta
		}
		switch strings.TrimSpace(p.opencodePartKinds[partID]) {
		case "reasoning":
			out = append(out, cliStreamEvent{Type: "thinking_delta", Delta: delta})
		case "tool", "step-start", "step-finish", "patch", "agent", "subtask", "retry", "compaction":
			out = append(out, cliStreamEvent{Type: "progress_delta", Delta: delta})
		default:
			out = append(out, cliStreamEvent{Type: "assistant_delta", Delta: delta})
		}
	case "text", "reasoning", "tool", "patch", "agent", "subtask", "retry", "compaction", "step_start", "step_finish":
		part, _ := raw["part"].(map[string]any)
		out = append(out, p.openCodePartEvents(part)...)
	case "session.status":
		statusType := strings.TrimSpace(nestedString(raw, "properties", "status", "type"))
		message := strings.TrimSpace(nestedString(raw, "properties", "status", "message"))
		if statusType != "" {
			msg := "[session] " + statusType
			if message != "" {
				msg += ": " + message
			}
			out = append(out, cliStreamEvent{Type: "progress_delta", Delta: msg})
		}
	case "session.compacted":
		out = append(out, cliStreamEvent{Type: "progress_delta", Delta: "[session] compacted"})
	case "permission.asked":
		permission := firstNonEmptyTrimmed(nestedString(raw, "properties", "permission"), "approval requested")
		out = append(out, cliStreamEvent{Type: "progress_delta", Delta: "[permission] " + permission})
	case "question.asked":
		out = append(out, cliStreamEvent{Type: "progress_delta", Delta: "[question] input requested"})
	case "todo.updated":
		out = append(out, cliStreamEvent{Type: "progress_delta", Delta: "[todo] updated"})
	case "error", "session.error":
		message := firstNonEmptyTrimmed(
			nestedString(raw, "error", "data", "message"),
			nestedString(raw, "error", "message"),
			nestedString(raw, "properties", "error", "data", "message"),
			nestedString(raw, "properties", "error", "message"),
		)
		if message != "" {
			out = append(out, cliStreamEvent{Type: "final", Text: message})
		}
	}
	return out
}

func (p *cliStreamParser) openCodePartEvents(part map[string]any) []cliStreamEvent {
	if len(part) == 0 {
		return nil
	}
	partID := strings.TrimSpace(stringValue(part["id"]))
	partType := strings.TrimSpace(stringValue(part["type"]))
	if partID != "" && partType != "" {
		p.opencodePartKinds[partID] = partType
	}
	switch partType {
	case "text", "reasoning":
		text := stringValue(part["text"])
		if text == "" {
			return nil
		}
		previous := ""
		if partID != "" {
			previous = p.opencodePartTexts[partID]
			p.opencodePartTexts[partID] = text
		}
		delta := text
		if previous != "" {
			if strings.HasPrefix(text, previous) {
				delta = text[len(previous):]
			} else if text == previous {
				delta = ""
			}
		}
		if delta == "" {
			return nil
		}
		if partType == "reasoning" {
			return []cliStreamEvent{{Type: "thinking_delta", Delta: delta}}
		}
		return []cliStreamEvent{{Type: "assistant_delta", Delta: delta}}
	default:
		if progress := openCodeProgressFromPart(part); progress != "" {
			return []cliStreamEvent{{Type: "progress_delta", Delta: progress}}
		}
	}
	return nil
}

func openCodeProgressFromPart(part map[string]any) string {
	partType := strings.TrimSpace(stringValue(part["type"]))
	switch partType {
	case "tool":
		state, _ := part["state"].(map[string]any)
		status := strings.TrimSpace(stringValue(state["status"]))
		title := firstNonEmptyTrimmed(stringValue(state["title"]), stringValue(part["tool"]), "tool")
		if status != "" {
			return fmt.Sprintf("[tool] %s (%s)", title, status)
		}
		return "[tool] " + title
	case "step-start":
		return "[step] started"
	case "step-finish":
		reason := firstNonEmptyTrimmed(stringValue(part["reason"]), "finished")
		return "[step] " + reason
	case "patch":
		if files, ok := part["files"].([]any); ok && len(files) > 0 {
			return fmt.Sprintf("[patch] %d files", len(files))
		}
		return "[patch] updated files"
	case "agent":
		return "[agent] " + firstNonEmptyTrimmed(stringValue(part["name"]), "agent")
	case "subtask":
		return "[subtask] " + firstNonEmptyTrimmed(stringValue(part["description"]), stringValue(part["prompt"]), "subtask")
	case "retry":
		return "[retry] " + firstNonEmptyTrimmed(nestedString(part, "error", "data", "message"), nestedString(part, "error", "message"), "retrying")
	case "compaction":
		return "[session] compacted context"
	}
	return ""
}

func parseClaudeCLIStreamEvents(line string) []cliStreamEvent {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil
	}
	out := make([]cliStreamEvent, 0, 3)
	if sid := firstNonEmptyTrimmed(stringValue(raw["session_id"]), stringValue(raw["sessionId"])); sid != "" {
		out = append(out, cliStreamEvent{Type: "session", SessionID: sid})
	}
	typeName := strings.TrimSpace(stringValue(raw["type"]))
	switch typeName {
	case "stream_event":
		event, _ := raw["event"].(map[string]any)
		if strings.TrimSpace(stringValue(event["type"])) == "content_block_delta" {
			delta, _ := event["delta"].(map[string]any)
			deltaType := strings.TrimSpace(stringValue(delta["type"]))
			if deltaType == "text_delta" {
				if text := strings.TrimSpace(stringValue(delta["text"])); text != "" {
					out = append(out, cliStreamEvent{Type: "assistant_delta", Delta: text})
				}
			}
			if deltaType == "thinking_delta" {
				if thinking := strings.TrimSpace(stringValue(delta["thinking"])); thinking != "" {
					out = append(out, cliStreamEvent{Type: "thinking_delta", Delta: thinking})
				}
			}
		}
	case "tool_use":
		name := firstNonEmptyTrimmed(stringValue(raw["tool_name"]), stringValue(raw["name"]), "tool")
		out = append(out, cliStreamEvent{Type: "progress_delta", Delta: "[tool] " + name})
	case "tool_result":
		out = append(out, cliStreamEvent{Type: "progress_delta", Delta: "[tool] result"})
	case "result":
		if result := strings.TrimSpace(stringValue(raw["result"])); result != "" {
			out = append(out, cliStreamEvent{Type: "final", Text: result})
		}
	}
	return out
}

func nestedString(root map[string]any, path ...string) string {
	cur := any(root)
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[key]
	}
	return stringValue(cur)
}

func stringValue(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case float64:
		return fmt.Sprintf("%v", vv)
	default:
		return ""
	}
}

func stringifyEventText(v any) string {
	switch vv := v.(type) {
	case string:
		return strings.TrimSpace(vv)
	case []any:
		parts := make([]string, 0, len(vv))
		for _, item := range vv {
			parts = append(parts, stringifyEventText(item))
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		if text := firstNonEmptyTrimmed(stringValue(vv["text"]), stringValue(vv["content"])); text != "" {
			return text
		}
		b, _ := json.Marshal(vv)
		return strings.TrimSpace(string(b))
	default:
		return ""
	}
}

func buildCLITurnPrompt(sess store.ChatSession, messages []store.ChatMessage, currentUser store.ChatMessage, modelInput string, keepRecent int) (string, string) {
	messages = applyCurrentModelInput(messages, currentUser, modelInput)
	prompt := renderConversation(sess.Summary, messages, "", keepRecent)
	debug := renderConversationDebug(sess.Summary, messages, keepRecent)
	if current := buildCurrentInputDebug(modelInput, currentUser.Attachments); strings.TrimSpace(current) != "" {
		if strings.TrimSpace(prompt) != "" {
			prompt = strings.TrimSpace(prompt) + "\n\n" + current
		} else {
			prompt = current
		}
		if strings.TrimSpace(debug) != "" {
			debug = strings.TrimSpace(debug) + "\n\n" + current
		} else {
			debug = current
		}
	}
	if lines := cliAttachmentPathLines(currentUser.Attachments); len(lines) > 0 {
		block := "Attachment file paths:\n" + strings.Join(lines, "\n")
		if strings.TrimSpace(prompt) != "" {
			prompt = strings.TrimSpace(prompt) + "\n\n" + block
		} else {
			prompt = block
		}
		if strings.TrimSpace(debug) != "" {
			debug = strings.TrimSpace(debug) + "\n\n" + block
		} else {
			debug = block
		}
	}
	return strings.TrimSpace(prompt), strings.TrimSpace(debug)
}

func cliAttachmentPathLines(attachments []store.ChatAttachment) []string {
	out := make([]string, 0, len(attachments))
	for _, att := range attachments {
		fullPath, err := attachmentDiskPath(att.StorageRelPath)
		if err != nil {
			continue
		}
		out = append(out, fmt.Sprintf("- %s (%s): %s", att.FileName, att.Kind, fullPath))
	}
	return out
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
		style, err := m.ensureOpenAIAPIStyle(ctx, sdk, env)
		if err != nil {
			return "", err
		}
		request := m.openAITextRequest(sdk, env, prompt)
		request.Instructions = system
		request.MaxOutputTokens = maxTokens
		request.Temperature = temp
		out, _, err := openaicompat.CompleteText(ctx, style, request)
		if err != nil && openaicompat.IsEndpointMismatch(err) && strings.TrimSpace(sdk.LLMModelID) != "" {
			detectReq := m.openAITextRequest(sdk, env, openaicompat.DetectionPrompt)
			style, _, retryErr := openaicompat.ReprobeSavedModelAPIStyle(ctx, sdk.LLMModelID, style, detectReq)
			if retryErr != nil {
				return "", retryErr
			}
			out, _, err = openaicompat.CompleteText(ctx, style, request)
		}
		return strings.TrimSpace(out), err
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

func (m *Manager) callProvider(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, translationRuntime *thinkingTranslationRuntime) (string, string, store.ChatAnchor, providerCallMeta, error) {
	provider := strings.ToLower(strings.TrimSpace(sdk.Provider))
	sdk.Provider = provider

	switch provider {
	case "openai":
		return m.callOpenAI(ctx, sess, currentUser, sdk, env, modelInput, anchor, translationRuntime)
	case "anthropic":
		return m.callAnthropic(ctx, sess, currentUser, sdk, env, modelInput, anchor, translationRuntime)
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

func (m *Manager) callOpenAI(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, translationRuntime *thinkingTranslationRuntime) (string, string, store.ChatAnchor, providerCallMeta, error) {
	style, err := m.ensureOpenAIAPIStyle(ctx, sdk, env)
	if err != nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, err
	}
	if style == openaicompat.APIStyleChatCompletions {
		return m.callOpenAIChatCompletions(ctx, sess, currentUser, sdk, env, modelInput)
	}
	out, reasoning, anchorOut, meta, err := m.callOpenAIResponses(ctx, sess, currentUser, sdk, env, modelInput, anchor, translationRuntime)
	if err != nil && openaicompat.IsEndpointMismatch(err) && strings.TrimSpace(sdk.LLMModelID) != "" {
		detectReq := m.openAITextRequest(sdk, env, openaicompat.DetectionPrompt)
		style, _, retryErr := openaicompat.ReprobeSavedModelAPIStyle(ctx, sdk.LLMModelID, openaicompat.APIStyleResponses, detectReq)
		if retryErr != nil {
			return "", "", store.ChatAnchor{}, providerCallMeta{}, retryErr
		}
		if style == openaicompat.APIStyleChatCompletions {
			return m.callOpenAIChatCompletions(ctx, sess, currentUser, sdk, env, modelInput)
		}
		return m.callOpenAIResponses(ctx, sess, currentUser, sdk, env, modelInput, anchor, translationRuntime)
	}
	return out, reasoning, anchorOut, meta, err
}

func (m *Manager) callOpenAIResponses(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, translationRuntime *thinkingTranslationRuntime) (string, string, store.ChatAnchor, providerCallMeta, error) {
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
			translationRuntime.add(ctx, "", delta)
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
			translationRuntime.add(ctx, "", done)
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
	translationRuntime.complete(ctx)
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

func (m *Manager) callOpenAIChatCompletions(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string) (string, string, store.ChatAnchor, providerCallMeta, error) {
	if len(currentUser.Attachments) > 0 {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, errors.New("openai chat completions fallback does not support attachments")
	}
	debugInput := strings.TrimSpace(modelInput)
	contextMode := "reconstructed"
	if msgs, err := m.store.ListChatMessages(ctx, sess.ID, 1000); err == nil {
		if anyMessageHasAttachments(msgs) {
			return "", "", store.ChatAnchor{}, providerCallMeta{}, errors.New("openai chat completions fallback does not support attachment sessions")
		}
		debugInput = renderConversation(sess.Summary, msgs, modelInput, m.keepRecent)
	}
	request := m.openAITextRequest(sdk, env, debugInput)
	var out strings.Builder
	usage, err := openaicompat.StreamText(ctx, openaicompat.APIStyleChatCompletions, request, func(delta string) {
		if delta == "" {
			return
		}
		out.WriteString(delta)
		m.broadcast("chat.turn.delta", map[string]any{
			"session_id": sess.ID,
			"delta":      delta,
		})
	})
	if err != nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, err
	}
	output := strings.TrimSpace(out.String())
	if output == "" {
		output = "(empty response)"
	}
	meta := providerCallMeta{ModelInput: debugInput, ContextMode: contextMode, TokenIn: usage.TokenIn, TokenOut: usage.TokenOut, CachedInputTokens: usage.CachedInputTokens}
	return output, "", store.ChatAnchor{}, meta, nil
}

func (m *Manager) ensureOpenAIAPIStyle(ctx context.Context, sdk runner.SDKSpec, env map[string]string) (openaicompat.APIStyle, error) {
	if strings.TrimSpace(sdk.LLMModelID) == "" {
		return openaicompat.APIStyleResponses, nil
	}
	detectReq := m.openAITextRequest(sdk, env, openaicompat.DetectionPrompt)
	style, _, err := openaicompat.EnsureSavedModelAPIStyle(ctx, sdk.LLMModelID, detectReq)
	return style, err
}

func (m *Manager) openAITextRequest(sdk runner.SDKSpec, env map[string]string, prompt string) openaicompat.TextRequest {
	baseURL := strings.TrimSpace(sdk.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(env["OPENAI_BASE_URL"])
	}
	return openaicompat.TextRequest{
		Model:           strings.TrimSpace(sdk.Model),
		BaseURL:         baseURL,
		APIKey:          strings.TrimSpace(env["OPENAI_API_KEY"]),
		OrganizationID:  strings.TrimSpace(env["OPENAI_ORG_ID"]),
		ProjectID:       strings.TrimSpace(env["OPENAI_PROJECT_ID"]),
		Prompt:          strings.TrimSpace(prompt),
		Instructions:    strings.TrimSpace(sdk.Instructions),
		MaxOutputTokens: sdk.MaxOutputTokens,
		Temperature:     sdk.Temperature,
	}
}

func (m *Manager) callAnthropic(ctx context.Context, sess store.ChatSession, currentUser store.ChatMessage, sdk runner.SDKSpec, env map[string]string, modelInput string, anchor store.ChatAnchor, translationRuntime *thinkingTranslationRuntime) (string, string, store.ChatAnchor, providerCallMeta, error) {
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
			translationRuntime.add(ctx, "", delta)
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
				translationRuntime.add(ctx, "", delta)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", "", store.ChatAnchor{}, providerCallMeta{}, err
	}
	translationRuntime.complete(ctx)
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

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func pointerStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func pointerOrNilString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func resolvedTurnOptionPointer(explicit *string, envValue string, fallback *string) *string {
	return pointerOrNilString(firstNonEmptyTrimmed(pointerStringValue(explicit), envValue, pointerStringValue(fallback)))
}

func buildCLIIncrementalPrompt(currentUser store.ChatMessage, modelInput string) string {
	parts := make([]string, 0, 2)
	if current := buildCurrentInputDebug(modelInput, currentUser.Attachments); strings.TrimSpace(current) != "" {
		parts = append(parts, current)
	}
	if lines := cliAttachmentPathLines(currentUser.Attachments); len(lines) > 0 {
		parts = append(parts, "Attachment file paths:\n"+strings.Join(lines, "\n"))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}
