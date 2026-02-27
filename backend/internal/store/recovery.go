package store

import (
	"context"
	"fmt"
	"time"
)

type runningExecutionRow struct {
	executionID string
	nodeID      string
	workflowID  string
}

// RecoverAfterRestart 功能：daemon 启动时扫描 DB，将遗留的 running executions 标记为 failed（原因：daemon_restarted），并同步修正 node/workflow 状态。
// 参数/返回：ctx 控制超时；返回被修正的 execution 数量与错误信息。
// 失败场景：查询或更新失败时返回 error。
// 副作用：写入 SQLite executions/nodes/workflows/events。
func (s *Store) RecoverAfterRestart(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT e.id, e.node_id, n.workflow_id
		   FROM executions e
		   JOIN nodes n ON n.id = e.node_id
		  WHERE e.status = 'running';`,
	)
	if err != nil {
		return 0, fmt.Errorf("query running executions: %w", err)
	}
	defer rows.Close()

	var running []runningExecutionRow
	for rows.Next() {
		var r runningExecutionRow
		if err := rows.Scan(&r.executionID, &r.nodeID, &r.workflowID); err != nil {
			return 0, fmt.Errorf("scan running execution: %w", err)
		}
		running = append(running, r)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate running executions: %w", err)
	}
	if len(running) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	reason := "daemon_restarted"

	workflowTouched := make(map[string]struct{})
	nodeTouched := make(map[string]struct{})

	updated := 0
	for _, r := range running {
		res, err := tx.ExecContext(
			ctx,
			`UPDATE executions
			    SET status = 'failed',
			        ended_at = ?,
			        error_message = ?
			  WHERE id = ? AND status = 'running';`,
			now, reason, r.executionID,
		)
		if err != nil {
			return updated, fmt.Errorf("update execution %s: %w", r.executionID, err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			continue
		}
		updated++

		nodeTouched[r.nodeID] = struct{}{}
		workflowTouched[r.workflowID] = struct{}{}

		if err := insertEvent(ctx, tx, r.workflowID, "execution", r.executionID, "execution.exited", now, map[string]any{
			"status": "failed",
			"reason": reason,
		}); err != nil {
			return updated, err
		}
		if err := insertEvent(ctx, tx, r.workflowID, "node", r.nodeID, "node.updated", now, map[string]any{
			"action": "daemon_restarted",
			"status": "failed",
		}); err != nil {
			return updated, err
		}
	}

	for nodeID := range nodeTouched {
		if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status = 'failed', updated_at = ?, error_message = ? WHERE id = ?;`, now, reason, nodeID); err != nil {
			return updated, fmt.Errorf("update node %s: %w", nodeID, err)
		}
	}

	for workflowID := range workflowTouched {
		if _, err := tx.ExecContext(ctx, `UPDATE workflows SET status = ?, updated_at = ?, error_message = ? WHERE id = ?;`, string(WorkflowStatusFailed), now, reason, workflowID); err != nil {
			return updated, fmt.Errorf("update workflow %s: %w", workflowID, err)
		}
		if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", now, map[string]any{
			"action": "daemon_restarted",
			"status": string(WorkflowStatusFailed),
		}); err != nil {
			return updated, err
		}
	}

	if err := tx.Commit(); err != nil {
		return updated, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}
