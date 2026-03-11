package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vibe-tree/backend/internal/cliruntime"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

type codexAppServerClient interface {
	Initialize(ctx context.Context) error
	StartThread(ctx context.Context, req codexAppServerThreadRequest) (string, error)
	ResumeThread(ctx context.Context, req codexAppServerThreadRequest) (string, error)
	StartTurn(ctx context.Context, threadID string, prompt string, reasoningEffort *string) (string, error)
	Notifications() <-chan codexAppServerNotification
	Done() <-chan struct{}
	Wait() error
	Close() error
}

type codexAppServerThreadRequest struct {
	ThreadID         string
	Model            string
	Cwd              string
	BaseInstructions string
	Config           map[string]any
}

type codexAppServerNotification struct {
	Method string
	Params json.RawMessage
}

type codexAppServerTurnMetrics struct {
	TokenIn           *int64
	TokenOut          *int64
	CachedInputTokens *int64
}

type codexAppServerEarlyFailure struct {
	err error
}

func (e *codexAppServerEarlyFailure) Error() string {
	if e == nil || e.err == nil {
		return "codex app-server early failure"
	}
	return e.err.Error()
}

func (e *codexAppServerEarlyFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

var newCodexAppServerClient = func(ctx context.Context, spec runner.RunSpec) (codexAppServerClient, error) {
	return startCodexAppServerClient(ctx, spec)
}

var codexAppServerOptOutNotificationMethods = []string{
	"codex/event/agent_message_content_delta",
	"codex/event/agent_message_delta",
	"codex/event/agent_reasoning_delta",
	"codex/event/reasoning_content_delta",
	"codex/event/reasoning_raw_content_delta",
	"codex/event/plan_delta",
	"codex/event/item_started",
	"codex/event/item_completed",
}

func (m *Manager) runCLITurn(ctx context.Context, sess store.ChatSession, turn store.ChatTurn, userMsg store.ChatMessage, modelInput string, spec runner.RunSpec, expertID, provider, model string, cliToolID, modelID, reasoningEffort *string, thinkingTranslation *ThinkingTranslationSpec) (TurnResult, error) {
	if runner.NormalizeCLIFamily(spec.Env["VIBE_TREE_CLI_FAMILY"]) == "codex" {
		result, err := m.runCodexAppServerTurn(ctx, sess, turn, userMsg, modelInput, spec, expertID, provider, model, cliToolID, modelID, reasoningEffort, thinkingTranslation)
		if err == nil {
			return result, nil
		}
		var early *codexAppServerEarlyFailure
		if errors.As(err, &early) {
			return m.runLegacyCLITurn(ctx, sess, turn, userMsg, modelInput, spec, expertID, provider, model, cliToolID, modelID, reasoningEffort, thinkingTranslation)
		}
		return TurnResult{}, err
	}
	return m.runLegacyCLITurn(ctx, sess, turn, userMsg, modelInput, spec, expertID, provider, model, cliToolID, modelID, reasoningEffort, thinkingTranslation)
}

func (m *Manager) runCodexAppServerTurn(ctx context.Context, sess store.ChatSession, turn store.ChatTurn, userMsg store.ChatMessage, modelInput string, spec runner.RunSpec, expertID, provider, model string, cliToolID, modelID, reasoningEffort *string, thinkingTranslation *ThinkingTranslationSpec) (TurnResult, error) {
	artifactDir, err := cliruntime.ChatTurnArtifactDir(sess.ID, userMsg.ID)
	if err != nil {
		artifactDir = ""
	}
	attemptResume := strings.TrimSpace(pointerStringValue(sess.CLISessionID))

	runOnce := func(prompt string, contextMode string, resumeThreadID string) (TurnResult, error) {
		runSpec, prepErr := m.prepareCLIRuntimeRunSpec(sess, spec, expertID)
		if prepErr != nil {
			return TurnResult{}, prepErr
		}
		threadReq, err := m.buildCodexThreadRequest(sess, runSpec, expertID, cliToolID, resumeThreadID)
		if err != nil {
			return TurnResult{}, err
		}
		lease, err := m.codexRuntimePool.Acquire(ctx, sess.ID, runSpec, threadReq)
		if err != nil {
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}
		released := false
		defer func() {
			if !released {
				lease.Release()
			}
		}()

		client := lease.Client()
		if artifactDir != "" {
			if setter, ok := client.(interface{ SetDiagnosticsDir(string) }); ok {
				setter.SetDiagnosticsDir(artifactDir)
				defer setter.SetDiagnosticsDir("")
			}
		}
		if err := drainCodexAppServerNotifications(client); err != nil {
			released = true
			_ = lease.Discard()
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}
		threadID := lease.ThreadID()
		if strings.TrimSpace(threadID) == "" {
			if strings.TrimSpace(resumeThreadID) != "" {
				threadID, err = client.ResumeThread(ctx, threadReq)
			} else {
				threadID, err = client.StartThread(ctx, threadReq)
			}
			if err != nil {
				released = true
				_ = lease.Discard()
				return TurnResult{}, &codexAppServerEarlyFailure{err: err}
			}
			lease.SetThreadID(threadID)
		}

		activeTurnID, err := client.StartTurn(ctx, threadID, prompt, reasoningEffort)
		if err != nil {
			released = true
			_ = lease.Discard()
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}

		translationRuntime := newThinkingTranslationRuntime(m, sess.ID, turn.ID, thinkingTranslation)
		feedEmitter := newCodexTurnFeedEmitter(m, turn.ID, sess.ID, userMsg.ID, artifactDir, translationRuntime)
		defer feedEmitter.Close()
		var assistantBuf strings.Builder
		var reasoningSummaryBuf strings.Builder
		var reasoningContentBuf strings.Builder
		var progressBuf strings.Builder
		var finalText string
		var terminalErrorSummary string
		metrics := codexAppServerTurnMetrics{}
		sawReasoningSummary := false

		finalizeTurn := func(ctx context.Context, finalText string, incomplete bool) (TurnResult, error) {
			translatedReasoningText := ""
			thinkingTranslationApplied := false
			thinkingTranslationFailed := false
			if translationRuntime != nil {
				translationRuntime.complete(ctx)
				translatedReasoningText = translationRuntime.translatedText()
				thinkingTranslationApplied = translationRuntime.applied()
				thinkingTranslationFailed = translationRuntime.failedState()
			}

			reasoningText := strings.TrimSpace(reasoningSummaryBuf.String())
			if reasoningText == "" {
				reasoningText = strings.TrimSpace(reasoningContentBuf.String())
			}
			if reasoningText == "" {
				reasoningText = strings.TrimSpace(progressBuf.String())
			}
			if strings.TrimSpace(finalText) == "" && assistantBuf.Len() > 0 {
				finalText = strings.TrimSpace(assistantBuf.String())
			}
			finalText = strings.TrimSpace(finalText)
			if finalText == "" {
				if strings.TrimSpace(terminalErrorSummary) != "" {
					finalText = fmt.Sprintf("Codex 运行时错误：%s", strings.TrimSpace(terminalErrorSummary))
				} else {
					finalText = "(empty response)"
				}
			}
			if incomplete {
				finalText = strings.TrimSpace(finalText) + "\n\n(提示：本轮未正常收敛，结果可能不完整。)"
			}

			persistCtx := ctxNoCancel(ctx)
			if artifactDir != "" {
				_ = persistCodexChatArtifacts(artifactDir, threadID, model, sess.WorkspacePath, finalText)
			}

			assistantMsg, err := m.store.AppendChatMessage(persistCtx, store.AppendChatMessageParams{
				SessionID:   sess.ID,
				Role:        "assistant",
				ContentText: finalText,
				ExpertID:    pointerString(expertID),
				Provider:    pointerString(provider),
				Model:       pointerString(model),
				TokenIn:     metrics.TokenIn,
				TokenOut:    metrics.TokenOut,
			})
			if err != nil {
				return TurnResult{}, err
			}
			_, _ = m.store.UpdateChatSessionDefaults(persistCtx, store.UpdateChatSessionDefaultsParams{
				SessionID:       sess.ID,
				ExpertID:        expertID,
				CLIToolID:       resolvedTurnOptionPointer(cliToolID, spec.Env["VIBE_TREE_CLI_TOOL_ID"], sess.CLIToolID),
				ModelID:         resolvedTurnOptionPointer(modelID, spec.Env["VIBE_TREE_MODEL_ID"], sess.ModelID),
				ReasoningEffort: reasoningEffort,
				CLISessionID:    pointerOrNilString(threadID),
				Provider:        provider,
				Model:           model,
			})
			if err := m.completeTurnEntry(persistCtx, turn.ID, "answer", "answer", finalText, nil); err != nil {
				return TurnResult{}, err
			}
			if strings.TrimSpace(reasoningText) != "" && len(feedEmitter.thinkingStates) == 0 {
				if err := m.completeTurnEntry(persistCtx, turn.ID, "thinking:1", "thinking", reasoningText, nil); err != nil {
					return TurnResult{}, err
				}
				if strings.TrimSpace(translatedReasoningText) != "" || thinkingTranslationFailed {
					if err := m.replaceTurnTranslation(persistCtx, turn.ID, "thinking:1", translatedReasoningText, thinkingTranslationFailed); err != nil {
						return TurnResult{}, err
					}
				}
			}
			if err := m.completeTurnTimeline(persistCtx, turn, assistantMsg, contextMode, providerCallMeta{TokenIn: metrics.TokenIn, TokenOut: metrics.TokenOut, CachedInputTokens: metrics.CachedInputTokens, ModelInput: modelInput, ContextMode: contextMode}, thinkingTranslationApplied, thinkingTranslationFailed); err != nil {
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
				"token_in":                     metrics.TokenIn,
				"token_out":                    metrics.TokenOut,
				"cached_input_tokens":          metrics.CachedInputTokens,
			})
			return TurnResult{
				UserMessage:                userMsg,
				AssistantMessage:           assistantMsg,
				ReasoningText:              pointerOrNilString(reasoningText),
				TranslatedReasoningText:    pointerOrNilString(translatedReasoningText),
				ModelInput:                 pointerString(modelInput),
				ContextMode:                pointerString(contextMode),
				CachedInputTokens:          metrics.CachedInputTokens,
				ThinkingTranslationApplied: thinkingTranslationApplied,
				ThinkingTranslationFailed:  thinkingTranslationFailed,
			}, nil
		}

		for {
			select {
			case <-ctx.Done():
				released = true
				_ = lease.Discard()
				if strings.TrimSpace(terminalErrorSummary) == "" {
					terminalErrorSummary = ctx.Err().Error()
				}
				return finalizeTurn(ctx, finalText, true)
			case note, ok := <-client.Notifications():
				if !ok {
					released = true
					_ = lease.Discard()
					waitErr := client.Wait()
					if waitErr != nil && strings.TrimSpace(terminalErrorSummary) == "" {
						terminalErrorSummary = waitErr.Error()
					}
					hasAnyActivity := strings.TrimSpace(finalText) != "" ||
						strings.TrimSpace(assistantBuf.String()) != "" ||
						strings.TrimSpace(reasoningSummaryBuf.String()) != "" ||
						strings.TrimSpace(reasoningContentBuf.String()) != "" ||
						strings.TrimSpace(progressBuf.String()) != ""
					if !hasAnyActivity {
						for _, state := range feedEmitter.toolStates {
							if state != nil && state.visible() {
								hasAnyActivity = true
								break
							}
						}
					}
					if !hasAnyActivity {
						for _, buf := range feedEmitter.planText {
							if buf != nil && strings.TrimSpace(buf.String()) != "" {
								hasAnyActivity = true
								break
							}
						}
					}
					if !hasAnyActivity {
						if waitErr != nil {
							return TurnResult{}, &codexAppServerEarlyFailure{err: fmt.Errorf("codex app-server closed before producing output: %w", waitErr)}
						}
						return TurnResult{}, &codexAppServerEarlyFailure{err: errors.New("codex app-server closed before producing output")}
					}
					return finalizeTurn(ctx, finalText, true)
				}
				if noteTurnID := codexAppServerNotificationTurnID(note.Params); noteTurnID != "" && strings.TrimSpace(activeTurnID) != "" && noteTurnID != strings.TrimSpace(activeTurnID) {
					continue
				}

				switch note.Method {
				case "thread/tokenUsage/updated":
					metrics = parseCodexAppServerTurnMetrics(note.Params)
					continue
				case "error":
					if msg, willRetry := parseCodexAppServerErrorNotification(note.Params); msg != "" && !willRetry {
						terminalErrorSummary = msg
					}
					// still forward into feedEmitter below
				case "turn/completed":
					if _, errMsg := parseCodexAppServerTurnCompletion(note.Params); errMsg != "" {
						terminalErrorSummary = errMsg
					}
					return finalizeTurn(ctx, finalText, false)
				case "item/completed":
					snapshot := parseCodexAppServerCompletedItem(note.Params)
					switch snapshot.Type {
					case "agentMessage":
						if strings.TrimSpace(finalText) == "" && strings.TrimSpace(snapshot.Text) != "" {
							finalText = strings.TrimSpace(snapshot.Text)
						}
					case "reasoning":
						if reasoningSummaryBuf.Len() == 0 && len(snapshot.Summary) > 0 {
							reasoningSummaryBuf.WriteString(strings.TrimSpace(strings.Join(snapshot.Summary, "\n")))
						}
						if reasoningContentBuf.Len() == 0 && len(snapshot.Content) > 0 {
							reasoningContentBuf.WriteString(strings.TrimSpace(strings.Join(snapshot.Content, "\n")))
						}
					}
				}

				feedEmitter.consume(ctx, note.Method, note.Params)
				events := parseCodexAppServerNotification(note.Method, note.Params)
				for _, event := range events {
					switch event.Type {
					case "session":
						if strings.TrimSpace(event.SessionID) != "" {
							threadID = strings.TrimSpace(event.SessionID)
							lease.SetThreadID(threadID)
						}
					case "assistant_delta":
						if event.Delta == "" {
							continue
						}
						assistantBuf.WriteString(event.Delta)
						m.broadcast("chat.turn.delta", map[string]any{"session_id": sess.ID, "delta": event.Delta})
					case "thinking_delta":
						if event.Delta == "" {
							continue
						}
						if note.Method == "item/reasoning/summaryTextDelta" {
							sawReasoningSummary = true
							reasoningSummaryBuf.WriteString(event.Delta)
						} else if note.Method == "item/reasoning/textDelta" {
							if sawReasoningSummary {
								continue
							}
							reasoningContentBuf.WriteString(event.Delta)
						} else {
							progressBuf.WriteString(event.Delta)
						}
						m.broadcast("chat.turn.thinking.delta", map[string]any{"session_id": sess.ID, "delta": event.Delta})
					case "progress_delta":
						if event.Delta == "" {
							continue
						}
						progressBuf.WriteString(event.Delta)
						m.broadcast("chat.turn.thinking.delta", map[string]any{"session_id": sess.ID, "delta": event.Delta})
					}
				}
			}
		}
	}

	if attemptResume != "" {
		prompt := buildCLIIncrementalPrompt(userMsg, modelInput)
		result, err := runOnce(prompt, "cli_resume", attemptResume)
		if err == nil {
			return result, nil
		}
		var early *codexAppServerEarlyFailure
		if !errors.As(err, &early) {
			return TurnResult{}, err
		}
	}

	if err := m.ensureCompaction(ctx, sess, modelInput, runner.SDKSpec{}, nil); err != nil {
		return TurnResult{}, err
	}
	messages, err := m.store.ListChatMessages(ctx, sess.ID, 1000)
	if err != nil {
		return TurnResult{}, err
	}
	prompt, _ := buildCLITurnPrompt(sess, messages, userMsg, modelInput, m.keepRecent)
	result, err := runOnce(prompt, "cli_reconstructed", "")
	if err != nil {
		return TurnResult{}, err
	}
	return result, nil
}

func parseCodexAppServerNotification(method string, params json.RawMessage) []cliStreamEvent {
	var raw map[string]any
	if len(params) > 0 {
		_ = json.Unmarshal(params, &raw)
	}
	suffix := codexEventSuffix(method)
	switch suffix {
	case "thread_started":
		if sid := firstNonEmptyTrimmed(stringValue(raw["threadId"]), stringValue(raw["thread_id"])); sid != "" {
			return []cliStreamEvent{{Type: "session", SessionID: sid}}
		}
	case "agent_message_content_delta":
		if delta := extractDeltaText(raw); delta != "" {
			return []cliStreamEvent{{Type: "assistant_delta", Delta: delta}}
		}
	case "reasoning_content_delta", "reasoning_summary_text_delta", "reasoning_text_delta":
		if delta := extractDeltaText(raw); delta != "" {
			return []cliStreamEvent{{Type: "thinking_delta", Delta: delta}}
		}
	case "plan_delta":
		if delta := extractDeltaText(raw); delta != "" {
			return []cliStreamEvent{{Type: "progress_delta", Delta: delta}}
		}
	case "task_started":
		msg := firstNonEmptyTrimmed(stringValue(raw["message"]), stringValue(raw["title"]), "task started")
		return []cliStreamEvent{{Type: "progress_delta", Delta: msg}}
	case "mcp_startup_update":
		phase := firstNonEmptyTrimmed(stringValue(raw["phase"]), stringValue(raw["status"]), stringValue(raw["state"]), "starting")
		msg := firstNonEmptyTrimmed(stringValue(raw["message"]), "MCP: "+phase)
		return []cliStreamEvent{{Type: "progress_delta", Delta: msg}}
	case "mcp_startup_complete":
		return []cliStreamEvent{{Type: "progress_delta", Delta: "MCP ready"}}
	case "item_started", "item_completed":
		snapshot := parseCodexEventItemSnapshot(raw)
		if normalizeItemType(snapshot.Type) == "command_execution" && strings.TrimSpace(snapshot.Command) != "" {
			prefix := "[tool] "
			if suffix == "item_completed" {
				prefix = "[tool] done: "
			}
			return []cliStreamEvent{{Type: "progress_delta", Delta: prefix + strings.TrimSpace(snapshot.Command)}}
		}
	}
	return nil
}

func parseCodexAppServerErrorNotification(params json.RawMessage) (string, bool) {
	var raw map[string]any
	if len(params) > 0 {
		if err := json.Unmarshal(params, &raw); err != nil {
			return "", false
		}
	}
	content := firstNonEmptyTrimmed(
		nestedString(raw, "error", "message"),
		stringValue(raw["message"]),
		"Codex CLI error",
	)
	willRetry := false
	if b := boolValue(raw["willRetry"]); b != nil {
		willRetry = *b
	} else if b := boolValue(raw["will_retry"]); b != nil {
		willRetry = *b
	}
	return content, willRetry
}

func parseCodexAppServerCompletedItem(params json.RawMessage) codexCompletedItemSnapshot {
	var raw map[string]any
	if err := json.Unmarshal(params, &raw); err != nil {
		return codexCompletedItemSnapshot{}
	}
	snapshot := parseCodexEventItemSnapshot(raw)
	return codexCompletedItemSnapshot{
		Type:             strings.TrimSpace(snapshot.Type),
		Text:             snapshot.Text,
		Summary:          snapshot.Summary,
		Content:          snapshot.Content,
		Command:          snapshot.Command,
		AggregatedOutput: snapshot.AggregatedOutput,
	}
}

type codexCompletedItemSnapshot struct {
	Type             string
	Text             string
	Summary          []string
	Content          []string
	Command          string
	AggregatedOutput string
}

func parseCodexAppServerTurnMetrics(params json.RawMessage) codexAppServerTurnMetrics {
	var payload struct {
		TokenUsage struct {
			Last struct {
				InputTokens       int64 `json:"inputTokens"`
				CachedInputTokens int64 `json:"cachedInputTokens"`
				OutputTokens      int64 `json:"outputTokens"`
			} `json:"last"`
		} `json:"tokenUsage"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return codexAppServerTurnMetrics{}
	}
	tokenIn := payload.TokenUsage.Last.InputTokens + payload.TokenUsage.Last.CachedInputTokens
	tokenOut := payload.TokenUsage.Last.OutputTokens
	cached := payload.TokenUsage.Last.CachedInputTokens
	return codexAppServerTurnMetrics{
		TokenIn:           pointerInt64(tokenIn),
		TokenOut:          pointerInt64(tokenOut),
		CachedInputTokens: pointerInt64(cached),
	}
}

func shouldFallbackCodexClosedBeforeCompletion(finalText, assistantText, reasoningSummary, reasoningContent string) bool {
	return strings.TrimSpace(finalText) == "" &&
		strings.TrimSpace(assistantText) == "" &&
		strings.TrimSpace(reasoningSummary) == "" &&
		strings.TrimSpace(reasoningContent) == ""
}

func shouldFallbackCodexStreamDisconnect(err error, finalText, assistantText, reasoningSummary, reasoningContent string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if !strings.Contains(message, "stream disconnected before completion") &&
		!strings.Contains(message, "stream closed before response.completed") &&
		!strings.Contains(message, "closed before turn completion") {
		return false
	}
	return shouldFallbackCodexClosedBeforeCompletion(finalText, assistantText, reasoningSummary, reasoningContent)
}

func parseCodexAppServerTurnCompletion(params json.RawMessage) (string, string) {
	var payload struct {
		Turn struct {
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return "", fmt.Sprintf("parse turn/completed: %v", err)
	}
	status := strings.TrimSpace(payload.Turn.Status)
	if status == "" || strings.EqualFold(status, "completed") {
		return "", ""
	}
	if payload.Turn.Error != nil && strings.TrimSpace(payload.Turn.Error.Message) != "" {
		return status, strings.TrimSpace(payload.Turn.Error.Message)
	}
	return status, fmt.Sprintf("codex turn ended with status %s", status)
}

func persistCodexChatArtifacts(dir, threadID, model, workspace, finalText string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	finalText = strings.TrimSpace(finalText)
	if finalText != "" {
		if err := os.WriteFile(filepath.Join(dir, "final_message.md"), []byte(finalText), 0o644); err != nil {
			return err
		}
	}
	modifiedCode := detectWorkspaceModified(workspace)
	summary := cliruntime.Summary{
		Status:       "ok",
		Summary:      summarizeArtifactText(finalText),
		ModifiedCode: modifiedCode,
		NextAction:   "",
		KeyFiles:     []string{},
	}
	summaryBytes, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), summaryBytes, 0o644); err != nil {
		return err
	}
	artifacts := map[string]any{
		"artifacts": []map[string]any{{
			"kind":    "cli_session_summary",
			"title":   "Codex Final Summary",
			"summary": summary.Summary,
		}},
	}
	artifactsBytes, err := json.Marshal(artifacts)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "artifacts.json"), artifactsBytes, 0o644); err != nil {
		return err
	}
	if strings.TrimSpace(threadID) != "" {
		sessionBytes, err := json.Marshal(cliruntime.Session{ToolID: "codex", SessionID: strings.TrimSpace(threadID), Model: strings.TrimSpace(model)})
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "session.json"), sessionBytes, 0o644); err != nil {
			return err
		}
	}
	if modifiedCode {
		_ = writeWorkspacePatch(filepath.Join(dir, "patch.diff"), workspace)
	}
	return nil
}

func summarizeArtifactText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Codex chat turn finished"
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func detectWorkspaceModified(workspace string) bool {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return false
	}
	if err := exec.Command("git", "-C", workspace, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return false
	}
	err := exec.Command("git", "-C", workspace, "diff", "--quiet", "--ignore-submodules", "--exit-code").Run()
	return err != nil
}

func writeWorkspacePatch(path, workspace string) error {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil
	}
	out, err := exec.Command("git", "-C", workspace, "diff", "--no-ext-diff").Output()
	if err != nil && len(out) == 0 {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

const (
	codexAppServerStdoutLineMaxBytes = 32 << 20 // 32MB
	codexAppServerStderrLineMaxBytes = 2 << 20  // 2MB

	codexAppServerDiagnosticsMaxBytes int64 = 1 << 20 // 1MB per diagnostics file per turn

	codexAppServerOverloadMaxRetries  = 5
	codexAppServerOverloadBaseBackoff = 200 * time.Millisecond
	codexAppServerOverloadMaxBackoff  = 5 * time.Second

	codexAppServerInitializeTimeout  = 15 * time.Second
	codexAppServerThreadStartTimeout = 2 * time.Minute
	codexAppServerTurnStartTimeout   = 90 * time.Second
)

var codexAppServerSleep = func(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type stdioCodexAppServerClient struct {
	cancel context.CancelFunc
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	enc    *json.Encoder

	writeMu sync.Mutex

	notifications chan codexAppServerNotification

	pendingMu sync.Mutex
	pending   map[string]chan codexRPCEnvelope
	nextID    int64

	stderrMu    sync.Mutex
	stderrLines []string

	diagMu        sync.Mutex
	diagDir       string
	diagBytes     map[string]int64
	diagTruncated map[string]bool

	readDone chan struct{}
	waitDone chan struct{}
	readErr  error
	waitErr  error
}

type codexRPCEnvelope struct {
	ID     *json.RawMessage `json:"id,omitempty"`
	Method string           `json:"method,omitempty"`
	Params json.RawMessage  `json:"params,omitempty"`
	Result json.RawMessage  `json:"result,omitempty"`
	Error  *codexRPCError   `json:"error,omitempty"`
}

type codexRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func startCodexAppServerClient(ctx context.Context, spec runner.RunSpec) (codexAppServerClient, error) {
	cliCmd := firstNonEmptyTrimmed(strings.TrimSpace(spec.Env["VIBE_TREE_CLI_COMMAND_PATH"]), "codex")
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, cliCmd, "app-server", "--listen", "stdio://")
	cmd.Dir = firstNonEmptyTrimmed(strings.TrimSpace(spec.Cwd), ".")
	envOverride := cloneEnvMap(spec.Env)
	applyCodexAppServerDefaultEnv(envOverride)
	cmd.Env = mergeCodexAppServerEnv(os.Environ(), envOverride)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	client := &stdioCodexAppServerClient{
		cancel:        cancel,
		cmd:           cmd,
		stdin:         stdin,
		enc:           json.NewEncoder(stdin),
		notifications: make(chan codexAppServerNotification, 256),
		pending:       make(map[string]chan codexRPCEnvelope),
		diagBytes:     make(map[string]int64),
		diagTruncated: make(map[string]bool),
		readDone:      make(chan struct{}),
		waitDone:      make(chan struct{}),
	}
	go client.readLoop(stdout)
	go client.stderrLoop(stderr)
	go client.waitLoop()
	return client, nil
}

func (c *stdioCodexAppServerClient) Initialize(ctx context.Context) error {
	var result map[string]any
	if err := c.call(ctx, "initialize", map[string]any{
		"clientInfo": map[string]any{
			"name":    "vibe_tree",
			"title":   "vibe-tree",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"experimentalApi":           true,
			"optOutNotificationMethods": codexAppServerOptOutNotificationMethods,
		},
	}, &result); err != nil {
		return err
	}
	return c.notify(map[string]any{"method": "initialized"})
}

func (c *stdioCodexAppServerClient) StartThread(ctx context.Context, req codexAppServerThreadRequest) (string, error) {
	var result struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	params := map[string]any{
		"model":          req.Model,
		"cwd":            req.Cwd,
		"approvalPolicy": "never",
		"sandbox":        "danger-full-access",
	}
	if strings.TrimSpace(req.BaseInstructions) != "" {
		params["baseInstructions"] = strings.TrimSpace(req.BaseInstructions)
	}
	if req.Config != nil {
		params["config"] = req.Config
	}
	if err := c.call(ctx, "thread/start", params, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Thread.ID), nil
}

func (c *stdioCodexAppServerClient) ResumeThread(ctx context.Context, req codexAppServerThreadRequest) (string, error) {
	var result struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	params := map[string]any{
		"threadId":       req.ThreadID,
		"model":          req.Model,
		"cwd":            req.Cwd,
		"approvalPolicy": "never",
		"sandbox":        "danger-full-access",
	}
	if strings.TrimSpace(req.BaseInstructions) != "" {
		params["baseInstructions"] = strings.TrimSpace(req.BaseInstructions)
	}
	if req.Config != nil {
		params["config"] = req.Config
	}
	if err := c.call(ctx, "thread/resume", params, &result); err != nil {
		return "", err
	}
	return firstNonEmptyTrimmed(result.Thread.ID, req.ThreadID), nil
}

func (c *stdioCodexAppServerClient) StartTurn(ctx context.Context, threadID string, prompt string, reasoningEffort *string) (string, error) {
	var result struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	params := map[string]any{
		"threadId": threadID,
		"input": []map[string]any{{
			"type":         "text",
			"text":         prompt,
			"textElements": []any{},
		}},
	}
	if reasoningEffort != nil && strings.TrimSpace(*reasoningEffort) != "" {
		params["effort"] = strings.TrimSpace(*reasoningEffort)
	}
	if err := c.call(ctx, "turn/start", params, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Turn.ID), nil
}

func (c *stdioCodexAppServerClient) Notifications() <-chan codexAppServerNotification {
	return c.notifications
}

func (c *stdioCodexAppServerClient) Done() <-chan struct{} {
	return c.waitDone
}

func (c *stdioCodexAppServerClient) Wait() error {
	<-c.waitDone
	if c.readErr != nil {
		return c.readErr
	}
	if c.waitErr != nil {
		return fmt.Errorf("%w%s", c.waitErr, c.stderrSuffix())
	}
	return nil
}

func (c *stdioCodexAppServerClient) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	<-c.readDone
	<-c.waitDone
	return nil
}

func (c *stdioCodexAppServerClient) SetDiagnosticsDir(dir string) {
	if c == nil {
		return
	}
	dir = strings.TrimSpace(dir)
	c.diagMu.Lock()
	defer c.diagMu.Unlock()
	if c.diagDir == dir {
		return
	}
	c.diagDir = dir
	if c.diagBytes == nil {
		c.diagBytes = make(map[string]int64)
	}
	if c.diagTruncated == nil {
		c.diagTruncated = make(map[string]bool)
	}
	for k := range c.diagBytes {
		delete(c.diagBytes, k)
	}
	for k := range c.diagTruncated {
		delete(c.diagTruncated, k)
	}
}

func (c *stdioCodexAppServerClient) appendDiagnostics(relPath, line string) {
	if c == nil {
		return
	}
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}

	var dir string
	var data []byte

	c.diagMu.Lock()
	dir = strings.TrimSpace(c.diagDir)
	if dir == "" {
		c.diagMu.Unlock()
		return
	}
	if c.diagBytes == nil {
		c.diagBytes = make(map[string]int64)
	}
	if c.diagTruncated == nil {
		c.diagTruncated = make(map[string]bool)
	}
	if c.diagTruncated[relPath] {
		c.diagMu.Unlock()
		return
	}
	data = []byte(line + "\n")
	used := c.diagBytes[relPath]
	if used >= codexAppServerDiagnosticsMaxBytes {
		c.diagTruncated[relPath] = true
		c.diagMu.Unlock()
		return
	}
	remaining := codexAppServerDiagnosticsMaxBytes - used
	if int64(len(data)) > remaining {
		data = data[:remaining]
		c.diagTruncated[relPath] = true
	}
	c.diagBytes[relPath] = used + int64(len(data))
	c.diagMu.Unlock()

	path := filepath.Join(dir, filepath.Clean(relPath))
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(data)
	_ = f.Close()
}

type codexRPCResponseError struct {
	Method  string
	Code    int
	Message string
}

func (e *codexRPCResponseError) Error() string {
	if e == nil {
		return "codex app-server error"
	}
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "codex app-server error"
	}
	if strings.TrimSpace(e.Method) == "" {
		return msg
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(e.Method), msg)
}

func isCodexRPCOverloaded(err error) bool {
	var rpcErr *codexRPCResponseError
	if !errors.As(err, &rpcErr) {
		return false
	}
	if rpcErr.Code == -32001 {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(rpcErr.Message))
	return strings.Contains(msg, "overload") || strings.Contains(msg, "retry later") || strings.Contains(msg, "too many requests")
}

func codexAppServerMethodTimeout(method string) time.Duration {
	switch strings.TrimSpace(method) {
	case "initialize":
		return codexAppServerInitializeTimeout
	case "thread/start", "thread/resume":
		return codexAppServerThreadStartTimeout
	case "turn/start":
		return codexAppServerTurnStartTimeout
	default:
		return codexAppServerTurnStartTimeout
	}
}

func codexAppServerOverloadBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := codexAppServerOverloadBaseBackoff
	for i := 0; i < attempt; i++ {
		next := delay * 2
		if next >= codexAppServerOverloadMaxBackoff {
			delay = codexAppServerOverloadMaxBackoff
			break
		}
		delay = next
	}
	jitterN := int64(delay / 4)
	if jitterN <= 0 {
		return delay
	}
	jitter := time.Duration(rand.Int63n(jitterN + 1))
	return delay + jitter
}

func (c *stdioCodexAppServerClient) call(ctx context.Context, method string, params any, out any) error {
	maxRetries := 0
	if method == "thread/start" || method == "thread/resume" || method == "turn/start" || method == "initialize" {
		maxRetries = codexAppServerOverloadMaxRetries
	}
	for attempt := 0; ; attempt++ {
		callCtx := ctx
		cancel := func() {}
		if timeout := codexAppServerMethodTimeout(method); timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		err := c.callOnce(callCtx, method, params, out)
		cancel()
		if err == nil {
			return nil
		}
		if attempt >= maxRetries || !isCodexRPCOverloaded(err) {
			return err
		}
		if sleepErr := codexAppServerSleep(ctx, codexAppServerOverloadBackoff(attempt)); sleepErr != nil {
			return sleepErr
		}
	}
}

func (c *stdioCodexAppServerClient) callOnce(ctx context.Context, method string, params any, out any) error {
	id := c.nextRequestID()
	respCh := make(chan codexRPCEnvelope, 1)
	key := strconv.FormatInt(id, 10)
	c.pendingMu.Lock()
	c.pending[key] = respCh
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, key)
		c.pendingMu.Unlock()
	}()

	if err := c.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.waitDone:
		if c.readErr != nil {
			return c.readErr
		}
		if c.waitErr != nil {
			return fmt.Errorf("%s: %w%s", method, c.waitErr, c.stderrSuffix())
		}
		return fmt.Errorf("%s: codex app-server closed before responding", method)
	case resp := <-respCh:
		if resp.Error != nil {
			return &codexRPCResponseError{Method: method, Code: resp.Error.Code, Message: resp.Error.Message}
		}
		if out != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return err
			}
		}
		return nil
	}
}

func (c *stdioCodexAppServerClient) notify(msg any) error {
	return c.write(msg)
}

func (c *stdioCodexAppServerClient) write(msg any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.enc.Encode(msg)
}

func (c *stdioCodexAppServerClient) nextRequestID() int64 {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.nextID++
	return c.nextID
}

var errCodexAppServerLineTooLong = errors.New("codex app-server line too long")

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func readCodexAppServerLine(r *bufio.Reader, maxBytes int) ([]byte, error) {
	if r == nil {
		return nil, io.EOF
	}
	if maxBytes <= 0 {
		maxBytes = codexAppServerStdoutLineMaxBytes
	}

	out := make([]byte, 0, minInt(maxBytes, 64*1024))
	for {
		chunk, err := r.ReadSlice('\n')
		if len(chunk) > 0 {
			if len(out)+len(chunk) <= maxBytes {
				out = append(out, chunk...)
			} else {
				// Keep a bounded prefix for diagnostics, then drain the rest of the line.
				remaining := maxBytes - len(out)
				if remaining > 0 {
					out = append(out, chunk[:remaining]...)
				}
				for errors.Is(err, bufio.ErrBufferFull) {
					_, err = r.ReadSlice('\n')
				}
				return bytes.TrimRight(out, "\r\n"), errCodexAppServerLineTooLong
			}
		}
		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(out) == 0 {
				return nil, io.EOF
			}
			break
		}
		return nil, err
	}
	return bytes.TrimRight(out, "\r\n"), nil
}

func (c *stdioCodexAppServerClient) readLoop(stdout io.Reader) {
	defer close(c.readDone)
	defer close(c.notifications)
	reader := bufio.NewReaderSize(stdout, 64*1024)
	for {
		line, err := readCodexAppServerLine(reader, codexAppServerStdoutLineMaxBytes)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, errCodexAppServerLineTooLong) {
				c.appendDiagnostics("codex_appserver.stdout.oversize.log", string(bytes.TrimSpace(line)))
			}
			c.readErr = fmt.Errorf("read codex app-server stdout: %w", err)
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var env codexRPCEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			c.appendDiagnostics("codex_appserver.stdout.non_json.log", string(line))
			continue
		}
		if env.Method != "" && env.ID != nil {
			_ = c.write(map[string]any{
				"id":    rawJSONValue(*env.ID),
				"error": map[string]any{"code": -32601, "message": "method not supported by vibe-tree"},
			})
			continue
		}
		if env.Method != "" {
			c.notifications <- codexAppServerNotification{Method: env.Method, Params: env.Params}
			continue
		}
		if env.ID == nil {
			continue
		}
		key := string(*env.ID)
		c.pendingMu.Lock()
		respCh := c.pending[key]
		c.pendingMu.Unlock()
		if respCh != nil {
			respCh <- env
		}
	}
}

func (c *stdioCodexAppServerClient) stderrLoop(stderr io.Reader) {
	reader := bufio.NewReaderSize(stderr, 64*1024)
	for {
		line, err := readCodexAppServerLine(reader, codexAppServerStderrLineMaxBytes)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		text := string(line)
		c.appendDiagnostics("codex_appserver.stderr.log", text)
		c.stderrMu.Lock()
		c.stderrLines = append(c.stderrLines, text)
		if len(c.stderrLines) > 20 {
			c.stderrLines = c.stderrLines[len(c.stderrLines)-20:]
		}
		c.stderrMu.Unlock()
	}
}

func (c *stdioCodexAppServerClient) waitLoop() {
	defer close(c.waitDone)
	c.waitErr = c.cmd.Wait()
}

func (c *stdioCodexAppServerClient) stderrSuffix() string {
	c.stderrMu.Lock()
	defer c.stderrMu.Unlock()
	if len(c.stderrLines) == 0 {
		return ""
	}
	return ": " + strings.Join(c.stderrLines, " | ")
}

func rawJSONValue(raw json.RawMessage) any {
	var out any
	if json.Unmarshal(raw, &out) == nil {
		return out
	}
	return string(raw)
}

func applyCodexAppServerDefaultEnv(env map[string]string) {
	if env == nil {
		return
	}
	defaults := map[string]string{
		"CODEX_NO_INTERACTIVE": "1",
		"NO_COLOR":             "1",
		"CLICOLOR":             "0",
		"RUST_LOG":             "error",
	}
	for key, value := range defaults {
		if _, ok := env[key]; ok {
			continue
		}
		if _, ok := os.LookupEnv(key); ok {
			continue
		}
		env[key] = value
	}
}

func mergeCodexAppServerEnv(base []string, override map[string]string) []string {
	if len(override) == 0 {
		return base
	}
	envMap := make(map[string]string, len(base)+len(override))
	for _, kv := range base {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		envMap[key] = value
	}
	for key, value := range override {
		envMap[key] = value
	}
	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+envMap[key])
	}
	return out
}

func drainCodexAppServerNotifications(client codexAppServerClient) error {
	if client == nil {
		return nil
	}
	for {
		select {
		case _, ok := <-client.Notifications():
			if !ok {
				return client.Wait()
			}
		default:
			return nil
		}
	}
}

func codexAppServerNotificationTurnID(params json.RawMessage) string {
	var raw map[string]any
	if err := json.Unmarshal(params, &raw); err != nil {
		return ""
	}
	return firstNonEmptyTrimmed(
		stringValue(raw["turnId"]),
		stringValue(raw["turn_id"]),
		nestedString(raw, "turn", "id"),
		nestedString(raw, "params", "turnId"),
	)
}
