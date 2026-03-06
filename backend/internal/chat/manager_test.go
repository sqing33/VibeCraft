package chat_test

import (
	"context"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

func TestCompactionLLMFallback_DemoProviderDoesNotFail(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "demo",
		ExpertID:      "demo",
		Provider:      "demo",
		Model:         "demo",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Create enough history to compact.
	for i := 0; i < 6; i++ {
		_, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
			SessionID:   sess.ID,
			Role:        "user",
			ContentText: "hello",
			ExpertID:    pointer("demo"),
			Provider:    pointer("demo"),
			Model:       pointer("demo"),
		})
		if err != nil {
			t.Fatalf("append message: %v", err)
		}
	}

	mgr := chat.NewManager(st, nil, chat.Options{KeepRecent: 2})
	updated, rec, err := mgr.CompactSession(context.Background(), sess.ID, runner.SDKSpec{Provider: "demo", Model: "demo"}, nil)
	if err != nil {
		t.Fatalf("compact session: %v", err)
	}
	if rec == nil || rec.ID == "" {
		t.Fatalf("expected compaction record")
	}
	if updated.Summary == nil || *updated.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}
}

func TestRunTurn_CompactionFallbackDoesNotFailTurn(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "demo",
		ExpertID:      "demo",
		Provider:      "demo",
		Model:         "demo",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Pre-seed enough messages so keepRecent=2 will compact during the turn.
	for i := 0; i < 3; i++ {
		_, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
			SessionID:   sess.ID,
			Role:        "assistant",
			ContentText: "seed",
			ExpertID:    pointer("demo"),
			Provider:    pointer("demo"),
			Model:       pointer("demo"),
		})
		if err != nil {
			t.Fatalf("append seed message: %v", err)
		}
	}

	mgr := chat.NewManager(st, nil, chat.Options{
		KeepRecent:    2,
		SoftRatio:     0.01,
		ForceRatio:    0.02,
		HardRatio:     0.99,
		ContextWindow: 64,
	})

	_, err = mgr.RunTurn(context.Background(), chat.TurnParams{
		Session:    sess,
		ExpertID:   "demo",
		UserInput:  "hello world",
		ModelInput: "hello world",
		SDK: runner.SDKSpec{
			Provider: "demo",
			Model:    "demo",
		},
		Env: nil,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}

	updated, err := st.GetChatSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Summary == nil || *updated.Summary == "" {
		t.Fatalf("expected summary to be updated by compaction")
	}
}

func pointer(s string) *string { return &s }

func TestRunTurn_AttachmentsSkipAutomaticCompaction(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "demo",
		ExpertID:      "demo",
		Provider:      "demo",
		Model:         "demo",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
			SessionID:   sess.ID,
			Role:        "assistant",
			ContentText: "seed",
			ExpertID:    pointer("demo"),
			Provider:    pointer("demo"),
			Model:       pointer("demo"),
		})
		if err != nil {
			t.Fatalf("append seed message: %v", err)
		}
	}

	mgr := chat.NewManager(st, nil, chat.Options{
		KeepRecent:    2,
		SoftRatio:     0.01,
		ForceRatio:    0.02,
		HardRatio:     0.99,
		ContextWindow: 64,
	})

	result, err := mgr.RunTurn(context.Background(), chat.TurnParams{
		Session:    sess,
		ExpertID:   "demo",
		UserInput:  "hello with file",
		ModelInput: "hello with file",
		Attachments: []chat.UploadedAttachment{{
			FileName: "note.txt",
			MIMEType: "text/plain",
			Bytes:    []byte("hello attachment"),
		}},
		SDK: runner.SDKSpec{
			Provider: "demo",
			Model:    "demo",
		},
		Env: nil,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if len(result.UserMessage.Attachments) != 1 {
		t.Fatalf("expected user attachment metadata, got %+v", result.UserMessage)
	}

	updated, err := st.GetChatSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.Summary != nil && *updated.Summary != "" {
		t.Fatalf("expected summary to remain empty when attachments are present, got %q", *updated.Summary)
	}
}

func TestRunTurn_ModelFallbackUsesSecondarySpec(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "fallback",
		ExpertID:      "ui-expert",
		Provider:      "demo",
		Model:         "demo",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	mgr := chat.NewManager(st, nil, chat.Options{KeepRecent: 4})
	result, err := mgr.RunTurn(context.Background(), chat.TurnParams{
		Session:    sess,
		ExpertID:   "ui-expert",
		UserInput:  "hello world",
		ModelInput: "hello world",
		SDK: runner.SDKSpec{
			Provider: "broken-provider",
			Model:    "broken-model",
		},
		Fallbacks: []runner.SDKFallback{{
			SDK: runner.SDKSpec{Provider: "demo", Model: "demo"},
		}},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if result.AssistantMessage.Provider == nil || *result.AssistantMessage.Provider != "demo" {
		t.Fatalf("expected fallback provider demo, got: %+v", result.AssistantMessage.Provider)
	}
	if result.AssistantMessage.Model == nil || *result.AssistantMessage.Model != "demo" {
		t.Fatalf("expected fallback model demo, got: %+v", result.AssistantMessage.Model)
	}
}
