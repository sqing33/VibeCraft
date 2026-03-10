package chat

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNeedsChineseTranslation(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "english", text: "I will inspect the code path and then fix the bug.", want: true},
		{name: "chinese", text: "我先检查一下这段逻辑，然后再修复这个问题。", want: false},
		{name: "mixed chinese dominant", text: "我先检查 gpt-5-codex 的调用链，然后继续处理。", want: false},
		{name: "japanese", text: "今日は計画を整理してから修正します。", want: true},
		{name: "blank", text: "   ", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsChineseTranslation(tc.text); got != tc.want {
				t.Fatalf("needsChineseTranslation(%q)=%v want=%v", tc.text, got, tc.want)
			}
		})
	}
}

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
	if !runtime.applied() {
		t.Fatalf("expected applied state after successful translation")
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

func TestThinkingTranslationRuntime_SkipsChineseDominantEntry(t *testing.T) {
	calls := 0
	mgr := NewManager(nil, nil, Options{
		ThinkingTranslationMinChars:   5,
		ThinkingTranslationForceChars: 50,
		ThinkingTranslationIdle:       time.Hour,
		ThinkingTranslator: func(_ context.Context, _ ThinkingTranslationSpec, text string) (string, error) {
			calls++
			return "[中]" + text, nil
		},
	})
	runtime := newThinkingTranslationRuntime(mgr, "cs_1", "ct_1", &ThinkingTranslationSpec{Provider: "openai", Model: "translator"})
	runtime.add(context.Background(), "thinking:1", "我先检查这一段逻辑。")
	runtime.complete(context.Background())
	if calls != 0 {
		t.Fatalf("expected translator not to be called, got %d", calls)
	}
	if got := runtime.translatedText(); got != "" {
		t.Fatalf("expected no translated text, got %q", got)
	}
	if runtime.applied() {
		t.Fatalf("expected applied=false for chinese-dominant thinking")
	}
}

func TestThinkingTranslationRuntime_KeepsTinyIdleBufferUntilBetterChunk(t *testing.T) {
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
	runtime.add(context.Background(), "", " next sentence.")
	if got := runtime.translatedText(); got != "" {
		t.Fatalf("unexpected translated text before complete: %q", got)
	}
	runtime.complete(context.Background())
	if got := runtime.translatedText(); got != "[中]buffered next sentence." {
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

func TestThinkingTranslationRuntime_ClosedShortEntryWaitsUntilComplete(t *testing.T) {
	mgr := NewManager(nil, nil, Options{
		ThinkingTranslationMinChars:   5,
		ThinkingTranslationForceChars: 50,
		ThinkingTranslationIdle:       time.Hour,
		ThinkingTranslator: func(_ context.Context, _ ThinkingTranslationSpec, text string) (string, error) {
			return "[中]" + text, nil
		},
	})
	runtime := newThinkingTranslationRuntime(mgr, "cs_1", "ct_1", &ThinkingTranslationSpec{Provider: "openai", Model: "translator"})
	runtime.add(context.Background(), "thinking:1", "tiny")
	runtime.closeEntry(context.Background(), "thinking:1")
	runtime.add(context.Background(), "thinking:2", "Second sentence.")
	if got := runtime.translatedText(); got != "[中]Second sentence." {
		t.Fatalf("unexpected translated text before complete: %q", got)
	}
	runtime.complete(context.Background())
	if got := runtime.translatedText(); got != "[中]tiny[中]Second sentence." {
		t.Fatalf("unexpected translated text after complete: %q", got)
	}
}

func TestThinkingTranslationRuntime_PreservesWhitespaceAcrossDeltas(t *testing.T) {
	inputs := make([]string, 0, 1)
	mgr := NewManager(nil, nil, Options{
		ThinkingTranslationMinChars:   1,
		ThinkingTranslationForceChars: 50,
		ThinkingTranslationIdle:       time.Hour,
		ThinkingTranslator: func(_ context.Context, _ ThinkingTranslationSpec, text string) (string, error) {
			inputs = append(inputs, text)
			return "[中]" + text, nil
		},
	})
	runtime := newThinkingTranslationRuntime(mgr, "cs_1", "ct_1", &ThinkingTranslationSpec{Provider: "openai", Model: "translator"})
	runtime.add(context.Background(), "thinking:1", "Hello ")
	runtime.add(context.Background(), "thinking:1", "world.")
	if len(inputs) != 1 || inputs[0] != "Hello world." {
		t.Fatalf("unexpected translator inputs: %#v", inputs)
	}
}
