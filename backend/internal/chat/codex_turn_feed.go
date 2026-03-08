package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

type chatTurnEventPayload struct {
	SessionID     string         `json:"session_id"`
	UserMessageID string         `json:"user_message_id"`
	EntryID       string         `json:"entry_id"`
	Kind          string         `json:"kind"`
	Op            string         `json:"op"`
	Status        string         `json:"status,omitempty"`
	Delta         string         `json:"delta,omitempty"`
	Content       string         `json:"content,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type codexToolFeedState struct {
	EntryID   string
	Command   string
	Stdout    strings.Builder
	Stderr    strings.Builder
	Status    string
	ExitCode  *int
	Succeeded *bool
}

func (s *codexToolFeedState) meta() map[string]any {
	meta := map[string]any{
		"command": s.Command,
		"stdout":  s.Stdout.String(),
		"stderr":  s.Stderr.String(),
	}
	if s.ExitCode != nil {
		meta["exit_code"] = *s.ExitCode
	}
	if s.Succeeded != nil {
		meta["success"] = *s.Succeeded
	}
	return meta
}

type codexTurnFeedEmitter struct {
	manager       *Manager
	sessionID     string
	userMessageID string
	toolStates    map[string]*codexToolFeedState
	planText      map[string]*strings.Builder
}

func newCodexTurnFeedEmitter(manager *Manager, sessionID, userMessageID string) *codexTurnFeedEmitter {
	return &codexTurnFeedEmitter{
		manager:       manager,
		sessionID:     strings.TrimSpace(sessionID),
		userMessageID: strings.TrimSpace(userMessageID),
		toolStates:    make(map[string]*codexToolFeedState),
		planText:      make(map[string]*strings.Builder),
	}
}

func (e *codexTurnFeedEmitter) emit(payload chatTurnEventPayload) {
	if e == nil || e.manager == nil {
		return
	}
	payload.SessionID = firstNonEmptyTrimmed(payload.SessionID, e.sessionID)
	payload.UserMessageID = firstNonEmptyTrimmed(payload.UserMessageID, e.userMessageID)
	payload.EntryID = strings.TrimSpace(payload.EntryID)
	payload.Kind = strings.TrimSpace(payload.Kind)
	payload.Op = strings.TrimSpace(payload.Op)
	if payload.SessionID == "" || payload.UserMessageID == "" || payload.EntryID == "" || payload.Kind == "" || payload.Op == "" {
		return
	}
	if payload.Meta != nil && len(payload.Meta) == 0 {
		payload.Meta = nil
	}
	e.manager.broadcast("chat.turn.event", payload)
}

func (e *codexTurnFeedEmitter) consume(method string, params json.RawMessage) {
	if e == nil {
		return
	}
	method = strings.TrimSpace(method)
	if method == "" {
		return
	}
	var raw map[string]any
	if len(params) > 0 {
		_ = json.Unmarshal(params, &raw)
	}
	suffix := codexEventSuffix(method)
	switch suffix {
	case "agent_message_content_delta":
		delta := extractDeltaText(raw)
		if delta != "" {
			e.emit(chatTurnEventPayload{EntryID: "answer", Kind: "answer", Op: "append", Status: "streaming", Delta: delta})
		}
	case "reasoning_content_delta", "reasoning_summary_text_delta", "reasoning_text_delta":
		delta := extractDeltaText(raw)
		if delta != "" {
			e.emit(chatTurnEventPayload{EntryID: "thinking", Kind: "thinking", Op: "append", Status: "streaming", Delta: delta})
		}
	case "plan_delta":
		planID := firstNonEmptyTrimmed(nestedString(raw, "itemId"), nestedString(raw, "item_id"), "plan")
		delta := extractDeltaText(raw)
		if delta == "" {
			return
		}
		buf := e.planText[planID]
		if buf == nil {
			buf = &strings.Builder{}
			e.planText[planID] = buf
		}
		buf.WriteString(delta)
		e.emit(chatTurnEventPayload{EntryID: "plan:" + planID, Kind: "plan", Op: "upsert", Status: "streaming", Content: strings.TrimSpace(buf.String())})
	case "task_started":
		content := firstNonEmptyTrimmed(
			stringValue(raw["title"]),
			stringValue(raw["message"]),
			stringValue(raw["hint"]),
			"任务已开始",
		)
		e.emit(chatTurnEventPayload{EntryID: "progress:task", Kind: "progress", Op: "upsert", Status: "pending_approval", Content: content})
	case "mcp_startup_update":
		phase := firstNonEmptyTrimmed(stringValue(raw["phase"]), stringValue(raw["status"]), stringValue(raw["state"]), "starting")
		content := firstNonEmptyTrimmed(stringValue(raw["message"]), fmt.Sprintf("MCP 启动：%s", phase))
		e.emit(chatTurnEventPayload{EntryID: "system:mcp", Kind: "system", Op: "upsert", Status: normalizeFeedStatus(phase), Content: content, Meta: map[string]any{"phase": phase}})
	case "mcp_startup_complete":
		e.emit(chatTurnEventPayload{EntryID: "system:mcp", Kind: "system", Op: "upsert", Status: "done", Content: "MCP 已就绪"})
	case "item_started":
		e.consumeItemSnapshot(method, raw)
	case "item_completed":
		e.consumeItemSnapshot(method, raw)
	case "exec_command_begin":
		callID := firstNonEmptyTrimmed(stringValue(raw["callId"]), stringValue(raw["call_id"]), stringValue(raw["itemId"]), stringValue(raw["item_id"]))
		if callID == "" {
			callID = "command"
		}
		command := firstNonEmptyTrimmed(commandText(raw["command"]), commandText(raw["cmd"]), "command execution")
		state := &codexToolFeedState{EntryID: "tool:" + callID, Command: command, Status: "created"}
		e.toolStates[callID] = state
		e.emit(chatTurnEventPayload{EntryID: state.EntryID, Kind: "tool", Op: "upsert", Status: state.Status, Content: command, Meta: state.meta()})
	case "exec_command_output_delta":
		callID := firstNonEmptyTrimmed(stringValue(raw["callId"]), stringValue(raw["call_id"]), stringValue(raw["itemId"]), stringValue(raw["item_id"]))
		state := e.ensureToolState(callID, commandText(raw["command"]))
		if state == nil {
			return
		}
		chunk := extractChunkText(raw["chunk"])
		if chunk == "" {
			return
		}
		stream := strings.ToLower(firstNonEmptyTrimmed(stringValue(raw["stream"]), "stdout"))
		if stream == "stderr" {
			state.Stderr.WriteString(chunk)
		} else {
			state.Stdout.WriteString(chunk)
		}
		state.Status = "streaming"
		e.emit(chatTurnEventPayload{EntryID: state.EntryID, Kind: "tool", Op: "upsert", Status: state.Status, Content: state.Command, Meta: state.meta()})
	case "exec_command_end":
		callID := firstNonEmptyTrimmed(stringValue(raw["callId"]), stringValue(raw["call_id"]), stringValue(raw["itemId"]), stringValue(raw["item_id"]))
		state := e.ensureToolState(callID, commandText(raw["command"]))
		if state == nil {
			return
		}
		if stdout := strings.TrimSpace(extractChunkText(raw["stdout"])); stdout != "" && state.Stdout.Len() == 0 {
			state.Stdout.WriteString(stdout)
		}
		if stderr := strings.TrimSpace(extractChunkText(raw["stderr"])); stderr != "" && state.Stderr.Len() == 0 {
			state.Stderr.WriteString(stderr)
		}
		if exitCode, ok := intValue(raw["exit_code"]); ok {
			state.ExitCode = &exitCode
		}
		success := boolValue(raw["success"])
		if success == nil && state.ExitCode != nil {
			fallback := *state.ExitCode == 0
			success = &fallback
		}
		state.Succeeded = success
		state.Status = "success"
		if success != nil && !*success {
			state.Status = "failed"
		}
		e.emit(chatTurnEventPayload{EntryID: state.EntryID, Kind: "tool", Op: "upsert", Status: state.Status, Content: state.Command, Meta: state.meta()})
	case "request_user_input":
		entryID := firstNonEmptyTrimmed(stringValue(raw["callId"]), stringValue(raw["call_id"]), "question")
		content, meta := extractQuestionContent(raw)
		e.emit(chatTurnEventPayload{EntryID: "question:" + entryID, Kind: "question", Op: "upsert", Status: "pending_approval", Content: content, Meta: meta})
	case "warning":
		content := firstNonEmptyTrimmed(stringValue(raw["message"]), "warning")
		e.emit(chatTurnEventPayload{EntryID: "system:warning", Kind: "system", Op: "upsert", Status: "created", Content: content})
	case "error":
		content := firstNonEmptyTrimmed(stringValue(raw["message"]), "Codex CLI error")
		e.emit(chatTurnEventPayload{EntryID: "error:codex", Kind: "error", Op: "upsert", Status: "failed", Content: content})
	}
}

func (e *codexTurnFeedEmitter) ensureToolState(callID, command string) *codexToolFeedState {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		callID = "command"
	}
	if existing := e.toolStates[callID]; existing != nil {
		if existing.Command == "" && strings.TrimSpace(command) != "" {
			existing.Command = strings.TrimSpace(command)
		}
		return existing
	}
	command = firstNonEmptyTrimmed(strings.TrimSpace(command), "command execution")
	state := &codexToolFeedState{EntryID: "tool:" + callID, Command: command, Status: "created"}
	e.toolStates[callID] = state
	return state
}

func (e *codexTurnFeedEmitter) consumeItemSnapshot(method string, raw map[string]any) {
	snapshot := parseCodexEventItemSnapshot(raw)
	itemType := normalizeItemType(snapshot.Type)
	switch itemType {
	case "command_execution":
		callID := firstNonEmptyTrimmed(snapshot.ItemID, snapshot.CallID, snapshot.Command)
		state := e.ensureToolState(callID, snapshot.Command)
		if state == nil {
			return
		}
		if method == "item/completed" || strings.HasSuffix(method, "/item_completed") {
			state.Status = "success"
			if strings.TrimSpace(snapshot.AggregatedOutput) != "" && state.Stdout.Len() == 0 {
				state.Stdout.WriteString(strings.TrimSpace(snapshot.AggregatedOutput))
			}
		} else {
			state.Status = "created"
		}
		e.emit(chatTurnEventPayload{EntryID: state.EntryID, Kind: "tool", Op: "upsert", Status: state.Status, Content: state.Command, Meta: state.meta()})
	case "plan":
		entryID := "plan:" + firstNonEmptyTrimmed(snapshot.ItemID, "plan")
		content := firstNonEmptyTrimmed(snapshot.Text, strings.Join(snapshot.Content, "\n"), strings.Join(snapshot.Summary, "\n"))
		if content == "" {
			content = "Plan"
		}
		e.emit(chatTurnEventPayload{EntryID: entryID, Kind: "plan", Op: "upsert", Status: "streaming", Content: strings.TrimSpace(content)})
	case "request_user_input", "question":
		entryID := "question:" + firstNonEmptyTrimmed(snapshot.ItemID, "question")
		content := firstNonEmptyTrimmed(snapshot.Text, "需要你的输入")
		e.emit(chatTurnEventPayload{EntryID: entryID, Kind: "question", Op: "upsert", Status: "pending_approval", Content: content})
	}
}

func codexEventSuffix(method string) string {
	method = strings.TrimSpace(method)
	switch method {
	case "item/agentMessage/delta":
		return "agent_message_content_delta"
	case "item/reasoning/summaryTextDelta":
		return "reasoning_summary_text_delta"
	case "item/reasoning/textDelta":
		return "reasoning_text_delta"
	case "item/plan/delta":
		return "plan_delta"
	case "item/started":
		return "item_started"
	case "item/completed":
		return "item_completed"
	}
	if strings.HasPrefix(method, "codex/event/") {
		return strings.TrimPrefix(method, "codex/event/")
	}
	return strings.ReplaceAll(method, "/", "_")
}

func normalizeItemType(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer("-", "_", " ", "_", ".", "_")
	value = replacer.Replace(value)
	var out strings.Builder
	for i, r := range value {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(r + ('a' - 'A'))
			continue
		}
		out.WriteRune(r)
	}
	return strings.Trim(strings.ToLower(out.String()), "_")
}

func normalizeFeedStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "starting", "created", "queued":
		return "created"
	case "running", "streaming", "ready":
		return "streaming"
	case "done", "completed", "complete", "success":
		return "done"
	case "failed", "error":
		return "failed"
	default:
		return "created"
	}
}

func extractDeltaText(raw map[string]any) string {
	return firstNonBlankPreserve(
		stringValue(raw["delta"]),
		stringValue(raw["content"]),
		nestedString(raw, "msg", "delta"),
		nestedString(raw, "msg", "content"),
		nestedString(raw, "params", "delta"),
		nestedString(raw, "params", "content"),
	)
}

func firstNonBlankPreserve(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func extractChunkText(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case []any:
		buf := make([]byte, 0, len(vv))
		for _, item := range vv {
			if n, ok := intValue(item); ok {
				buf = append(buf, byte(n))
				continue
			}
			if s := stringValue(item); s != "" {
				return s
			}
		}
		return string(buf)
	default:
		return stringifyEventText(v)
	}
}

func commandText(v any) string {
	switch vv := v.(type) {
	case string:
		return strings.TrimSpace(vv)
	case []any:
		parts := make([]string, 0, len(vv))
		for _, item := range vv {
			if part := strings.TrimSpace(stringValue(item)); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	default:
		return strings.TrimSpace(stringifyEventText(v))
	}
}

func intValue(v any) (int, bool) {
	switch vv := v.(type) {
	case int:
		return vv, true
	case int32:
		return int(vv), true
	case int64:
		return int(vv), true
	case float64:
		return int(vv), true
	default:
		return 0, false
	}
}

func boolValue(v any) *bool {
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}

type codexEventItemSnapshot struct {
	Type             string
	ItemID           string
	CallID           string
	Text             string
	Summary          []string
	Content          []string
	Command          string
	AggregatedOutput string
}

func parseCodexEventItemSnapshot(raw map[string]any) codexEventItemSnapshot {
	item, _ := raw["item"].(map[string]any)
	if item == nil {
		item, _ = raw["snapshot"].(map[string]any)
	}
	if item == nil {
		item, _ = raw["msg"].(map[string]any)
	}
	content := extractStringList(item["content"])
	summary := extractStringList(item["summary"])
	return codexEventItemSnapshot{
		Type:             firstNonEmptyTrimmed(stringValue(item["type"]), stringValue(raw["type"])),
		ItemID:           firstNonEmptyTrimmed(stringValue(item["id"]), stringValue(raw["itemId"]), stringValue(raw["item_id"])),
		CallID:           firstNonEmptyTrimmed(stringValue(item["callId"]), stringValue(item["call_id"]), stringValue(raw["callId"]), stringValue(raw["call_id"])),
		Text:             firstNonEmptyTrimmed(stringValue(item["text"]), stringValue(raw["text"])),
		Summary:          summary,
		Content:          content,
		Command:          firstNonEmptyTrimmed(commandText(item["command"]), commandText(raw["command"])),
		AggregatedOutput: firstNonEmptyTrimmed(stringValue(item["aggregatedOutput"]), stringValue(item["aggregated_output"]), stringValue(raw["aggregatedOutput"]), stringValue(raw["aggregated_output"])),
	}
}

func extractStringList(v any) []string {
	items, _ := v.([]any)
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(stringifyEventText(item)); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func extractQuestionContent(raw map[string]any) (string, map[string]any) {
	questions := make([]map[string]any, 0)
	if items, ok := raw["questions"].([]any); ok {
		for _, item := range items {
			question, _ := item.(map[string]any)
			if question == nil {
				continue
			}
			options := make([]map[string]any, 0)
			if rawOptions, ok := question["options"].([]any); ok {
				for _, option := range rawOptions {
					m, _ := option.(map[string]any)
					if m == nil {
						continue
					}
					options = append(options, map[string]any{
						"label":       stringValue(m["label"]),
						"description": stringValue(m["description"]),
					})
				}
			}
			questions = append(questions, map[string]any{
				"header":   stringValue(question["header"]),
				"question": stringValue(question["question"]),
				"options":  options,
			})
		}
	}
	content := "需要你的输入"
	if len(questions) == 1 {
		if question := strings.TrimSpace(stringValue(questions[0]["question"])); question != "" {
			content = question
		}
	} else if len(questions) > 1 {
		content = fmt.Sprintf("需要回答 %d 个问题", len(questions))
	}
	if len(questions) == 0 {
		content = firstNonEmptyTrimmed(stringValue(raw["message"]), content)
	}
	meta := map[string]any{}
	if len(questions) > 0 {
		meta["questions"] = questions
	}
	return content, meta
}
