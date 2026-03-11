package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/codexhistory"
	"vibe-tree/backend/internal/server"
	"vibe-tree/backend/internal/store"

	_ "modernc.org/sqlite"
)

func TestCodexHistoryAPI_ListAndImport(t *testing.T) {
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
	rolloutPath := filepath.Join(sourceRoot, "sessions", "rollout-thread-api.jsonl")
	if err := os.MkdirAll(filepath.Dir(rolloutPath), 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}
	rolloutLines := []map[string]any{
		{
			"timestamp": "2026-03-11T08:00:00.000Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":    "user_message",
				"message": "我想把 Codex 历史导入到当前项目。",
			},
		},
		{
			"timestamp": "2026-03-11T08:00:01.000Z",
			"type":      "event_msg",
			"payload": map[string]any{
				"type":               "task_complete",
				"last_agent_message": "已经导入完成。",
			},
		},
	}
	file, err := os.Create(rolloutPath)
	if err != nil {
		t.Fatalf("create rollout: %v", err)
	}
	enc := json.NewEncoder(file)
	for _, line := range rolloutLines {
		if err := enc.Encode(line); err != nil {
			t.Fatalf("encode rollout line: %v", err)
		}
	}
	_ = file.Close()

	sourceDBPath := filepath.Join(sourceRoot, "state_5.sqlite")
	db, err := sql.Open("sqlite", "file:"+sourceDBPath+"?mode=rwc")
	if err != nil {
		t.Fatalf("open source db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE threads (
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
	);`); err != nil {
		t.Fatalf("create threads table: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO threads (id, rollout_path, created_at, updated_at, source, model_provider, cwd, title, archived, first_user_message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?);`,
		"thread-api",
		rolloutPath,
		int64(1773216000),
		int64(1773216001),
		"cli",
		"custom",
		"/workspace/api",
		"我想给当前项目导入 codex cli 的历史对话记录",
		"我想把 Codex 历史导入到当前项目。",
	); err != nil {
		t.Fatalf("insert source thread: %v", err)
	}
	_ = db.Close()

	svc := codexhistory.NewService(st, codexhistory.Options{SourceRoot: sourceRoot})
	engine := server.New(server.Options{DevCORS: false}, api.Deps{Store: st, CodexHistory: svc})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	listRes, err := http.Get(httpSrv.URL + "/api/v1/codex-history/threads")
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %s", listRes.Status)
	}
	var threads []codexhistory.ThreadSummary
	if err := json.NewDecoder(listRes.Body).Decode(&threads); err != nil {
		t.Fatalf("decode threads: %v", err)
	}
	if len(threads) != 1 || threads[0].DisplayTitle == "" {
		t.Fatalf("unexpected threads: %#v", threads)
	}

	body, _ := json.Marshal(map[string]any{"thread_ids": []string{"thread-api"}})
	importRes, err := http.Post(httpSrv.URL+"/api/v1/codex-history/import", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("import threads: %v", err)
	}
	defer importRes.Body.Close()
	if importRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected import status: %s", importRes.Status)
	}
	var imported codexhistory.ImportResponse
	if err := json.NewDecoder(importRes.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if len(imported.Results) != 1 || !imported.Results[0].Imported {
		t.Fatalf("unexpected import response: %#v", imported)
	}
}
