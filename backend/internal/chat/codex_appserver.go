package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"vibe-tree/backend/internal/cliruntime"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

type codexAppServerClient interface {
	Initialize(ctx context.Context) error
	StartThread(ctx context.Context, req codexAppServerThreadRequest) (string, error)
	ResumeThread(ctx context.Context, req codexAppServerThreadRequest) (string, error)
	StartTurn(ctx context.Context, threadID string, prompt string) (string, error)
	Notifications() <-chan codexAppServerNotification
	Wait() error
	Close() error
}

type codexAppServerThreadRequest struct {
	ThreadID         string
	Model            string
	Cwd              string
	BaseInstructions string
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

func (m *Manager) runCLITurn(ctx context.Context, sess store.ChatSession, userMsg store.ChatMessage, modelInput string, spec runner.RunSpec, expertID, provider, model string, thinkingTranslation *ThinkingTranslationSpec) (TurnResult, error) {
	if runner.NormalizeCLIFamily(spec.Env["VIBE_TREE_CLI_FAMILY"]) == "codex" {
		result, err := m.runCodexAppServerTurn(ctx, sess, userMsg, modelInput, spec, expertID, provider, model, thinkingTranslation)
		if err == nil {
			return result, nil
		}
		var early *codexAppServerEarlyFailure
		if errors.As(err, &early) {
			return m.runLegacyCLITurn(ctx, sess, userMsg, modelInput, spec, expertID, provider, model)
		}
		return TurnResult{}, err
	}
	return m.runLegacyCLITurn(ctx, sess, userMsg, modelInput, spec, expertID, provider, model)
}

func (m *Manager) runCodexAppServerTurn(ctx context.Context, sess store.ChatSession, userMsg store.ChatMessage, modelInput string, spec runner.RunSpec, expertID, provider, model string, thinkingTranslation *ThinkingTranslationSpec) (TurnResult, error) {
	artifactDir, err := cliruntime.ChatTurnArtifactDir(sess.ID, userMsg.ID)
	if err != nil {
		artifactDir = ""
	}
	attemptResume := strings.TrimSpace(pointerStringValue(sess.CLISessionID))

	runOnce := func(prompt string, contextMode string, resumeThreadID string) (TurnResult, error) {
		client, err := newCodexAppServerClient(ctx, spec)
		if err != nil {
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}
		defer client.Close()

		if err := client.Initialize(ctx); err != nil {
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}

		threadReq := codexAppServerThreadRequest{
			ThreadID:         resumeThreadID,
			Model:            strings.TrimSpace(spec.Env["VIBE_TREE_MODEL"]),
			Cwd:              firstNonEmptyTrimmed(strings.TrimSpace(spec.Cwd), strings.TrimSpace(sess.WorkspacePath), "."),
			BaseInstructions: strings.TrimSpace(spec.Env["VIBE_TREE_SYSTEM_PROMPT"]),
		}

		threadID := ""
		if strings.TrimSpace(resumeThreadID) != "" {
			threadID, err = client.ResumeThread(ctx, threadReq)
		} else {
			threadID, err = client.StartThread(ctx, threadReq)
		}
		if err != nil {
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}

		if _, err := client.StartTurn(ctx, threadID, prompt); err != nil {
			return TurnResult{}, &codexAppServerEarlyFailure{err: err}
		}

		translationRuntime := newThinkingTranslationRuntime(m, sess.ID, thinkingTranslation)
		var assistantBuf strings.Builder
		var reasoningSummaryBuf strings.Builder
		var reasoningContentBuf strings.Builder
		var progressBuf strings.Builder
		var finalText string
		metrics := codexAppServerTurnMetrics{}
		sawReasoningSummary := false

		for {
			select {
			case <-ctx.Done():
				return TurnResult{}, ctx.Err()
			case note, ok := <-client.Notifications():
				if !ok {
					if err := client.Wait(); err != nil {
						return TurnResult{}, err
					}
					return TurnResult{}, fmt.Errorf("codex app-server closed before turn completion")
				}

				switch note.Method {
				case "thread/tokenUsage/updated":
					metrics = parseCodexAppServerTurnMetrics(note.Params)
					continue
				case "turn/completed":
					if err := parseCodexAppServerTurnCompletion(note.Params); err != nil {
						return TurnResult{}, err
					}
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
					if strings.TrimSpace(finalText) == "" {
						finalText = "(empty response)"
					}

					if artifactDir != "" {
						_ = persistCodexChatArtifacts(artifactDir, threadID, model, sess.WorkspacePath, finalText)
					}

					assistantMsg, err := m.store.AppendChatMessage(ctx, store.AppendChatMessageParams{
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
					_, _ = m.store.UpdateChatSessionDefaults(ctx, store.UpdateChatSessionDefaultsParams{
						SessionID:    sess.ID,
						ExpertID:     expertID,
						CLIToolID:    sess.CLIToolID,
						ModelID:      sess.ModelID,
						CLISessionID: pointerOrNilString(threadID),
						Provider:     provider,
						Model:        model,
					})
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

				events := parseCodexAppServerNotification(note.Method, note.Params)
				for _, event := range events {
					switch event.Type {
					case "session":
						if strings.TrimSpace(event.SessionID) != "" {
							threadID = strings.TrimSpace(event.SessionID)
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
						if translationRuntime != nil && (note.Method == "item/reasoning/summaryTextDelta" || note.Method == "item/reasoning/textDelta") {
							translationRuntime.add(ctx, event.Delta)
						}
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
	switch method {
	case "thread/started":
		var payload struct {
			ThreadID string `json:"threadId"`
		}
		if json.Unmarshal(params, &payload) == nil && strings.TrimSpace(payload.ThreadID) != "" {
			return []cliStreamEvent{{Type: "session", SessionID: strings.TrimSpace(payload.ThreadID)}}
		}
	case "item/agentMessage/delta":
		var payload struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(params, &payload) == nil && payload.Delta != "" {
			return []cliStreamEvent{{Type: "assistant_delta", Delta: payload.Delta}}
		}
	case "item/reasoning/summaryTextDelta", "item/reasoning/textDelta":
		var payload struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(params, &payload) == nil && payload.Delta != "" {
			return []cliStreamEvent{{Type: "thinking_delta", Delta: payload.Delta}}
		}
	case "item/plan/delta":
		var payload struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal(params, &payload) == nil && payload.Delta != "" {
			return []cliStreamEvent{{Type: "progress_delta", Delta: payload.Delta}}
		}
	case "item/started", "item/completed":
		snapshot := parseCodexAppServerCompletedItem(params)
		if snapshot.Type == "commandExecution" && strings.TrimSpace(snapshot.Command) != "" {
			prefix := "[tool] "
			if method == "item/completed" {
				prefix = "[tool] done: "
			}
			return []cliStreamEvent{{Type: "progress_delta", Delta: prefix + strings.TrimSpace(snapshot.Command)}}
		}
	}
	return nil
}

func parseCodexAppServerCompletedItem(params json.RawMessage) codexCompletedItemSnapshot {
	var payload struct {
		Item struct {
			Type             string   `json:"type"`
			Text             string   `json:"text"`
			Summary          []string `json:"summary"`
			Content          []string `json:"content"`
			Command          string   `json:"command"`
			AggregatedOutput string   `json:"aggregatedOutput"`
		} `json:"item"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return codexCompletedItemSnapshot{}
	}
	return codexCompletedItemSnapshot{
		Type:             strings.TrimSpace(payload.Item.Type),
		Text:             payload.Item.Text,
		Summary:          payload.Item.Summary,
		Content:          payload.Item.Content,
		Command:          payload.Item.Command,
		AggregatedOutput: payload.Item.AggregatedOutput,
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

func parseCodexAppServerTurnCompletion(params json.RawMessage) error {
	var payload struct {
		Turn struct {
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return err
	}
	status := strings.TrimSpace(payload.Turn.Status)
	if status == "" || strings.EqualFold(status, "completed") {
		return nil
	}
	if payload.Turn.Error != nil && strings.TrimSpace(payload.Turn.Error.Message) != "" {
		return fmt.Errorf("codex turn %s: %s", status, strings.TrimSpace(payload.Turn.Error.Message))
	}
	return fmt.Errorf("codex turn ended with status %s", status)
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
}

func startCodexAppServerClient(ctx context.Context, spec runner.RunSpec) (codexAppServerClient, error) {
	cliCmd := firstNonEmptyTrimmed(strings.TrimSpace(spec.Env["VIBE_TREE_CLI_COMMAND_PATH"]), "codex")
	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, cliCmd, "app-server", "--listen", "stdio://")
	cmd.Dir = firstNonEmptyTrimmed(strings.TrimSpace(spec.Cwd), ".")
	cmd.Env = mergeCodexAppServerEnv(os.Environ(), spec.Env)

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
			"experimentalApi": true,
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
	if err := c.call(ctx, "thread/resume", params, &result); err != nil {
		return "", err
	}
	return firstNonEmptyTrimmed(result.Thread.ID, req.ThreadID), nil
}

func (c *stdioCodexAppServerClient) StartTurn(ctx context.Context, threadID string, prompt string) (string, error) {
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
	if err := c.call(ctx, "turn/start", params, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Turn.ID), nil
}

func (c *stdioCodexAppServerClient) Notifications() <-chan codexAppServerNotification {
	return c.notifications
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

func (c *stdioCodexAppServerClient) call(ctx context.Context, method string, params any, out any) error {
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
			return fmt.Errorf("%s: %s", method, strings.TrimSpace(resp.Error.Message))
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

func (c *stdioCodexAppServerClient) readLoop(stdout io.Reader) {
	defer close(c.readDone)
	defer close(c.notifications)
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var env codexRPCEnvelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			c.readErr = fmt.Errorf("parse codex app-server stdout: %w", err)
			return
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
	if err := scanner.Err(); err != nil {
		c.readErr = err
	}
}

func (c *stdioCodexAppServerClient) stderrLoop(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		c.stderrMu.Lock()
		c.stderrLines = append(c.stderrLines, line)
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
