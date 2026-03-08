package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/store"
)

func TestMigrateV3_ChatTablesAvailable(t *testing.T) {
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

	expertID := "claudecode"
	provider := "anthropic"
	model := "claude-3-7-sonnet-latest"

	if _, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
		SessionID:   sess.ID,
		Role:        "user",
		ContentText: "hi",
		ExpertID:    &expertID,
		Provider:    &provider,
		Model:       &model,
	}); err != nil {
		t.Fatalf("append user message: %v", err)
	}
	if _, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
		SessionID:   sess.ID,
		Role:        "assistant",
		ContentText: "hello",
		ExpertID:    &expertID,
		Provider:    &provider,
		Model:       &model,
	}); err != nil {
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
	for _, msg := range msgs {
		if msg.ExpertID == nil || *msg.ExpertID != expertID {
			t.Fatalf("unexpected expert_id: %+v", msg)
		}
		if msg.Provider == nil || *msg.Provider != provider {
			t.Fatalf("unexpected provider: %+v", msg)
		}
		if msg.Model == nil || *msg.Model != model {
			t.Fatalf("unexpected model: %+v", msg)
		}
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
	forkMessages, err := st.ListChatMessages(context.Background(), forked.ID, 50)
	if err != nil {
		t.Fatalf("list fork messages: %v", err)
	}
	if len(forkMessages) != len(msgs) {
		t.Fatalf("expected %d fork context messages, got %d", len(msgs), len(forkMessages))
	}
	for i := range forkMessages {
		if forkMessages[i].Role != msgs[i].Role {
			t.Fatalf("fork role mismatch at %d: got %q want %q", i, forkMessages[i].Role, msgs[i].Role)
		}
		if forkMessages[i].ContentText != msgs[i].ContentText {
			t.Fatalf("fork content mismatch at %d", i)
		}
		if forkMessages[i].Provider == nil || *forkMessages[i].Provider != store.ForkContextProvider {
			t.Fatalf("fork context provider missing at %d: %+v", i, forkMessages[i])
		}
		if forkMessages[i].ExpertID != nil || forkMessages[i].Model != nil {
			t.Fatalf("fork context message should not carry expert/model at %d: %+v", i, forkMessages[i])
		}
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

func TestChatStore_ListMessagesHydratesAttachments(t *testing.T) {
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
		Title:         "with-attachments",
		ExpertID:      "demo",
		Provider:      "demo",
		Model:         "demo",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create chat session: %v", err)
	}

	expertID := "demo"
	provider := "demo"
	model := "demo"
	msg, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
		SessionID:   sess.ID,
		Role:        "user",
		ContentText: "see file",
		ExpertID:    &expertID,
		Provider:    &provider,
		Model:       &model,
	})
	if err != nil {
		t.Fatalf("append user message: %v", err)
	}

	err = st.CreateChatAttachments(context.Background(), store.CreateChatAttachmentsParams{Attachments: []store.ChatAttachment{
		{
			ID:             "ca_test_1",
			SessionID:      sess.ID,
			MessageID:      msg.ID,
			Kind:           store.ChatAttachmentKindText,
			FileName:       "note.txt",
			MIMEType:       "text/plain",
			SizeBytes:      12,
			StorageRelPath: "chat-attachments/test/note.txt",
			CreatedAt:      msg.CreatedAt,
		},
	}})
	if err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	msgs, err := st.ListChatMessages(context.Background(), sess.ID, 20)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Attachments) != 1 {
		t.Fatalf("expected hydrated attachment, got %+v", msgs[0])
	}
	if msgs[0].Attachments[0].FileName != "note.txt" {
		t.Fatalf("unexpected attachment metadata: %+v", msgs[0].Attachments[0])
	}
}

func TestMigrate_RepairsMalformedChatAttachmentsSchema(t *testing.T) {
	t.Parallel()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if _, err := st.DB().ExecContext(context.Background(), `PRAGMA user_version = 6;`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := st.DB().ExecContext(context.Background(), `
CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  turn INTEGER NOT NULL,
  role TEXT NOT NULL,
  content_text TEXT NOT NULL,
  created_at INTEGER NOT NULL
);`); err != nil {
		t.Fatalf("create chat_messages: %v", err)
	}
	if _, err := st.DB().ExecContext(context.Background(), `
CREATE TABLE IF NOT EXISTS executions (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL,
  attempt INTEGER NOT NULL,
  pid INTEGER,
  exit_code INTEGER,
  status TEXT NOT NULL,
  log_path TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER,
  error_message TEXT
);`); err != nil {
		t.Fatalf("create executions: %v", err)
	}
	if _, err := st.DB().ExecContext(context.Background(), `
CREATE TABLE chat_attachments (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  file_name TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  storage_rel_path TEXT NOT NULL,
  created_at INTEGER NOT NULL
);`); err != nil {
		t.Fatalf("create malformed chat_attachments: %v", err)
	}

	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	rows, err := st.DB().QueryContext(context.Background(), `PRAGMA table_info(chat_attachments);`)
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		cols[name] = true
	}
	for _, name := range []string{"session_id", "message_id", "kind", "file_name", "mime_type", "size_bytes", "storage_rel_path", "created_at"} {
		if !cols[name] {
			t.Fatalf("expected repaired column %q in chat_attachments, got %+v", name, cols)
		}
	}
}

func TestChatTurnStoreLifecycle(t *testing.T) {
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
		Title:         "timeline",
		ExpertID:      "codex",
		Provider:      "cli",
		Model:         "gpt-5-codex",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	userMsg, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
		SessionID:   sess.ID,
		Role:        "user",
		ContentText: "hello timeline",
	})
	if err != nil {
		t.Fatalf("append user message: %v", err)
	}

	turn, err := st.StartChatTurn(context.Background(), store.CreateChatTurnParams{
		SessionID:     sess.ID,
		UserMessageID: userMsg.ID,
		Turn:          userMsg.Turn,
		ExpertID:      strPtr("codex"),
		Provider:      strPtr("cli"),
		Model:         strPtr("gpt-5-codex"),
		ModelInput:    strPtr("hello timeline"),
	})
	if err != nil {
		t.Fatalf("start chat turn: %v", err)
	}
	if _, err := st.UpsertChatTurnItem(context.Background(), store.UpsertChatTurnItemParams{
		TurnID:  turn.ID,
		EntryID: "thinking:1",
		Seq:     1,
		Kind:    "thinking",
		Status:  "streaming",
		Op:      "append",
		Delta:   "first ",
	}); err != nil {
		t.Fatalf("append thinking delta: %v", err)
	}
	if _, err := st.UpsertChatTurnItem(context.Background(), store.UpsertChatTurnItemParams{
		TurnID:  turn.ID,
		EntryID: "thinking:1",
		Seq:     99,
		Kind:    "thinking",
		Status:  "done",
		Op:      "replace",
		Delta:   "summary",
	}); err != nil {
		t.Fatalf("replace thinking content: %v", err)
	}
	if _, err := st.AppendChatTurnItemTranslatedContent(context.Background(), store.AppendChatTurnItemTranslatedContentParams{
		TurnID:  turn.ID,
		EntryID: "thinking:1",
		Delta:   "摘要",
		Replace: true,
	}); err != nil {
		t.Fatalf("replace translated thinking: %v", err)
	}
	assistantMsg, err := st.AppendChatMessage(context.Background(), store.AppendChatMessageParams{
		SessionID:   sess.ID,
		Role:        "assistant",
		ContentText: "done",
	})
	if err != nil {
		t.Fatalf("append assistant message: %v", err)
	}
	if _, err := st.CompleteChatTurn(context.Background(), store.CompleteChatTurnParams{
		TurnID:             turn.ID,
		SessionID:          sess.ID,
		UserMessageID:      userMsg.ID,
		AssistantMessageID: assistantMsg.ID,
		ContextMode:        strPtr("cli_reconstructed"),
	}); err != nil {
		t.Fatalf("complete chat turn: %v", err)
	}

	turns, err := st.ListChatTurns(context.Background(), sess.ID, 20)
	if err != nil {
		t.Fatalf("list chat turns: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].AssistantMessageID == nil || *turns[0].AssistantMessageID != assistantMsg.ID {
		t.Fatalf("unexpected assistant linkage: %+v", turns[0])
	}
	if turns[0].Status != "completed" {
		t.Fatalf("expected completed turn, got %+v", turns[0])
	}
	if len(turns[0].Items) != 1 {
		t.Fatalf("expected 1 persisted item, got %+v", turns[0].Items)
	}
	if turns[0].Items[0].Seq != 1 {
		t.Fatalf("expected stable seq=1 after update, got %+v", turns[0].Items[0])
	}
	if turns[0].Items[0].ContentText != "summary" {
		t.Fatalf("expected summary content, got %+v", turns[0].Items[0])
	}
	if got, _ := turns[0].Items[0].Meta["translated_content"].(string); got != "摘要" {
		t.Fatalf("expected translated thinking, got %+v", turns[0].Items[0].Meta)
	}
}

func strPtr(s string) *string { return &s }

func pointer(value string) *string {
	return &value
}
