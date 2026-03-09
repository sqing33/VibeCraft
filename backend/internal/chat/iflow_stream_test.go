package chat

import "testing"

func TestParseCLIStreamEvents_IFLOWTreatsLineAsAssistantDelta(t *testing.T) {
	events := parseCLIStreamEvents("iflow", "hello from iflow")
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].Type != "assistant_delta" {
		t.Fatalf("event type = %q, want assistant_delta", events[0].Type)
	}
	if events[0].Delta != "hello from iflow\n" {
		t.Fatalf("event delta = %q, want hello from iflow\\n", events[0].Delta)
	}
}
