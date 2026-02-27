package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"
)

type Execution struct {
	ID        string  `json:"execution_id"`
	NodeID    string  `json:"node_id"`
	Attempt   int     `json:"attempt"`
	PID       *int    `json:"pid,omitempty"`
	ExitCode  *int    `json:"exit_code,omitempty"`
	Status    string  `json:"status"`
	LogPath   string  `json:"log_path"`
	StartedAt int64   `json:"started_at"`
	EndedAt   *int64  `json:"ended_at,omitempty"`
	Signal    *string `json:"signal,omitempty"`
	Error     *string `json:"error_message,omitempty"`
}

type StartExecutionParams struct {
	ExecutionID string
	WorkflowID  string
	NodeID      string
	Attempt     int
	PID         int
	LogPath     string
	StartedAt   int64
	Command     string
	Args        []string
	Cwd         string
}

// StartExecution 功能：为 node 创建一条 running execution 记录，并更新 node.last_execution_id/status。
// 参数/返回：params 指定 execution 元数据；成功返回更新后的 Node。
// 失败场景：node 不存在返回 os.ErrNotExist；写库失败返回 error。
// 副作用：写入 SQLite executions/nodes/events。
func (s *Store) StartExecution(ctx context.Context, params StartExecutionParams) (Node, error) {
	if s == nil || s.db == nil {
		return Node{}, fmt.Errorf("store not initialized")
	}
	if params.ExecutionID == "" || params.NodeID == "" || params.WorkflowID == "" {
		return Node{}, fmt.Errorf("%w: missing execution/node/workflow id", ErrValidation)
	}
	if params.Attempt <= 0 {
		params.Attempt = 1
	}
	if params.LogPath == "" {
		return Node{}, fmt.Errorf("%w: log_path is required", ErrValidation)
	}
	if params.StartedAt <= 0 {
		params.StartedAt = time.Now().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Ensure node exists.
	var node Node
	err = tx.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
		        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
		   FROM nodes
		  WHERE id = ?
		  LIMIT 1;`,
		params.NodeID,
	).Scan(
		&node.ID,
		&node.WorkflowID,
		&node.NodeType,
		&node.ExpertID,
		&node.Title,
		&node.Prompt,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
		&node.LastExecution,
		&node.ResultSummary,
		&node.ResultJSON,
		&node.ErrorMessage,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, os.ErrNotExist
		}
		return Node{}, fmt.Errorf("query node: %w", err)
	}
	if node.WorkflowID != params.WorkflowID {
		return Node{}, fmt.Errorf("%w: node workflow mismatch", ErrValidation)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO executions (
			id, node_id, attempt, pid, exit_code, status, log_path, started_at, ended_at, error_message
		) VALUES (?, ?, ?, ?, NULL, ?, ?, ?, NULL, NULL);`,
		params.ExecutionID,
		params.NodeID,
		params.Attempt,
		params.PID,
		"running",
		params.LogPath,
		params.StartedAt,
	)
	if err != nil {
		return Node{}, fmt.Errorf("insert execution: %w", err)
	}

	node.LastExecution = &params.ExecutionID
	node.Status = "running"
	node.UpdatedAt = params.StartedAt
	_, err = tx.ExecContext(
		ctx,
		`UPDATE nodes
		    SET status = ?, updated_at = ?, last_execution_id = ?
		  WHERE id = ?;`,
		node.Status, node.UpdatedAt, params.ExecutionID, params.NodeID,
	)
	if err != nil {
		return Node{}, fmt.Errorf("update node execution: %w", err)
	}

	if err := insertEvent(ctx, tx, params.WorkflowID, "execution", params.ExecutionID, "execution.started", params.StartedAt, map[string]any{
		"execution_id": params.ExecutionID,
		"node_id":      params.NodeID,
		"command":      params.Command,
		"args":         params.Args,
		"cwd":          params.Cwd,
	}); err != nil {
		return Node{}, err
	}
	if err := insertEvent(ctx, tx, params.WorkflowID, "node", params.NodeID, "node.updated", params.StartedAt, map[string]any{
		"action":     "execution.started",
		"node_id":    params.NodeID,
		"status":     node.Status,
		"execution":  params.ExecutionID,
		"started_at": params.StartedAt,
	}); err != nil {
		return Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit: %w", err)
	}
	return node, nil
}

type FinalizeExecutionParams struct {
	ExecutionID string
	WorkflowID  string
	NodeID      string
	Status      string
	ExitCode    int
	Signal      string
	StartedAt   int64
	EndedAt     int64
}

// FinalizeExecution 功能：将 execution 更新为终态（succeeded/failed/canceled/timeout），并同步更新 node 状态；必要时更新 workflow 状态。
// 参数/返回：params 指定退出信息；成功返回更新后的 Workflow 与 Node。
// 失败场景：execution/node/workflow 不存在返回 os.ErrNotExist；写库失败返回 error。
// 副作用：写入 SQLite executions/nodes/workflows/events。
func (s *Store) FinalizeExecution(ctx context.Context, params FinalizeExecutionParams) (Workflow, Node, error) {
	if s == nil || s.db == nil {
		return Workflow{}, Node{}, fmt.Errorf("store not initialized")
	}
	if params.ExecutionID == "" || params.NodeID == "" || params.WorkflowID == "" {
		return Workflow{}, Node{}, fmt.Errorf("%w: missing execution/node/workflow id", ErrValidation)
	}
	if params.EndedAt <= 0 {
		params.EndedAt = time.Now().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wf, err := getWorkflowTx(ctx, tx, params.WorkflowID)
	if err != nil {
		return Workflow{}, Node{}, err
	}

	var node Node
	err = tx.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
		        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
		   FROM nodes
		  WHERE id = ?
		  LIMIT 1;`,
		params.NodeID,
	).Scan(
		&node.ID,
		&node.WorkflowID,
		&node.NodeType,
		&node.ExpertID,
		&node.Title,
		&node.Prompt,
		&node.Status,
		&node.CreatedAt,
		&node.UpdatedAt,
		&node.LastExecution,
		&node.ResultSummary,
		&node.ResultJSON,
		&node.ErrorMessage,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, Node{}, os.ErrNotExist
		}
		return Workflow{}, Node{}, fmt.Errorf("query node: %w", err)
	}
	if node.WorkflowID != params.WorkflowID {
		return Workflow{}, Node{}, fmt.Errorf("%w: node workflow mismatch", ErrValidation)
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE executions
		    SET status = ?, exit_code = ?, started_at = ?, ended_at = ?, error_message = NULL
		  WHERE id = ?;`,
		params.Status, params.ExitCode, params.StartedAt, params.EndedAt, params.ExecutionID,
	)
	if err != nil {
		return Workflow{}, Node{}, fmt.Errorf("update execution: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return Workflow{}, Node{}, os.ErrNotExist
	}

	node.Status = params.Status
	node.UpdatedAt = params.EndedAt
	_, err = tx.ExecContext(
		ctx,
		`UPDATE nodes
		    SET status = ?, updated_at = ?, error_message = NULL
		  WHERE id = ?;`,
		node.Status, node.UpdatedAt, params.NodeID,
	)
	if err != nil {
		return Workflow{}, Node{}, fmt.Errorf("update node status: %w", err)
	}

	// MVP：当 workflow 只有 master node 时，让 workflow 直接跟随 master 终态。
	var otherCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM nodes WHERE workflow_id = ? AND node_type != 'master';`, params.WorkflowID).Scan(&otherCount); err == nil && otherCount == 0 {
		switch params.Status {
		case "succeeded":
			wf.Status = string(WorkflowStatusDone)
		case "canceled":
			wf.Status = string(WorkflowStatusCanceled)
		default:
			wf.Status = string(WorkflowStatusFailed)
		}
	}
	wf.UpdatedAt = params.EndedAt
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET status = ?, updated_at = ? WHERE id = ?;`, wf.Status, wf.UpdatedAt, params.WorkflowID); err != nil {
		return Workflow{}, Node{}, fmt.Errorf("update workflow: %w", err)
	}

	if err := insertEvent(ctx, tx, params.WorkflowID, "execution", params.ExecutionID, "execution.exited", params.EndedAt, map[string]any{
		"status":    params.Status,
		"exit_code": params.ExitCode,
		"signal":    params.Signal,
	}); err != nil {
		return Workflow{}, Node{}, err
	}
	if err := insertEvent(ctx, tx, params.WorkflowID, "node", params.NodeID, "node.updated", params.EndedAt, map[string]any{
		"action":    "execution.exited",
		"node_id":   params.NodeID,
		"status":    node.Status,
		"execution": params.ExecutionID,
		"ended_at":  params.EndedAt,
		"exit_code": params.ExitCode,
		"signal":    params.Signal,
	}); err != nil {
		return Workflow{}, Node{}, err
	}
	if err := insertEvent(ctx, tx, params.WorkflowID, "workflow", params.WorkflowID, "workflow.updated", wf.UpdatedAt, map[string]any{
		"action": "execution.exited",
		"status": wf.Status,
	}); err != nil {
		return Workflow{}, Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, Node{}, fmt.Errorf("commit: %w", err)
	}
	return wf, node, nil
}
