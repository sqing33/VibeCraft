package store

import (
	"context"
	"fmt"
	"time"
)

// ApproveRunnableNodes 功能：在 manual 模式下，将所有“依赖已满足”的 pending_approval 节点推进到 queued，等待调度执行。
// 参数/返回：workflowID 为目标；返回更新后的 workflow 与被推进的 nodes 列表（可能为空）。
// 失败场景：workflow 不存在返回 os.ErrNotExist；workflow 非 manual 返回 ErrValidation；写库失败返回 error。
// 副作用：写入 SQLite nodes/workflows/events。
func (s *Store) ApproveRunnableNodes(ctx context.Context, workflowID string) (Workflow, []Node, error) {
	if s == nil || s.db == nil {
		return Workflow{}, nil, fmt.Errorf("store not initialized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wf, err := getWorkflowTx(ctx, tx, workflowID)
	if err != nil {
		return Workflow{}, nil, err
	}
	if wf.Mode != string(WorkflowModeManual) {
		return Workflow{}, nil, fmt.Errorf("%w: approve only allowed in manual mode", ErrValidation)
	}

	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
		        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
		   FROM nodes n
		  WHERE n.workflow_id = ?
		    AND n.node_type != 'master'
		    AND n.status = 'pending_approval'
		    AND NOT EXISTS (
		      SELECT 1
		        FROM edges e
		        JOIN nodes dep ON dep.id = e.from_node_id
		       WHERE e.workflow_id = n.workflow_id
		         AND e.to_node_id = n.id
		         AND e.type = 'success'
		         AND dep.status != 'succeeded'
		    )
		  ORDER BY n.created_at ASC;`,
		workflowID,
	)
	if err != nil {
		return Workflow{}, nil, fmt.Errorf("query approvable nodes: %w", err)
	}
	defer rows.Close()

	toApprove := make([]Node, 0)
	for rows.Next() {
		var n Node
		if err := rows.Scan(
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
			return Workflow{}, nil, fmt.Errorf("scan approvable node: %w", err)
		}
		toApprove = append(toApprove, n)
	}
	if err := rows.Err(); err != nil {
		return Workflow{}, nil, fmt.Errorf("iterate approvable nodes: %w", err)
	}

	now := time.Now().UnixMilli()
	for i := range toApprove {
		n := &toApprove[i]
		n.Status = "queued"
		n.UpdatedAt = now
		n.ErrorMessage = nil
		if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status = ?, updated_at = ?, error_message = NULL WHERE id = ?;`, n.Status, n.UpdatedAt, n.ID); err != nil {
			return Workflow{}, nil, fmt.Errorf("approve node %s: %w", n.ID, err)
		}
		if err := insertEvent(ctx, tx, workflowID, "node", n.ID, "node.updated", now, map[string]any{
			"action":  "approved",
			"node_id": n.ID,
			"status":  n.Status,
		}); err != nil {
			return Workflow{}, nil, err
		}
	}

	wf.UpdatedAt = now
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET updated_at = ? WHERE id = ?;`, wf.UpdatedAt, workflowID); err != nil {
		return Workflow{}, nil, fmt.Errorf("touch workflow updated_at: %w", err)
	}
	if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", now, map[string]any{
		"action": "approved",
	}); err != nil {
		return Workflow{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, nil, fmt.Errorf("commit: %w", err)
	}
	return wf, toApprove, nil
}
