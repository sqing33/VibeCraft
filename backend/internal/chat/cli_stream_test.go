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

func TestCLIStreamParser_OpenCodeTracksPartKinds(t *testing.T) {
	parser := newCLIStreamParser("opencode")
	events := parser.Parse(`{"type":"session.created","properties":{"info":{"id":"ses_123"}}}`)
	if len(events) == 0 || events[0].Type != "session" || events[0].SessionID != "ses_123" {
		t.Fatalf("unexpected session events: %+v", events)
	}
	events = parser.Parse(`{"type":"message.part.updated","properties":{"part":{"id":"part_reason","sessionID":"ses_123","messageID":"msg_1","type":"reasoning","text":"thinking: "}}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "thinking_delta" || events[1].Delta != "thinking: " {
		t.Fatalf("unexpected reasoning update events: %+v", events)
	}
	events = parser.Parse(`{"type":"message.part.delta","properties":{"sessionID":"ses_123","messageID":"msg_1","partID":"part_reason","field":"text","delta":"step 1"}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "thinking_delta" || events[1].Delta != "step 1" {
		t.Fatalf("unexpected reasoning delta events: %+v", events)
	}
	events = parser.Parse(`{"type":"message.part.updated","properties":{"part":{"id":"part_text","sessionID":"ses_123","messageID":"msg_1","type":"text","text":"hello"}}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "assistant_delta" || events[1].Delta != "hello" {
		t.Fatalf("unexpected text update events: %+v", events)
	}
	events = parser.Parse(`{"type":"message.part.delta","properties":{"sessionID":"ses_123","messageID":"msg_1","partID":"part_text","field":"text","delta":" world"}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "assistant_delta" || events[1].Delta != " world" {
		t.Fatalf("unexpected text delta events: %+v", events)
	}
}

func TestCLIStreamParser_OpenCodeParsesToolAndError(t *testing.T) {
	parser := newCLIStreamParser("opencode")
	events := parser.Parse(`{"type":"message.part.updated","properties":{"part":{"id":"part_tool","sessionID":"ses_123","messageID":"msg_1","type":"tool","tool":"shell","state":{"status":"running","title":"rg"}}}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "progress_delta" || events[1].Delta != "[tool] rg (running)" {
		t.Fatalf("unexpected tool events: %+v", events)
	}
	events = parser.Parse(`{"type":"error","timestamp":1773049125132,"sessionID":"ses_123","error":{"name":"UnknownError","data":{"message":"Model not found: openai/gpt-4o-mini."}}}`)
	if len(events) < 2 {
		t.Fatalf("expected session + final events, got %+v", events)
	}
	if events[0].Type != "session" || events[0].SessionID != "ses_123" {
		t.Fatalf("unexpected session event: %+v", events[0])
	}
	if events[1].Type != "final" || events[1].Text != "Model not found: openai/gpt-4o-mini." {
		t.Fatalf("unexpected final event: %+v", events[1])
	}
}

func TestCLIStreamParser_OpenCodeParsesFlattenedRunEvents(t *testing.T) {
	parser := newCLIStreamParser("opencode")
	events := parser.Parse(`{"type":"text","sessionID":"ses_123","part":{"id":"part_text","sessionID":"ses_123","messageID":"msg_1","type":"text","text":"hello"}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "assistant_delta" || events[1].Delta != "hello" {
		t.Fatalf("unexpected flattened text events: %+v", events)
	}
	events = parser.Parse(`{"type":"step_finish","sessionID":"ses_123","part":{"id":"part_step","sessionID":"ses_123","messageID":"msg_1","type":"step-finish","reason":"stop"}}`)
	if len(events) < 2 || events[0].Type != "session" || events[1].Type != "progress_delta" || events[1].Delta != "[step] stop" {
		t.Fatalf("unexpected flattened step events: %+v", events)
	}
}
