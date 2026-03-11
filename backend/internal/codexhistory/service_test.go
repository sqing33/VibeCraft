package codexhistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vibe-tree/backend/internal/store"

	_ "modernc.org/sqlite"
)

func TestParseReadableTitle_RecentConversationUsesCurrentUserInput(t *testing.T) {
	title := "Recent conversation:\nUSER: 旧问题\nCurrent user input: 我现在看到标题都是 Codex xxx，能不能解析到具体标题？"
	got := parseReadableTitle(title)
	if got != "我现在看到标题都是 Codex xxx，能不能解析到具体标题？" {
		t.Fatalf("parseReadableTitle = %q", got)
	}
}

func TestService_ImportThreadsEndToEnd(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sourceRoot := t.TempDir()
	rolloutPath := filepath.Join(sourceRoot, "sessions", "rollout-thread-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(rolloutPath), 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}
	writeRolloutFile(t, rolloutPath, []map[string]any{
		{
			"timestamp": "2026-03-11T05:03:27.476Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type": "task_started",
			},
		},
		{
			"timestamp": "2026-03-11T05:03:27.477Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":    "user_message",
				"message": "请帮我把 Codex 历史会话导入到当前项目。",
			},
		},
		{
			"timestamp": "2026-03-11T05:03:30.000Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":    "agent_message",
				"message": "我先检查 `~/.codex` 下的 state 数据结构。",
			},
		},
		{
			"timestamp": "2026-03-11T05:03:31.000Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type": "reasoning",
				"summary": []map[string]any{
					{"type": "summary_text", "text": "准备先读取线程标题和 rollout_path。"},
				},
			},
		},
		{
			"timestamp": "2026-03-11T05:03:32.000Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type":      "function_call",
				"name":      "exec_command",
				"call_id":   "call_1",
				"arguments": `{"cmd":"sqlite3 ~/.codex/state_5.sqlite '.schema threads'"}`,
			},
		},
		{
			"timestamp": "2026-03-11T05:03:33.000Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "Chunk ID: test\nProcess exited with code 0\nOutput:\nthreads schema\n",
			},
		},
		{
			"timestamp": "2026-03-11T05:03:34.000Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":               "task_complete",
				"last_agent_message": "已经确认可以读取历史，并准备导入选中的记录。",
			},
		},
	})

	stateDBPath := filepath.Join(sourceRoot, "state_5.sqlite")
	createSourceStateDB(t, stateDBPath, rolloutPath)

	svc := NewService(st, Options{SourceRoot: sourceRoot})

	threads, err := svc.ListThreads(ctx, 50)
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("threads len = %d, want 1", len(threads))
	}
	if threads[0].DisplayTitle != "导入 Codex 历史" {
		t.Fatalf("display title = %q", threads[0].DisplayTitle)
	}
	if threads[0].AlreadyImported {
		t.Fatalf("expected thread to be not imported yet")
	}

	imported, err := svc.ImportThreads(ctx, []string{"thread-1"})
	if err != nil {
		t.Fatalf("ImportThreads: %v", err)
	}
	if len(imported.Results) != 1 || !imported.Results[0].Imported {
		t.Fatalf("unexpected import result: %#v", imported.Results)
	}

	sessions, err := st.ListChatSessions(ctx, 10)
	if err != nil {
		t.Fatalf("ListChatSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(sessions))
	}
	if sessions[0].Title != "导入 Codex 历史" {
		t.Fatalf("session title = %q", sessions[0].Title)
	}
	if sessions[0].CLISessionID == nil || *sessions[0].CLISessionID != "thread-1" {
		t.Fatalf("cli_session_id = %#v", sessions[0].CLISessionID)
	}

	messages, err := st.ListChatMessages(ctx, sessions[0].ID, 20)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(messages))
	}
	if messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("unexpected message roles: %#v", messages)
	}
	if messages[0].Turn != messages[1].Turn {
		t.Fatalf("expected imported user/assistant to share turn, got %d and %d", messages[0].Turn, messages[1].Turn)
	}

	turns, err := st.ListChatTurns(ctx, sessions[0].ID, 20)
	if err != nil {
		t.Fatalf("ListChatTurns: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("turns len = %d, want 1", len(turns))
	}
	kinds := make([]string, 0, len(turns[0].Items))
	for _, item := range turns[0].Items {
		kinds = append(kinds, item.Kind)
	}
	joinedKinds := strings.Join(kinds, ",")
	for _, want := range []string{"progress", "thinking", "tool", "answer"} {
		if !strings.Contains(joinedKinds, want) {
			t.Fatalf("turn kinds = %q, want to contain %q", joinedKinds, want)
		}
	}

	reimported, err := svc.ImportThreads(ctx, []string{"thread-1"})
	if err != nil {
		t.Fatalf("reimport ImportThreads: %v", err)
	}
	if len(reimported.Results) != 1 || !reimported.Results[0].AlreadyImported {
		t.Fatalf("unexpected reimport result: %#v", reimported.Results)
	}
}

func TestService_FallsBackWhenLatestStateDBIsBroken(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sourceRoot := t.TempDir()
	rolloutPath := filepath.Join(sourceRoot, "sessions", "rollout-thread-2.jsonl")
	if err := os.MkdirAll(filepath.Dir(rolloutPath), 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}
	writeRolloutFile(t, rolloutPath, []map[string]any{
		{
			"timestamp": "2026-03-11T05:03:27.477Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":    "user_message",
				"message": "回退到旧 state 库继续导入。",
			},
		},
		{
			"timestamp": "2026-03-11T05:03:34.000Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":               "task_complete",
				"last_agent_message": "已经回退到可读 state 库。",
			},
		},
	})

	olderDBPath := filepath.Join(sourceRoot, "state_5.sqlite")
	createSourceStateDB(t, olderDBPath, rolloutPath)
	newerBrokenDBPath := filepath.Join(sourceRoot, "state_6.sqlite")
	if err := os.WriteFile(newerBrokenDBPath, []byte("not-a-valid-sqlite-db"), 0o644); err != nil {
		t.Fatalf("write broken db: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(olderDBPath, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatalf("chtimes older db: %v", err)
	}
	if err := os.Chtimes(newerBrokenDBPath, now, now); err != nil {
		t.Fatalf("chtimes newer broken db: %v", err)
	}

	svc := NewService(st, Options{SourceRoot: sourceRoot})

	threads, err := svc.ListThreads(ctx, 50)
	if err != nil {
		t.Fatalf("ListThreads with broken latest db: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("threads len = %d, want 1", len(threads))
	}
	if threads[0].ThreadID != "thread-1" {
		t.Fatalf("thread id = %q", threads[0].ThreadID)
	}

	imported, err := svc.ImportThreads(ctx, []string{"thread-1"})
	if err != nil {
		t.Fatalf("ImportThreads with broken latest db: %v", err)
	}
	if len(imported.Results) != 1 || !imported.Results[0].Imported {
		t.Fatalf("unexpected import result: %#v", imported.Results)
	}
}

func writeRolloutFile(t *testing.T, path string, lines []map[string]any) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	for _, line := range lines {
		if err := enc.Encode(line); err != nil {
			t.Fatalf("encode rollout line: %v", err)
		}
	}
}

func createSourceStateDB(t *testing.T, dbPath, rolloutPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=rwc")
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE threads (
			id TEXT PRIMARY KEY,
			rollout_path TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			source TEXT NOT NULL,
			model_provider TEXT NOT NULL,
			cwd TEXT NOT NULL,
			title TEXT NOT NULL,
			archived INTEGER NOT NULL DEFAULT 0,
			first_user_message TEXT NOT NULL DEFAULT ''
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec source stmt: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO threads (id, rollout_path, created_at, updated_at, source, model_provider, cwd, title, archived, first_user_message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?);`,
		"thread-1",
		rolloutPath,
		int64(1773205407),
		int64(1773205414),
		"cli",
		"custom",
		"/workspace/demo",
		"你是并行 worker w01。\n- task_title: 导入 Codex 历史\n- task_scope: 解析标题并导入会话",
		"请帮我把 Codex 历史会话导入到当前项目。",
	); err != nil {
		t.Fatalf("insert thread: %v", err)
	}
}
