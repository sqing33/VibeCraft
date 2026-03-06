## MODIFIED Requirements

### Database Schema

The system MUST maintain 5 tables: `workflows` (id, title, workspace_path, mode, status, created_at, updated_at, error_message, summary), `nodes` (id, workflow_id, node_type, expert_id, title, prompt, status, created_at, updated_at, last_execution_id, result_summary, result_json, error_message), `edges` (id, workflow_id, from_node_id, to_node_id, source_handle, target_handle, type), `executions` (id, node_id, attempt, pid, exit_code, status, log_path, started_at, ended_at, error_message), `events` (id, workflow_id, entity_type, entity_id, type, ts, payload_json). The system MUST create index `idx_events_workflow_ts ON events(workflow_id, ts)`.

The system MUST additionally maintain `expert_builder_sessions` (id, title, target_expert_id, builder_model_id, status, created_at, updated_at, latest_snapshot_id), `expert_builder_messages` (id, session_id, role, content_text, created_at), and `expert_builder_snapshots` (id, session_id, version, assistant_message, draft_json, raw_json, warnings_json, created_at).

#### Scenario: Builder tables exist after migration
- **WHEN** migration runs on a database older than the builder-session schema version
- **THEN** the builder session, message, and snapshot tables are created with indexes
- **AND** user_version is updated to the latest version
