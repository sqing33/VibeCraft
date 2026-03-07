package chat

import "testing"

func TestParseCodexCLIStreamEvents(t *testing.T) {
	line := `{"type":"item.completed","item":{"type":"agent_message","text":"hello"}}`
	events := parseCodexCLIStreamEvents(line)
	if len(events) == 0 || events[0].Type != "assistant_delta" || events[0].Delta != "hello" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseClaudeCLIStreamEvents(t *testing.T) {
	line := `{"type":"stream_event","session_id":"sess-1","event":{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"let me think"}}}`
	events := parseClaudeCLIStreamEvents(line)
	if len(events) < 2 {
		t.Fatalf("expected session + thinking events, got %+v", events)
	}
	if events[0].Type != "session" || events[0].SessionID != "sess-1" {
		t.Fatalf("unexpected session event: %+v", events[0])
	}
	if events[1].Type != "thinking_delta" || events[1].Delta != "let me think" {
		t.Fatalf("unexpected thinking event: %+v", events[1])
	}
}
