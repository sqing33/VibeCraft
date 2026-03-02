package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/store"
)

func TestMigrateV2_ChatTablesAvailable(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "hello",
		ExpertID:      "codex",
		Provider:      "openai",
		Model:         "gpt-5-codex",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create chat session: %v", err)
	}
	if sess.ID == "" {
		t.Fatalf("missing session id")
	}
}

func TestChatStoreLifecycle(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "session-a",
		ExpertID:      "claudecode",
		Provider:      "anthropic",
		Model:         "claude-3-7-sonnet-latest",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create chat session: %v", err)
	}

	if _, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{SessionID: sess.ID, Role: "user", ContentText: "hi"}); err != nil {
		t.Fatalf("append user message: %v", err)
	}
	if _, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{SessionID: sess.ID, Role: "assistant", ContentText: "hello"}); err != nil {
		t.Fatalf("append assistant message: %v", err)
	}

	msgs, err := st.ListChatMessages(context.Background(), sess.ID, 50)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected message order: %+v", msgs)
	}
	if msgs[0].Turn >= msgs[1].Turn {
		t.Fatalf("expected ascending turn order")
	}

	anchorID := "resp_123"
	if err := st.UpsertChatAnchor(context.Background(), store.UpsertChatAnchorParams{
		SessionID:        sess.ID,
		Provider:         sess.Provider,
		PreviousResponse: &anchorID,
	}); err != nil {
		t.Fatalf("upsert anchor: %v", err)
	}
	anchor, err := st.GetChatAnchor(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get anchor: %v", err)
	}
	if anchor.PreviousResponse == nil || *anchor.PreviousResponse != anchorID {
		t.Fatalf("unexpected anchor: %+v", anchor)
	}

	updated, err := st.UpdateChatSummary(context.Background(), sess.ID, "summary")
	if err != nil {
		t.Fatalf("update summary: %v", err)
	}
	if updated.Summary == nil || *updated.Summary != "summary" {
		t.Fatalf("summary not updated: %+v", updated)
	}

	forked, err := st.ForkChatSession(context.Background(), sess.ID, "")
	if err != nil {
		t.Fatalf("fork session: %v", err)
	}
	if forked.ID == sess.ID {
		t.Fatalf("forked session should have new id")
	}
	if forked.Summary == nil || *forked.Summary != "summary" {
		t.Fatalf("forked summary mismatch: %+v", forked)
	}

	comp, err := st.CreateChatCompaction(context.Background(), store.CreateChatCompactionParams{
		SessionID:    sess.ID,
		FromTurn:     msgs[0].Turn,
		ToTurn:       msgs[1].Turn,
		BeforeTokens: 100,
		AfterTokens:  20,
		SummaryDelta: "delta",
	})
	if err != nil {
		t.Fatalf("create compaction: %v", err)
	}
	if comp.ID == "" {
		t.Fatalf("missing compaction id")
	}
}
