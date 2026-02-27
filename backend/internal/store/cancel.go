package store

import (
	"context"
	"fmt"
	"time"
)

type CancelWorkflowResult struct {
	Workflow            Workflow
	CanceledNodes       []Node
	RunningExecutionIDs []string
}

// CancelWorkflow 功能：取消一个 workflow（将 workflow 标记为 canceled，并将未开始的 worker nodes 标记为 canceled，同时返回当前 running 的 execution IDs 供上层触发进程 cancel）。
// 参数/返回：workflowID 为目标；返回取消结果（含 workflow、被标记为 canceled 的 nodes，以及需要 cancel 的 running execution IDs）。
// 失败场景：workflow 不存在返回 os.ErrNotExist；workflow 已终态（done/failed）返回 ErrConflict；写库失败返回 error。
// 副作用：写入 SQLite workflows/nodes/events，并读取 executions 列表。
func (s *Store) CancelWorkflow(ctx context.Context, workflowID string) (CancelWorkflowResult, error) {
	if s == nil || s.db == nil {
		return CancelWorkflowResult{}, fmt.Errorf("store not initialized")
	}
	if workflowID == "" {
		return CancelWorkflowResult{}, fmt.Errorf("%w: workflow_id is required", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wf, err := getWorkflowTx(ctx, tx, workflowID)
	if err != nil {
		return CancelWorkflowResult{}, err
	}

	switch wf.Status {
	case string(WorkflowStatusDone), string(WorkflowStatusFailed):
		return CancelWorkflowResult{}, fmt.Errorf("%w: workflow already finished", ErrConflict)
	case string(WorkflowStatusCanceled):
		return CancelWorkflowResult{Workflow: wf}, nil
	}

	rows, err := tx.QueryContext(
		ctx,
		`SELECT e.id
		   FROM executions e
		   JOIN nodes n ON n.id = e.node_id
		  WHERE n.workflow_id = ?
		    AND e.status = 'running'
		  ORDER BY e.started_at ASC;`,
		workflowID,
	)
	if err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("query running executions: %w", err)
	}
	var execIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return CancelWorkflowResult{}, fmt.Errorf("scan running execution id: %w", err)
		}
		execIDs = append(execIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("iterate running executions: %w", err)
	}

	now := time.Now().UnixMilli()
	wf.Status = string(WorkflowStatusCanceled)
	wf.UpdatedAt = now
	reason := "workflow_canceled"

	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET status = ?, updated_at = ?, error_message = NULL WHERE id = ?;`, wf.Status, wf.UpdatedAt, workflowID); err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("update workflow canceled: %w", err)
	}
	if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", now, map[string]any{
		"action": "canceled",
		"status": wf.Status,
	}); err != nil {
		return CancelWorkflowResult{}, err
	}

	// 将未开始的 worker nodes 标记为 canceled，避免被调度器继续推进。
	nodeRows, err := tx.QueryContext(
		ctx,
		`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
		        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
		   FROM nodes
		  WHERE workflow_id = ?
		    AND node_type != 'master'
		    AND status IN ('queued', 'pending_approval', 'draft')
		  ORDER BY created_at ASC;`,
		workflowID,
	)
	if err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("query nodes to cancel: %w", err)
	}
	var canceledNodes []Node
	for nodeRows.Next() {
		var n Node
		if err := nodeRows.Scan(
			&n.ID,
			&n.WorkflowID,
			&n.NodeType,
			&n.ExpertID,
			&n.Title,
			&n.Prompt,
			&n.Status,
			&n.CreatedAt,
			&n.UpdatedAt,
			&n.LastExecution,
			&n.ResultSummary,
			&n.ResultJSON,
			&n.ErrorMessage,
		); err != nil {
			nodeRows.Close()
			return CancelWorkflowResult{}, fmt.Errorf("scan node to cancel: %w", err)
		}
		canceledNodes = append(canceledNodes, n)
	}
	nodeRows.Close()
	if err := nodeRows.Err(); err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("iterate nodes to cancel: %w", err)
	}

	for i := range canceledNodes {
		n := &canceledNodes[i]
		n.Status = "canceled"
		n.UpdatedAt = now
		msg := reason
		n.ErrorMessage = &msg

		if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status = ?, updated_at = ?, error_message = ? WHERE id = ?;`, n.Status, n.UpdatedAt, reason, n.ID); err != nil {
			return CancelWorkflowResult{}, fmt.Errorf("update node canceled: %w", err)
		}
		if err := insertEvent(ctx, tx, workflowID, "node", n.ID, "node.updated", now, map[string]any{
			"action":  "canceled",
			"node_id": n.ID,
			"status":  n.Status,
			"reason":  reason,
		}); err != nil {
			return CancelWorkflowResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return CancelWorkflowResult{}, fmt.Errorf("commit: %w", err)
	}

	return CancelWorkflowResult{
		Workflow:            wf,
		CanceledNodes:       canceledNodes,
		RunningExecutionIDs: execIDs,
	}, nil
}
