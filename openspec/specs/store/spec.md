# 存储层

## Purpose

SQLite + 文件的混合存储策略。SQLite 存储结构化元数据和审计事件，文件系统存储高频日志流。
## Requirements

### SQLite Configuration

The system MUST enable WAL mode (Write-Ahead Logging). The system MUST configure `_busy_timeout=5000` (milliseconds). The system MUST enable `_foreign_keys=on`. The system MUST enforce single connection pool: `SetMaxOpenConns(1)` and `SetMaxIdleConns(1)`. The system MUST prohibit any external I/O (network/PTY) within transactions to keep write lock duration minimal.

#### Scenario: Database initialization

- **WHEN** daemon opens state.db for the first time
- **THEN** WAL mode is enabled
- **AND** busy_timeout is set to 5000ms
- **AND** foreign keys are enabled
- **AND** connection pool is limited to 1

#### Scenario: No lock contention under concurrent writes

- **WHEN** multiple goroutines simultaneously update node statuses
- **THEN** all writes succeed without "database is locked" errors
- **AND** busy_timeout handles contention transparently

### Database Schema

The system MUST maintain 5 tables: `workflows` (id, title, workspace_path, mode, status, created_at, updated_at, error_message, summary), `nodes` (id, workflow_id, node_type, expert_id, title, prompt, status, created_at, updated_at, last_execution_id, result_summary, result_json, error_message), `edges` (id, workflow_id, from_node_id, to_node_id, source_handle, target_handle, type), `executions` (id, node_id, attempt, pid, exit_code, status, log_path, started_at, ended_at, error_message), `events` (id, workflow_id, entity_type, entity_id, type, ts, payload_json). The system MUST create index `idx_events_workflow_ts ON events(workflow_id, ts)`.

The system MUST additionally maintain `expert_builder_sessions` (id, title, target_expert_id, builder_model_id, status, created_at, updated_at, latest_snapshot_id), `expert_builder_messages` (id, session_id, role, content_text, created_at), and `expert_builder_snapshots` (id, session_id, version, assistant_message, draft_json, raw_json, warnings_json, created_at).

#### Scenario: Tables and indexes exist after init

- **WHEN** migration runs on a fresh database
- **THEN** all 5 tables are created with correct columns
- **AND** the events index is created

### Schema Migration

The system MUST use `PRAGMA user_version` to manage schema versions. Each schema change MUST increment user_version.

#### Scenario: Schema upgrade on daemon restart

- **WHEN** daemon starts with an older user_version
- **THEN** pending migrations are applied in order
- **AND** user_version is updated to the latest version

#### Scenario: Builder tables exist after migration

- **WHEN** migration runs on a database older than the builder-session schema version
- **THEN** the builder session, message, and snapshot tables are created with indexes
- **AND** user_version is updated to the latest version

### Audit Events

The system MUST use the events table as append-only audit log. The system MUST record event types: `workflow.updated`, `dag.generated`, `node.log`, `prompt.updated`, `node.updated`, `status.changed`. The system MUST NOT store high-frequency log chunks in the events table.

#### Scenario: Prompt update audit

- **WHEN** user modifies a node's prompt via PATCH API
- **THEN** an event with type `prompt.updated` is written
- **AND** the payload contains old and new prompt values

### Requirement: Chat attachment metadata SHALL be stored in SQLite
The system MUST persist chat attachment metadata in SQLite using a table dedicated to message-linked attachments.

Each attachment record MUST reference its owning chat session and message.

#### Scenario: Attachment record is created
- **WHEN** a chat turn with attachments is successfully accepted
- **THEN** SQLite stores one attachment record per uploaded file
- **AND** each record links to the owning `session_id` and `message_id`

### File Storage Layout

The system MUST use data directory `~/.local/share/vibe-tree/`. The database file MUST be at `~/.local/share/vibe-tree/state.db`. Log files MUST be stored under `~/.local/share/vibe-tree/logs/`. Each execution MUST have its own log file: `{execution_id}.log`.

Chat attachments MUST be stored under a dedicated subdirectory of the data directory, organized by session and message so that persisted attachment files can be re-read for future chat reconstruction.

#### Scenario: Execution log file creation

- **WHEN** a new execution starts
- **THEN** a log file is created at `~/.local/share/vibe-tree/logs/{execution_id}.log`

#### Scenario: Chat attachment file creation
- **WHEN** a user sends a chat turn with attachments
- **THEN** each accepted attachment is stored under the chat attachment data directory
- **AND** the storage path is stable enough for later message history reconstruction

### ID Generation

The system MUST use prefixed IDs: `wf_` for workflows, `nd_` for nodes, `ex_` for executions. IDs MUST use short random strings (MVP).

#### Scenario: Generate workflow ID

- **WHEN** a new workflow is created
- **THEN** its ID starts with `wf_` followed by a random string

### Requirement: Store MUST persist Repo Library entities
The store SHALL persist repository sources, repository snapshots, analysis runs, knowledge cards, card evidence, and search query history for Repo Library.

Repo analysis runs SHALL also persist the associated chat session identifier and the selected runtime/tool/model metadata when the analysis is AI-chat driven.

#### Scenario: Store creates Repo Library records
- **WHEN** the backend creates a repository source, snapshot, analysis run, and associated automated chat session
- **THEN** the store persists those records with stable identifiers
- **AND** later queries can retrieve the analysis run together with its linked chat session metadata

#### Scenario: Store lists repository summaries
- **WHEN** the UI requests Repo Library repositories
- **THEN** the store returns repository-level summaries with latest snapshot and latest analysis metadata
- **AND** the records are ordered by recent activity


### Requirement: Store MUST persist chat turn timelines in SQLite
The store MUST persist chat turn timelines in SQLite using dedicated turn and turn-item tables in addition to existing chat session and message tables.

The turn table MUST record turn-level metadata needed for recovery, including session linkage, user message linkage, completion linkage, lifecycle state, and restoration metadata.

The turn-item table MUST record persisted structured timeline entries keyed by stable turn identity and `entry_id`, and it MUST preserve chronological `seq` ordering for recovery.

#### Scenario: Timeline tables exist after migration
- **WHEN** the daemon opens a database that has not yet seen chat timeline persistence
- **THEN** migration creates the chat turn and chat turn item tables with required indexes
- **AND** the schema version is updated to the latest version

#### Scenario: Store upserts one timeline entry in place
- **WHEN** the daemon receives a new update for an existing persisted timeline `entry_id`
- **THEN** the store updates the existing turn-item row instead of creating a duplicate visible entry
- **AND** the original chronological `seq` remains stable

### Requirement: Store MUST list persisted chat turns with ordered items
The store MUST provide query helpers that return persisted chat turns together with their ordered timeline items for one session.

Returned items MUST be sorted by ascending `seq` within each turn.

#### Scenario: Query returns ordered timeline items
- **WHEN** the backend reads persisted turns for a session
- **THEN** each turn includes its associated persisted items in chronological order
- **AND** the query result is sufficient for the frontend to rebuild completed and running timelines
