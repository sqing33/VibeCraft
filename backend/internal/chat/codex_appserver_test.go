package chat

import (
	"encoding/json"
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
