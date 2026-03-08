package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const schemaVersion = 10

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
	if userVersion < 5 {
		if err := migrateV5(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 5;"); err != nil {
			return fmt.Errorf("set user_version=5: %w", err)
		}
	}
	if userVersion < 6 {
		if err := migrateV6(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 6;"); err != nil {
			return fmt.Errorf("set user_version=6: %w", err)
		}
	}
	if userVersion < 7 {
		if err := migrateV7(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 7;"); err != nil {
			return fmt.Errorf("set user_version=7: %w", err)
		}
	}
	if userVersion < 8 {
		if err := migrateV8(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 8;"); err != nil {
			return fmt.Errorf("set user_version=8: %w", err)
		}
	}
	if userVersion < 9 {
		if err := migrateV9(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 9;"); err != nil {
			return fmt.Errorf("set user_version=9: %w", err)
		}
	}
	if userVersion < 10 {
		if err := migrateV10(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 10;"); err != nil {
			return fmt.Errorf("set user_version=10: %w", err)
		}
	}
	if userVersion < 10 {
		if err := migrateV10(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 10;"); err != nil {
			return fmt.Errorf("set user_version=10: %w", err)
		}
	}

	if userVersion < schemaVersion {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration: %w", err)
		}
	} else {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			return fmt.Errorf("rollback noop migration tx: %w", err)
		}
	}
	if err := reconcileCompatibility(ctx, db); err != nil {
		return err
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
  cli_tool_id TEXT,
  model_id TEXT,
  cli_session_id TEXT,
  mcp_server_ids_json TEXT,
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

func migrateV5(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS expert_builder_sessions (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  target_expert_id TEXT,
  builder_model_id TEXT NOT NULL,
  status TEXT NOT NULL,
  latest_snapshot_id TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_sessions_updated_at ON expert_builder_sessions(updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_sessions_target_expert ON expert_builder_sessions(target_expert_id, updated_at DESC);`,
		`
CREATE TABLE IF NOT EXISTS expert_builder_messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content_text TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES expert_builder_sessions(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_messages_session_created ON expert_builder_messages(session_id, created_at);`,
		`
CREATE TABLE IF NOT EXISTS expert_builder_snapshots (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  assistant_message TEXT NOT NULL,
  draft_json TEXT NOT NULL,
  raw_json TEXT,
  warnings_json TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES expert_builder_sessions(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_snapshots_session_version ON expert_builder_snapshots(session_id, version DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func migrateV6(ctx context.Context, tx *sql.Tx) error {
	alterStmts := []string{
		`ALTER TABLE executions ADD COLUMN orchestration_id TEXT;`,
		`ALTER TABLE executions ADD COLUMN round_id TEXT;`,
		`ALTER TABLE executions ADD COLUMN agent_run_id TEXT;`,
	}
	for _, stmt := range alterStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}

	createStmts := []string{
		`
CREATE TABLE IF NOT EXISTS orchestrations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  goal TEXT NOT NULL,
  workspace_path TEXT NOT NULL,
  status TEXT NOT NULL,
  current_round INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  error_message TEXT,
  summary TEXT
);`,
		`
CREATE TABLE IF NOT EXISTS orchestration_rounds (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_index INTEGER NOT NULL,
  goal TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  summary TEXT,
  synthesis_step_id TEXT,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id)
);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_orchestration_rounds_unique ON orchestration_rounds(orchestration_id, round_index);`,
		`
CREATE TABLE IF NOT EXISTS agent_runs (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_id TEXT NOT NULL,
  role TEXT NOT NULL,
  title TEXT NOT NULL,
  goal TEXT NOT NULL,
  expert_id TEXT NOT NULL,
  intent TEXT NOT NULL,
  workspace_mode TEXT NOT NULL,
  workspace_path TEXT NOT NULL,
  branch_name TEXT,
  base_ref TEXT,
  worktree_path TEXT,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  last_execution_id TEXT,
  result_summary TEXT,
  error_message TEXT,
  modified_code INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id),
  FOREIGN KEY(round_id) REFERENCES orchestration_rounds(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_runs_orchestration_round ON agent_runs(orchestration_id, round_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status, created_at);`,
		`
CREATE TABLE IF NOT EXISTS synthesis_steps (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_id TEXT NOT NULL,
  decision TEXT NOT NULL,
  summary TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id),
  FOREIGN KEY(round_id) REFERENCES orchestration_rounds(id)
);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_synthesis_steps_round_unique ON synthesis_steps(round_id);`,
		`
CREATE TABLE IF NOT EXISTS orchestration_artifacts (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_id TEXT,
  agent_run_id TEXT,
  synthesis_step_id TEXT,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT,
  payload_json TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id),
  FOREIGN KEY(round_id) REFERENCES orchestration_rounds(id),
  FOREIGN KEY(agent_run_id) REFERENCES agent_runs(id),
  FOREIGN KEY(synthesis_step_id) REFERENCES synthesis_steps(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_orchestration_artifacts_lookup ON orchestration_artifacts(orchestration_id, round_id, agent_run_id, created_at);`,
		`
CREATE TABLE IF NOT EXISTS orchestration_events (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  type TEXT NOT NULL,
  ts INTEGER NOT NULL,
  payload_json TEXT NOT NULL,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_orchestration_events_ts ON orchestration_events(orchestration_id, ts);`,
		`
CREATE TABLE IF NOT EXISTS agent_run_executions (
  id TEXT PRIMARY KEY,
  agent_run_id TEXT NOT NULL,
  attempt INTEGER NOT NULL,
  pid INTEGER,
  exit_code INTEGER,
  status TEXT NOT NULL,
  log_path TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER,
  signal TEXT,
  error_message TEXT,
  FOREIGN KEY(agent_run_id) REFERENCES agent_runs(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_run_executions_agent_run ON agent_run_executions(agent_run_id, attempt);`,
	}
	for _, stmt := range createStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func migrateV7(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS repo_sources (
  id TEXT PRIMARY KEY,
  repo_url TEXT NOT NULL UNIQUE,
  owner TEXT NOT NULL,
  repo TEXT NOT NULL,
  repo_key TEXT NOT NULL UNIQUE,
  default_branch TEXT,
  visibility TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_sources_updated_at ON repo_sources(updated_at DESC);`,
		`
CREATE TABLE IF NOT EXISTS repo_snapshots (
  id TEXT PRIMARY KEY,
  repo_source_id TEXT NOT NULL,
  requested_ref TEXT NOT NULL,
  resolved_ref TEXT,
  commit_sha TEXT,
  storage_path TEXT NOT NULL,
  report_path TEXT,
  subagent_results_path TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(repo_source_id) REFERENCES repo_sources(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_snapshots_source_created ON repo_snapshots(repo_source_id, created_at DESC);`,
		`
CREATE TABLE IF NOT EXISTS repo_analysis_runs (
  id TEXT PRIMARY KEY,
  repo_source_id TEXT NOT NULL,
  repo_snapshot_id TEXT NOT NULL,
  execution_id TEXT,
  status TEXT NOT NULL,
  language TEXT,
  depth TEXT,
  agent_mode TEXT,
  features_json TEXT NOT NULL,
  summary TEXT,
  error_message TEXT,
  result_json TEXT,
  report_path TEXT,
  started_at INTEGER,
  ended_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(repo_source_id) REFERENCES repo_sources(id),
  FOREIGN KEY(repo_snapshot_id) REFERENCES repo_snapshots(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_analysis_runs_source_created ON repo_analysis_runs(repo_source_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_analysis_runs_snapshot_created ON repo_analysis_runs(repo_snapshot_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_analysis_runs_status ON repo_analysis_runs(status, created_at DESC);`,
		`
CREATE TABLE IF NOT EXISTS repo_knowledge_cards (
  id TEXT PRIMARY KEY,
  repo_source_id TEXT NOT NULL,
  repo_snapshot_id TEXT NOT NULL,
  analysis_run_id TEXT NOT NULL,
  title TEXT NOT NULL,
  card_type TEXT NOT NULL,
  summary TEXT NOT NULL,
  mechanism TEXT,
  confidence TEXT,
  tags_json TEXT,
  section_title TEXT,
  sort_index INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(repo_source_id) REFERENCES repo_sources(id),
  FOREIGN KEY(repo_snapshot_id) REFERENCES repo_snapshots(id),
  FOREIGN KEY(analysis_run_id) REFERENCES repo_analysis_runs(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_knowledge_cards_snapshot ON repo_knowledge_cards(repo_snapshot_id, sort_index, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_knowledge_cards_analysis ON repo_knowledge_cards(analysis_run_id, sort_index, created_at);`,
		`
CREATE TABLE IF NOT EXISTS repo_knowledge_evidence (
  id TEXT PRIMARY KEY,
  card_id TEXT NOT NULL,
  path TEXT NOT NULL,
  line INTEGER,
  snippet TEXT,
  dimension TEXT,
  sort_index INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(card_id) REFERENCES repo_knowledge_cards(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_knowledge_evidence_card ON repo_knowledge_evidence(card_id, sort_index, created_at);`,
		`
CREATE TABLE IF NOT EXISTS repo_similarity_queries (
  id TEXT PRIMARY KEY,
  query_text TEXT NOT NULL,
  repo_filters_json TEXT,
  mode TEXT NOT NULL,
  top_k INTEGER NOT NULL,
  result_json TEXT,
  created_at INTEGER NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_similarity_queries_created ON repo_similarity_queries(created_at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func reconcileCompatibility(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin compatibility tx: %w", err)
	}
	defer tx.Rollback()
	if err := ensureChatMessageMetadataColumns(ctx, tx); err != nil {
		return err
	}
	if err := ensureChatAttachmentsSchema(ctx, tx); err != nil {
		return err
	}
	if err := ensureExpertBuilderSchema(ctx, tx); err != nil {
		return err
	}
	if err := ensureOrchestrationSchema(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit compatibility tx: %w", err)
	}
	return nil
}

func ensureChatMessageMetadataColumns(ctx context.Context, tx *sql.Tx) error {
	for _, stmt := range []string{
		`ALTER TABLE chat_messages ADD COLUMN expert_id TEXT;`,
		`ALTER TABLE chat_messages ADD COLUMN provider TEXT;`,
		`ALTER TABLE chat_messages ADD COLUMN model TEXT;`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("ensure chat_messages metadata columns: %w", err)
		}
	}
	return nil
}

func ensureChatAttachmentsSchema(ctx context.Context, tx *sql.Tx) error {
	columns, err := tableColumns(ctx, tx, "chat_attachments")
	if err != nil {
		return err
	}
	expected := []string{"id", "session_id", "message_id", "kind", "file_name", "mime_type", "size_bytes", "storage_rel_path", "created_at"}
	if len(columns) == 0 {
		return createChatAttachmentsTable(ctx, tx)
	}
	for _, col := range expected {
		if !columns[col] {
			if _, err := tx.ExecContext(ctx, `ALTER TABLE chat_attachments RENAME TO chat_attachments_legacy_broken;`); err != nil {
				return fmt.Errorf("rename malformed chat_attachments: %w", err)
			}
			return createChatAttachmentsTable(ctx, tx)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_chat_attachments_message_id ON chat_attachments(message_id);`); err != nil {
		return fmt.Errorf("ensure chat_attachments message index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_chat_attachments_session_created ON chat_attachments(session_id, created_at);`); err != nil {
		return fmt.Errorf("ensure chat_attachments session index: %w", err)
	}
	return nil
}

func createChatAttachmentsTable(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS chat_attachments (
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
			return fmt.Errorf("create chat_attachments compatibility table: %w", err)
		}
	}
	return nil
}

func ensureExpertBuilderSchema(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS expert_builder_sessions (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  target_expert_id TEXT,
  builder_model_id TEXT NOT NULL,
  status TEXT NOT NULL,
  latest_snapshot_id TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_sessions_updated_at ON expert_builder_sessions(updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_sessions_target_expert ON expert_builder_sessions(target_expert_id, updated_at DESC);`,
		`CREATE TABLE IF NOT EXISTS expert_builder_messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content_text TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES expert_builder_sessions(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_messages_session_created ON expert_builder_messages(session_id, created_at);`,
		`CREATE TABLE IF NOT EXISTS expert_builder_snapshots (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  assistant_message TEXT NOT NULL,
  draft_json TEXT NOT NULL,
  raw_json TEXT,
  warnings_json TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES expert_builder_sessions(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_expert_builder_snapshots_session_version ON expert_builder_snapshots(session_id, version DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure expert builder schema: %w", err)
		}
	}
	return nil
}

func ensureOrchestrationSchema(ctx context.Context, tx *sql.Tx) error {
	for _, stmt := range []string{
		`ALTER TABLE executions ADD COLUMN orchestration_id TEXT;`,
		`ALTER TABLE executions ADD COLUMN round_id TEXT;`,
		`ALTER TABLE executions ADD COLUMN agent_run_id TEXT;`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("ensure orchestration execution columns: %w", err)
		}
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS orchestrations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  goal TEXT NOT NULL,
  workspace_path TEXT NOT NULL,
  status TEXT NOT NULL,
  current_round INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  error_message TEXT,
  summary TEXT
);`,
		`CREATE TABLE IF NOT EXISTS orchestration_rounds (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_index INTEGER NOT NULL,
  goal TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  summary TEXT,
  synthesis_step_id TEXT,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id)
);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_orchestration_rounds_unique ON orchestration_rounds(orchestration_id, round_index);`,
		`CREATE TABLE IF NOT EXISTS agent_runs (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_id TEXT NOT NULL,
  role TEXT NOT NULL,
  title TEXT NOT NULL,
  goal TEXT NOT NULL,
  expert_id TEXT NOT NULL,
  intent TEXT NOT NULL,
  workspace_mode TEXT NOT NULL,
  workspace_path TEXT NOT NULL,
  branch_name TEXT,
  base_ref TEXT,
  worktree_path TEXT,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  last_execution_id TEXT,
  result_summary TEXT,
  error_message TEXT,
  modified_code INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id),
  FOREIGN KEY(round_id) REFERENCES orchestration_rounds(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_runs_orchestration_round ON agent_runs(orchestration_id, round_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status, created_at);`,
		`CREATE TABLE IF NOT EXISTS synthesis_steps (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_id TEXT NOT NULL,
  decision TEXT NOT NULL,
  summary TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id),
  FOREIGN KEY(round_id) REFERENCES orchestration_rounds(id)
);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_synthesis_steps_round_unique ON synthesis_steps(round_id);`,
		`CREATE TABLE IF NOT EXISTS orchestration_artifacts (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  round_id TEXT,
  agent_run_id TEXT,
  synthesis_step_id TEXT,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT,
  payload_json TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id),
  FOREIGN KEY(round_id) REFERENCES orchestration_rounds(id),
  FOREIGN KEY(agent_run_id) REFERENCES agent_runs(id),
  FOREIGN KEY(synthesis_step_id) REFERENCES synthesis_steps(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_orchestration_artifacts_lookup ON orchestration_artifacts(orchestration_id, round_id, agent_run_id, created_at);`,
		`CREATE TABLE IF NOT EXISTS orchestration_events (
  id TEXT PRIMARY KEY,
  orchestration_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  type TEXT NOT NULL,
  ts INTEGER NOT NULL,
  payload_json TEXT NOT NULL,
  FOREIGN KEY(orchestration_id) REFERENCES orchestrations(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_orchestration_events_ts ON orchestration_events(orchestration_id, ts);`,
		`CREATE TABLE IF NOT EXISTS agent_run_executions (
  id TEXT PRIMARY KEY,
  agent_run_id TEXT NOT NULL,
  attempt INTEGER NOT NULL,
  pid INTEGER,
  exit_code INTEGER,
  status TEXT NOT NULL,
  log_path TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER,
  signal TEXT,
  error_message TEXT,
  FOREIGN KEY(agent_run_id) REFERENCES agent_runs(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_run_executions_agent_run ON agent_run_executions(agent_run_id, attempt);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure orchestration schema: %w", err)
		}
	}
	return nil
}

func tableExists(ctx context.Context, tx *sql.Tx, table string) (bool, error) {
	row := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?;`, table)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("query sqlite_master for %s: %w", table, err)
	}
	return count > 0, nil
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return nil, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		cols[strings.TrimSpace(strings.ToLower(name))] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table_info(%s): %w", table, err)
	}
	return cols, nil
}

func migrateV8(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExists(ctx, tx, "chat_sessions")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	cols, err := tableColumns(ctx, tx, "chat_sessions")
	if err != nil {
		return err
	}
	if !cols["cli_tool_id"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE chat_sessions ADD COLUMN cli_tool_id TEXT;`); err != nil {
			return fmt.Errorf("add chat_sessions.cli_tool_id: %w", err)
		}
	}
	if !cols["model_id"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE chat_sessions ADD COLUMN model_id TEXT;`); err != nil {
			return fmt.Errorf("add chat_sessions.model_id: %w", err)
		}
	}
	if !cols["cli_session_id"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE chat_sessions ADD COLUMN cli_session_id TEXT;`); err != nil {
			return fmt.Errorf("add chat_sessions.cli_session_id: %w", err)
		}
	}
	if !cols["mcp_server_ids_json"] {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE chat_sessions ADD COLUMN mcp_server_ids_json TEXT;`); err != nil {
			return fmt.Errorf("add chat_sessions.mcp_server_ids_json: %w", err)
		}
	}
	return nil
}

func migrateV9(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExists(ctx, tx, "repo_analysis_runs")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	alterStmts := []string{
		`ALTER TABLE repo_analysis_runs ADD COLUMN chat_session_id TEXT;`,
		`ALTER TABLE repo_analysis_runs ADD COLUMN chat_user_message_id TEXT;`,
		`ALTER TABLE repo_analysis_runs ADD COLUMN chat_assistant_message_id TEXT;`,
		`ALTER TABLE repo_analysis_runs ADD COLUMN runtime_kind TEXT;`,
		`ALTER TABLE repo_analysis_runs ADD COLUMN cli_tool_id TEXT;`,
		`ALTER TABLE repo_analysis_runs ADD COLUMN model_id TEXT;`,
	}
	for _, stmt := range alterStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				continue
			}
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	indexStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_repo_analysis_runs_chat_session ON repo_analysis_runs(chat_session_id);`,
		`CREATE INDEX IF NOT EXISTS idx_repo_analysis_runs_runtime_kind ON repo_analysis_runs(runtime_kind, created_at DESC);`,
	}
	for _, stmt := range indexStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func migrateV10(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS chat_turns (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  user_message_id TEXT NOT NULL,
  assistant_message_id TEXT,
  turn INTEGER NOT NULL,
  status TEXT NOT NULL,
  expert_id TEXT,
  provider TEXT,
  model TEXT,
  model_input TEXT,
  context_mode TEXT,
  thinking_translation_applied INTEGER NOT NULL DEFAULT 0,
  thinking_translation_failed INTEGER NOT NULL DEFAULT 0,
  token_in INTEGER,
  token_out INTEGER,
  cached_input_tokens INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  completed_at INTEGER,
  FOREIGN KEY(session_id) REFERENCES chat_sessions(id),
  FOREIGN KEY(user_message_id) REFERENCES chat_messages(id),
  FOREIGN KEY(assistant_message_id) REFERENCES chat_messages(id)
);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_turns_user_message ON chat_turns(user_message_id);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_turns_session_turn ON chat_turns(session_id, turn, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_turns_session_status ON chat_turns(session_id, status, updated_at);`,
		`CREATE TABLE IF NOT EXISTS chat_turn_items (
  turn_id TEXT NOT NULL,
  entry_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  content_text TEXT NOT NULL,
  translated_content TEXT,
  translation_failed INTEGER NOT NULL DEFAULT 0,
  meta_json TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY(turn_id, entry_id),
  FOREIGN KEY(turn_id) REFERENCES chat_turns(id)
);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_turn_items_turn_seq ON chat_turn_items(turn_id, seq, created_at);`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration stmt: %w", err)
		}
	}
	return nil
}

func ensureChatSessionCLIColumns(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExists(ctx, tx, "chat_sessions")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	cols, err := tableColumns(ctx, tx, "chat_sessions")
	if err != nil {
		return err
	}
	needed := map[string]string{
		"cli_tool_id":         `ALTER TABLE chat_sessions ADD COLUMN cli_tool_id TEXT;`,
		"model_id":            `ALTER TABLE chat_sessions ADD COLUMN model_id TEXT;`,
		"cli_session_id":      `ALTER TABLE chat_sessions ADD COLUMN cli_session_id TEXT;`,
		"mcp_server_ids_json": `ALTER TABLE chat_sessions ADD COLUMN mcp_server_ids_json TEXT;`,
	}
	for key, stmt := range needed {
		if cols[key] {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("repair chat_sessions.%s: %w", key, err)
		}
	}
	return nil
}
