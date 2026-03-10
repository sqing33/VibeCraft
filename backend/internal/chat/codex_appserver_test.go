package chat

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseCodexAppServerNotificationAgentDelta(t *testing.T) {
	events := parseCodexAppServerNotification("item/agentMessage/delta", json.RawMessage(`{"threadId":"th_1","turnId":"tu_1","itemId":"it_1","delta":"hel"}`))
	if len(events) != 1 || events[0].Type != "assistant_delta" || events[0].Delta != "hel" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationReasoningDelta(t *testing.T) {
	events := parseCodexAppServerNotification("item/reasoning/summaryTextDelta", json.RawMessage(`{"threadId":"th_1","turnId":"tu_1","itemId":"it_2","summaryIndex":0,"delta":"thinking"}`))
	if len(events) != 1 || events[0].Type != "thinking_delta" || events[0].Delta != "thinking" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationCommandStarted(t *testing.T) {
	events := parseCodexAppServerNotification("item/started", json.RawMessage(`{"threadId":"th_1","turnId":"tu_1","item":{"type":"commandExecution","id":"it_3","command":"git status"}}`))
	if len(events) != 1 || events[0].Type != "progress_delta" || events[0].Delta != "[tool] git status" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationLegacyReasoningDelta(t *testing.T) {
	events := parseCodexAppServerNotification("codex/event/reasoning_content_delta", json.RawMessage(`{"delta":"plan more"}`))
	if len(events) != 1 || events[0].Type != "thinking_delta" || events[0].Delta != "plan more" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationLegacyAnswerDelta(t *testing.T) {
	events := parseCodexAppServerNotification("codex/event/agent_message_content_delta", json.RawMessage(`{"delta":"done"}`))
	if len(events) != 1 || events[0].Type != "assistant_delta" || events[0].Delta != "done" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationPreservesDeltaWhitespace(t *testing.T) {
	events := parseCodexAppServerNotification("codex/event/agent_message_content_delta", json.RawMessage("{\"delta\":\" hello\\n\"}"))
	if len(events) != 1 || events[0].Type != "assistant_delta" || events[0].Delta != " hello\n" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationLegacyTaskStarted(t *testing.T) {
	events := parseCodexAppServerNotification("codex/event/task_started", json.RawMessage(`{"message":"Starting task"}`))
	if len(events) != 1 || events[0].Type != "progress_delta" || events[0].Delta != "Starting task" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerNotificationLegacyMCPReady(t *testing.T) {
	events := parseCodexAppServerNotification("codex/event/mcp_startup_complete", json.RawMessage(`{"status":"ready"}`))
	if len(events) != 1 || events[0].Type != "progress_delta" || events[0].Delta != "MCP ready" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseCodexAppServerTurnMetrics(t *testing.T) {
	metrics := parseCodexAppServerTurnMetrics(json.RawMessage(`{"threadId":"th_1","turnId":"tu_1","tokenUsage":{"last":{"inputTokens":10,"cachedInputTokens":3,"outputTokens":7}}}`))
	if metrics.TokenIn == nil || *metrics.TokenIn != 13 {
		t.Fatalf("token_in=%v", metrics.TokenIn)
	}
	if metrics.TokenOut == nil || *metrics.TokenOut != 7 {
		t.Fatalf("token_out=%v", metrics.TokenOut)
	}
	if metrics.CachedInputTokens == nil || *metrics.CachedInputTokens != 3 {
		t.Fatalf("cached=%v", metrics.CachedInputTokens)
	}
}

func TestShouldFallbackCodexStreamDisconnect_OnlyBeforeVisibleOutput(t *testing.T) {
	err := errors.New("stream disconnected before completion: stream closed before response.completed")
	if !shouldFallbackCodexStreamDisconnect(err, "", "", "", "") {
		t.Fatalf("expected fallback for empty output disconnect")
	}
	if shouldFallbackCodexStreamDisconnect(err, "", "partial answer", "", "") {
		t.Fatalf("expected no fallback after assistant output")
	}
	if shouldFallbackCodexStreamDisconnect(err, "", "", "已有思考", "") {
		t.Fatalf("expected no fallback after reasoning output")
	}
}

func TestShouldFallbackCodexClosedBeforeCompletion_RequiresEmptyVisibleOutput(t *testing.T) {
	if !shouldFallbackCodexClosedBeforeCompletion("", "", "", "") {
		t.Fatalf("expected fallback when no visible output exists")
	}
	if shouldFallbackCodexClosedBeforeCompletion("done", "", "", "") {
		t.Fatalf("expected no fallback after final text exists")
	}
}
