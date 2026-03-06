package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const schemaVersion = 4

// Migrate 功能：执行 state DB schema 迁移（MVP：使用 PRAGMA user_version 管理版本）。
// 参数/返回：ctx 控制超时；成功返回 nil。
// 失败场景：读取 user_version 失败、DDL 执行失败或写回 user_version 失败时返回 error。
// 副作用：在 SQLite 中创建/更新表结构与索引。
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	return migrate(ctx, s.db)
}

func migrate(ctx context.Context, db *sql.DB) error {
	var userVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version;").Scan(&userVersion); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if userVersion >= schemaVersion {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback()

	if userVersion < 1 {
		if err := migrateV1(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 1;"); err != nil {
			return fmt.Errorf("set user_version=1: %w", err)
		}
	}
	if userVersion < 2 {
		if err := migrateV2(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 2;"); err != nil {
			return fmt.Errorf("set user_version=2: %w", err)
		}
	}
	if userVersion < 3 {
		if err := migrateV3(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 3;"); err != nil {
			return fmt.Errorf("set user_version=3: %w", err)
		}
	}
	if userVersion < 4 {
		if err := migrateV4(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 4;"); err != nil {
			return fmt.Errorf("set user_version=4: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func migrateV1(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS workflows (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  workspace_path TEXT NOT NULL,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  error_message TEXT,
  summary TEXT
);`,
		`
CREATE TABLE IF NOT EXISTS nodes (
  id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  node_type TEXT NOT NULL,
  expert_id TEXT NOT NULL,
  title TEXT NOT NULL,
  prompt TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  last_execution_id TEXT,
  result_summary TEXT,
  result_json TEXT,
  error_message TEXT,
  FOREIGN KEY(workflow_id) REFERENCES workflows(id)
);`,
		`
CREATE TABLE IF NOT EXISTS edges (
  id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  from_node_id TEXT NOT NULL,
  to_node_id TEXT NOT NULL,
  source_handle TEXT,
  target_handle TEXT,
  type TEXT NOT NULL,
  FOREIGN KEY(workflow_id) REFERENCES workflows(id)
);`,
		`
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
  error_message TEXT,
  FOREIGN KEY(node_id) REFERENCES nodes(id)
);`,
		`
CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  type TEXT NOT NULL,
  ts INTEGER NOT NULL,
  payload_json TEXT NOT NULL,
  FOREIGN KEY(workflow_id) REFERENCES workflows(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_events_workflow_ts ON events(workflow_id, ts);`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func migrateV2(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS chat_sessions (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  expert_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  workspace_path TEXT NOT NULL,
  status TEXT NOT NULL,
  summary TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  last_turn INTEGER NOT NULL DEFAULT 0
);`,
		`
CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  turn INTEGER NOT NULL,
  role TEXT NOT NULL,
  content_text TEXT NOT NULL,
  token_in INTEGER,
  token_out INTEGER,
  provider_message_id TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES chat_sessions(id)
);`,
		`
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_turn ON chat_messages(session_id, turn);`,
		`
CREATE TABLE IF NOT EXISTS chat_anchors (
  session_id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  previous_response_id TEXT,
  container_id TEXT,
  provider_message_id TEXT,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES chat_sessions(id)
);`,
		`
CREATE TABLE IF NOT EXISTS chat_compactions (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  from_turn INTEGER NOT NULL,
  to_turn INTEGER NOT NULL,
  before_tokens INTEGER NOT NULL,
  after_tokens INTEGER NOT NULL,
  summary_delta TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES chat_sessions(id)
);`,
		`
CREATE INDEX IF NOT EXISTS idx_chat_compactions_session_created ON chat_compactions(session_id, created_at);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func migrateV3(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE chat_messages ADD COLUMN expert_id TEXT;`,
		`ALTER TABLE chat_messages ADD COLUMN provider TEXT;`,
		`ALTER TABLE chat_messages ADD COLUMN model TEXT;`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			// Best-effort compatibility: tolerate manual pre-migrations.
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func migrateV4(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS chat_attachments (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  file_name TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  storage_rel_path TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES chat_sessions(id),
  FOREIGN KEY(message_id) REFERENCES chat_messages(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_attachments_message_id ON chat_attachments(message_id);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_attachments_session_created ON chat_attachments(session_id, created_at);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}
