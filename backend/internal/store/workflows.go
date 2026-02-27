package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"vibe-tree/backend/internal/id"
)

type WorkflowMode string

const (
	WorkflowModeAuto   WorkflowMode = "auto"
	WorkflowModeManual WorkflowMode = "manual"
)

type WorkflowStatus string

const (
	WorkflowStatusTodo     WorkflowStatus = "todo"
	WorkflowStatusRunning  WorkflowStatus = "running"
	WorkflowStatusDone     WorkflowStatus = "done"
	WorkflowStatusFailed   WorkflowStatus = "failed"
	WorkflowStatusCanceled WorkflowStatus = "canceled"
)

type Workflow struct {
	ID            string  `json:"workflow_id"`
	Title         string  `json:"title"`
	WorkspacePath string  `json:"workspace_path"`
	Mode          string  `json:"mode"`
	Status        string  `json:"status"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
	ErrorMessage  *string `json:"error_message,omitempty"`
	Summary       *string `json:"summary,omitempty"`
}

type CreateWorkflowParams struct {
	Title         string
	WorkspacePath string
	Mode          string
}

// CreateWorkflow 功能：创建一条 workflow 记录（初始 status=todo），并写入 `workflow.updated` 审计事件。
// 参数/返回：params 指定 title/workspace/mode；返回创建后的 Workflow。
// 失败场景：参数非法、插入失败或事件写入失败时返回 error。
// 副作用：写入 SQLite workflows/events。
func (s *Store) CreateWorkflow(ctx context.Context, params CreateWorkflowParams) (Workflow, error) {
	if s == nil || s.db == nil {
		return Workflow{}, fmt.Errorf("store not initialized")
	}

	title := params.Title
	if title == "" {
		title = "Untitled"
	}
	workspace := params.WorkspacePath
	if workspace == "" {
		return Workflow{}, fmt.Errorf("%w: workspace_path is required", ErrValidation)
	}

	mode := params.Mode
	if mode == "" {
		mode = string(WorkflowModeManual)
	}
	if mode != string(WorkflowModeAuto) && mode != string(WorkflowModeManual) {
		return Workflow{}, fmt.Errorf("%w: invalid mode %q", ErrValidation, mode)
	}

	now := time.Now().UnixMilli()
	wf := Workflow{
		ID:            id.New("wf_"),
		Title:         title,
		WorkspacePath: workspace,
		Mode:          mode,
		Status:        string(WorkflowStatusTodo),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO workflows (id, title, workspace_path, mode, status, created_at, updated_at, error_message, summary)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL, NULL);`,
		wf.ID, wf.Title, wf.WorkspacePath, wf.Mode, wf.Status, wf.CreatedAt, wf.UpdatedAt,
	)
	if err != nil {
		return Workflow{}, fmt.Errorf("insert workflow: %w", err)
	}

	if err := insertEvent(ctx, tx, wf.ID, "workflow", wf.ID, "workflow.updated", now, map[string]any{
		"action": "created",
	}); err != nil {
		return Workflow{}, err
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, fmt.Errorf("commit: %w", err)
	}
	return wf, nil
}

// ListWorkflows 功能：按更新时间倒序读取 workflows 列表。
// 参数/返回：limit 为最大条数（<=0 使用默认 50）；返回 workflows 切片。
// 失败场景：SQL 查询失败或扫描失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListWorkflows(ctx context.Context, limit int) ([]Workflow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, title, workspace_path, mode, status, created_at, updated_at, error_message, summary
		 FROM workflows
		 ORDER BY updated_at DESC
		 LIMIT ?;`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query workflows: %w", err)
	}
	defer rows.Close()

	var out []Workflow
	for rows.Next() {
		var wf Workflow
		if err := rows.Scan(
			&wf.ID,
			&wf.Title,
			&wf.WorkspacePath,
			&wf.Mode,
			&wf.Status,
			&wf.CreatedAt,
			&wf.UpdatedAt,
			&wf.ErrorMessage,
			&wf.Summary,
		); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		out = append(out, wf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflows: %w", err)
	}
	return out, nil
}

// GetWorkflow 功能：读取指定 workflow。
// 参数/返回：workflowID 为目标 ID；命中返回 Workflow，未命中返回 os.ErrNotExist。
// 失败场景：SQL 查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetWorkflow(ctx context.Context, workflowID string) (Workflow, error) {
	if s == nil || s.db == nil {
		return Workflow{}, fmt.Errorf("store not initialized")
	}

	var wf Workflow
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, title, workspace_path, mode, status, created_at, updated_at, error_message, summary
		 FROM workflows
		 WHERE id = ?
		 LIMIT 1;`,
		workflowID,
	).Scan(
		&wf.ID,
		&wf.Title,
		&wf.WorkspacePath,
		&wf.Mode,
		&wf.Status,
		&wf.CreatedAt,
		&wf.UpdatedAt,
		&wf.ErrorMessage,
		&wf.Summary,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, os.ErrNotExist
		}
		return Workflow{}, fmt.Errorf("query workflow: %w", err)
	}
	return wf, nil
}

type UpdateWorkflowParams struct {
	Title         *string
	WorkspacePath *string
	Mode          *string
}

// UpdateWorkflow 功能：更新 workflow 的 title/workspace/mode（按需），并写入 `workflow.updated` 审计事件。
// 参数/返回：workflowID 为目标；patch 为可选字段；返回更新后的 Workflow。
// 失败场景：workflow 不存在、参数非法或 SQL 更新失败时返回 error。
// 副作用：写入 SQLite workflows/events。
func (s *Store) UpdateWorkflow(ctx context.Context, workflowID string, patch UpdateWorkflowParams) (Workflow, error) {
	if s == nil || s.db == nil {
		return Workflow{}, fmt.Errorf("store not initialized")
	}
	if patch.Title == nil && patch.WorkspacePath == nil && patch.Mode == nil {
		return Workflow{}, fmt.Errorf("%w: empty patch", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	current, err := getWorkflowTx(ctx, tx, workflowID)
	if err != nil {
		return Workflow{}, err
	}

	fields := make([]string, 0, 3)
	updated := current
	if patch.Title != nil {
		updated.Title = *patch.Title
		fields = append(fields, "title")
	}
	if patch.WorkspacePath != nil {
		if *patch.WorkspacePath == "" {
			return Workflow{}, fmt.Errorf("%w: workspace_path is required", ErrValidation)
		}
		updated.WorkspacePath = *patch.WorkspacePath
		fields = append(fields, "workspace_path")
	}
	if patch.Mode != nil {
		if *patch.Mode != string(WorkflowModeAuto) && *patch.Mode != string(WorkflowModeManual) {
			return Workflow{}, fmt.Errorf("%w: invalid mode %q", ErrValidation, *patch.Mode)
		}
		updated.Mode = *patch.Mode
		fields = append(fields, "mode")
	}

	updated.UpdatedAt = time.Now().UnixMilli()

	res, err := tx.ExecContext(
		ctx,
		`UPDATE workflows
		 SET title = ?, workspace_path = ?, mode = ?, updated_at = ?
		 WHERE id = ?;`,
		updated.Title, updated.WorkspacePath, updated.Mode, updated.UpdatedAt, workflowID,
	)
	if err != nil {
		return Workflow{}, fmt.Errorf("update workflow: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return Workflow{}, os.ErrNotExist
	}

	if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", updated.UpdatedAt, map[string]any{
		"action": "updated",
		"fields": fields,
	}); err != nil {
		return Workflow{}, err
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}

func getWorkflowTx(ctx context.Context, tx *sql.Tx, workflowID string) (Workflow, error) {
	var wf Workflow
	err := tx.QueryRowContext(
		ctx,
		`SELECT id, title, workspace_path, mode, status, created_at, updated_at, error_message, summary
		 FROM workflows
		 WHERE id = ?
		 LIMIT 1;`,
		workflowID,
	).Scan(
		&wf.ID,
		&wf.Title,
		&wf.WorkspacePath,
		&wf.Mode,
		&wf.Status,
		&wf.CreatedAt,
		&wf.UpdatedAt,
		&wf.ErrorMessage,
		&wf.Summary,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, os.ErrNotExist
		}
		return Workflow{}, fmt.Errorf("query workflow: %w", err)
	}
	return wf, nil
}

func insertEvent(
	ctx context.Context,
	tx *sql.Tx,
	workflowID string,
	entityType string,
	entityID string,
	eventType string,
	ts int64,
	payload any,
) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO events (id, workflow_id, entity_type, entity_id, type, ts, payload_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?);`,
		id.New("ev_"), workflowID, entityType, entityID, eventType, ts, string(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}
