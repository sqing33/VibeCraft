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
	ExecutionID   string
	WorkflowID    string
	NodeID        string
	Status        string
	ExitCode      int
	Signal        string
	StartedAt     int64
	EndedAt       int64
	ErrorMessage  string
	ResultSummary *string
}

// FinalizeExecution 功能：将 execution 更新为终态（succeeded/failed/canceled/timeout），并同步更新 node/workflow 状态（含 fail-fast 跳过未开始节点）。
// 参数/返回：params 指定退出信息与可选 ErrorMessage；成功返回更新后的 Workflow 与本次被更新的 Nodes（含当前 node 与被 skip 的节点）。
// 失败场景：execution/node/workflow 不存在返回 os.ErrNotExist；写库失败返回 error。
// 副作用：写入 SQLite executions/nodes/workflows/events。
func (s *Store) FinalizeExecution(ctx context.Context, params FinalizeExecutionParams) (Workflow, []Node, error) {
	if s == nil || s.db == nil {
		return Workflow{}, nil, fmt.Errorf("store not initialized")
	}
	if params.ExecutionID == "" || params.NodeID == "" || params.WorkflowID == "" {
		return Workflow{}, nil, fmt.Errorf("%w: missing execution/node/workflow id", ErrValidation)
	}
	if params.EndedAt <= 0 {
		params.EndedAt = time.Now().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wf, err := getWorkflowTx(ctx, tx, params.WorkflowID)
	if err != nil {
		return Workflow{}, nil, err
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
			return Workflow{}, nil, os.ErrNotExist
		}
		return Workflow{}, nil, fmt.Errorf("query node: %w", err)
	}
	if node.WorkflowID != params.WorkflowID {
		return Workflow{}, nil, fmt.Errorf("%w: node workflow mismatch", ErrValidation)
	}

	execErr := any(nil)
	nodeErr := any(nil)
	if params.ErrorMessage != "" && (params.Status == "failed" || params.Status == "canceled" || params.Status == "timeout") {
		execErr = params.ErrorMessage
		nodeErr = params.ErrorMessage
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE executions
		    SET status = ?, exit_code = ?, started_at = ?, ended_at = ?, error_message = ?
		  WHERE id = ?;`,
		params.Status, params.ExitCode, params.StartedAt, params.EndedAt, execErr, params.ExecutionID,
	)
	if err != nil {
		return Workflow{}, nil, fmt.Errorf("update execution: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return Workflow{}, nil, os.ErrNotExist
	}

	node.Status = params.Status
	node.UpdatedAt = params.EndedAt
	if nodeErr == nil {
		node.ErrorMessage = nil
	} else {
		msg := params.ErrorMessage
		node.ErrorMessage = &msg
	}
	if params.ResultSummary != nil {
		node.ResultSummary = params.ResultSummary
	}

	summaryArg := any(nil)
	if params.ResultSummary != nil {
		summaryArg = *params.ResultSummary
	}
	_, err = tx.ExecContext(
		ctx,
		`UPDATE nodes
		    SET status = ?, updated_at = ?, error_message = ?, result_summary = COALESCE(?, result_summary)
		  WHERE id = ?;`,
		node.Status, node.UpdatedAt, nodeErr, summaryArg, params.NodeID,
	)
	if err != nil {
		return Workflow{}, nil, fmt.Errorf("update node status: %w", err)
	}

	updatedNodes := []Node{node}

	// Fail-fast：worker 节点失败/取消/超时后，将未开始的节点标记为 skipped。
	if node.NodeType != "master" && (params.Status == "failed" || params.Status == "canceled" || params.Status == "timeout") {
		rows, err := tx.QueryContext(
			ctx,
			`SELECT id
			   FROM nodes
			  WHERE workflow_id = ?
			    AND node_type != 'master'
			    AND status IN ('queued', 'pending_approval', 'draft')
			    AND id != ?;`,
			params.WorkflowID,
			node.ID,
		)
		if err != nil {
			return Workflow{}, nil, fmt.Errorf("list nodes to skip: %w", err)
		}
		var toSkip []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return Workflow{}, nil, fmt.Errorf("scan node id: %w", err)
			}
			toSkip = append(toSkip, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return Workflow{}, nil, fmt.Errorf("iterate nodes to skip: %w", err)
		}

		if len(toSkip) > 0 {
			reason := "fail_fast"
			for _, id := range toSkip {
				if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status = 'skipped', updated_at = ?, error_message = ? WHERE id = ?;`, params.EndedAt, reason, id); err != nil {
					return Workflow{}, nil, fmt.Errorf("skip node %s: %w", id, err)
				}
				if err := insertEvent(ctx, tx, params.WorkflowID, "node", id, "node.updated", params.EndedAt, map[string]any{
					"action":  "skipped",
					"node_id": id,
					"status":  "skipped",
					"reason":  reason,
				}); err != nil {
					return Workflow{}, nil, err
				}

				var skipped Node
				if err := tx.QueryRowContext(
					ctx,
					`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
					        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
					   FROM nodes
					  WHERE id = ?
					  LIMIT 1;`,
					id,
				).Scan(
					&skipped.ID,
					&skipped.WorkflowID,
					&skipped.NodeType,
					&skipped.ExpertID,
					&skipped.Title,
					&skipped.Prompt,
					&skipped.Status,
					&skipped.CreatedAt,
					&skipped.UpdatedAt,
					&skipped.LastExecution,
					&skipped.ResultSummary,
					&skipped.ResultJSON,
					&skipped.ErrorMessage,
				); err == nil {
					updatedNodes = append(updatedNodes, skipped)
				}
			}
		}
	}

	// 计算 workflow 状态（master-only / 多节点）。
	workerTotal := 0
	workerSucceeded := 0
	hasRunningOrPending := false
	hasFailed := false
	hasCanceled := false
	hasSkipped := false

	rows, err := tx.QueryContext(ctx, `SELECT node_type, status FROM nodes WHERE workflow_id = ?;`, params.WorkflowID)
	if err != nil {
		return Workflow{}, nil, fmt.Errorf("query workflow nodes: %w", err)
	}
	for rows.Next() {
		var nodeType, status string
		if err := rows.Scan(&nodeType, &status); err != nil {
			rows.Close()
			return Workflow{}, nil, fmt.Errorf("scan workflow node status: %w", err)
		}
		if nodeType != "master" {
			workerTotal++
		}

		switch status {
		case "failed", "timeout":
			hasFailed = true
		case "canceled":
			hasCanceled = true
		case "skipped":
			hasSkipped = true
		case "running", "queued", "pending_approval", "draft":
			if nodeType != "master" {
				hasRunningOrPending = true
			}
		case "succeeded":
			if nodeType != "master" {
				workerSucceeded++
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Workflow{}, nil, fmt.Errorf("iterate workflow nodes: %w", err)
	}

	switch {
	case workerTotal == 0:
		switch params.Status {
		case "succeeded":
			wf.Status = string(WorkflowStatusDone)
		case "canceled":
			wf.Status = string(WorkflowStatusCanceled)
		default:
			wf.Status = string(WorkflowStatusFailed)
		}
	case hasFailed || hasSkipped:
		wf.Status = string(WorkflowStatusFailed)
	case hasCanceled:
		wf.Status = string(WorkflowStatusCanceled)
	case workerSucceeded == workerTotal:
		wf.Status = string(WorkflowStatusDone)
	case hasRunningOrPending:
		wf.Status = string(WorkflowStatusRunning)
	default:
		wf.Status = string(WorkflowStatusRunning)
	}

	wf.UpdatedAt = params.EndedAt
	workflowErr := any(nil)
	if (wf.Status == string(WorkflowStatusFailed) || wf.Status == string(WorkflowStatusCanceled)) && params.ErrorMessage != "" {
		workflowErr = params.ErrorMessage
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET status = ?, updated_at = ?, error_message = ? WHERE id = ?;`, wf.Status, wf.UpdatedAt, workflowErr, params.WorkflowID); err != nil {
		return Workflow{}, nil, fmt.Errorf("update workflow: %w", err)
	}

	if err := insertEvent(ctx, tx, params.WorkflowID, "execution", params.ExecutionID, "execution.exited", params.EndedAt, map[string]any{
		"status":    params.Status,
		"exit_code": params.ExitCode,
		"signal":    params.Signal,
	}); err != nil {
		return Workflow{}, nil, err
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
		return Workflow{}, nil, err
	}
	if err := insertEvent(ctx, tx, params.WorkflowID, "workflow", params.WorkflowID, "workflow.updated", wf.UpdatedAt, map[string]any{
		"action": "execution.exited",
		"status": wf.Status,
	}); err != nil {
		return Workflow{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, nil, fmt.Errorf("commit: %w", err)
	}
	return wf, updatedNodes, nil
}
