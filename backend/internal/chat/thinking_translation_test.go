package chat

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestThinkingTranslationRuntime_FlushesOnBoundaryAndComplete(t *testing.T) {
	mgr := NewManager(nil, nil, Options{
		ThinkingTranslationMinChars:   5,
		ThinkingTranslationForceChars: 50,
		ThinkingTranslationIdle:       time.Hour,
		ThinkingTranslator: func(_ context.Context, _ ThinkingTranslationSpec, text string) (string, error) {
			return "[中]" + text, nil
		},
	})
	runtime := newThinkingTranslationRuntime(mgr, "cs_1", "ct_1", &ThinkingTranslationSpec{Provider: "openai", Model: "translator"})
	runtime.add(context.Background(), "", "First sentence.")
	if got := runtime.translatedText(); got != "[中]First sentence." {
		t.Fatalf("unexpected translated text after boundary flush: %q", got)
	}
	runtime.add(context.Background(), "", "tail without boundary")
	if got := runtime.translatedText(); got != "[中]First sentence." {
		t.Fatalf("unexpected translated text before complete: %q", got)
	}
	runtime.complete(context.Background())
	if got := runtime.translatedText(); got != "[中]First sentence.[中]tail without boundary" {
		t.Fatalf("unexpected translated text after complete: %q", got)
	}
}

func TestThinkingTranslationRuntime_FlushesBufferedTextAfterIdleGap(t *testing.T) {
	mgr := NewManager(nil, nil, Options{
		ThinkingTranslationMinChars:   100,
		ThinkingTranslationForceChars: 200,
		ThinkingTranslationIdle:       time.Millisecond,
		ThinkingTranslator: func(_ context.Context, _ ThinkingTranslationSpec, text string) (string, error) {
			return "[中]" + text, nil
		},
	})
	runtime := newThinkingTranslationRuntime(mgr, "cs_1", "ct_1", &ThinkingTranslationSpec{Provider: "openai", Model: "translator"})
	runtime.add(context.Background(), "", "buffered")
	time.Sleep(5 * time.Millisecond)
	runtime.add(context.Background(), "", "next sentence.")
	runtime.complete(context.Background())
	if got := runtime.translatedText(); got != "[中]buffered[中]next sentence." {
		t.Fatalf("unexpected translated text after idle flush: %q", got)
	}
}

func TestThinkingTranslationRuntime_MarksFailureWithoutBreakingRawFlow(t *testing.T) {
	mgr := NewManager(nil, nil, Options{
		ThinkingTranslationMinChars:   5,
		ThinkingTranslationForceChars: 50,
		ThinkingTranslationIdle:       time.Hour,
		ThinkingTranslator: func(_ context.Context, _ ThinkingTranslationSpec, _ string) (string, error) {
			return "", errors.New("boom")
		},
	})
	runtime := newThinkingTranslationRuntime(mgr, "cs_1", "ct_1", &ThinkingTranslationSpec{Provider: "openai", Model: "translator"})
	runtime.add(context.Background(), "", "Hello.")
	if !runtime.failedState() {
		t.Fatalf("expected failed state")
	}
	if got := runtime.translatedText(); got != "" {
		t.Fatalf("expected no translated text on failure, got %q", got)
	}
}

func TestThinkingTranslationRuntime_ForceSegmentKeepsEntryIDUntilEmit(t *testing.T) {
	runtime := &thinkingTranslationRuntime{buffer: "Hello.", bufferEntryID: "thinking:2"}
	segment := runtime.nextSegment(true)
	if segment != "Hello." {
		t.Fatalf("unexpected segment: %q", segment)
	}
	if runtime.bufferEntryID != "thinking:2" {
		t.Fatalf("expected bufferEntryID to survive force split, got %q", runtime.bufferEntryID)
	}
}
