package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"
)

// MarkNodeAndWorkflowFailed 功能：在启动/落库失败等不可恢复场景下，将 node 与 workflow 标记为 failed，并记录 error_message。
// 参数/返回：workflowID/nodeID 为目标；message 为错误信息；成功返回 nil。
// 失败场景：workflow/node 不存在返回 os.ErrNotExist；写库失败返回 error。
// 副作用：写入 SQLite workflows/nodes/events。
func (s *Store) MarkNodeAndWorkflowFailed(ctx context.Context, workflowID, nodeID, message string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if workflowID == "" || nodeID == "" {
		return fmt.Errorf("%w: missing workflow/node id", ErrValidation)
	}
	if message == "" {
		message = "unknown error"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := getWorkflowTx(ctx, tx, workflowID); err != nil {
		return err
	}
	var exists string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM nodes WHERE id = ? LIMIT 1;`, nodeID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return os.ErrNotExist
		}
		return fmt.Errorf("query node: %w", err)
	}

	now := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET status = ?, updated_at = ?, error_message = ? WHERE id = ?;`, string(WorkflowStatusFailed), now, message, workflowID); err != nil {
		return fmt.Errorf("update workflow failed: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status = ?, updated_at = ?, error_message = ? WHERE id = ?;`, "failed", now, message, nodeID); err != nil {
		return fmt.Errorf("update node failed: %w", err)
	}

	if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", now, map[string]any{
		"action": "failed",
		"status": string(WorkflowStatusFailed),
		"error":  message,
	}); err != nil {
		return err
	}
	if err := insertEvent(ctx, tx, workflowID, "node", nodeID, "node.updated", now, map[string]any{
		"action": "failed",
		"status": "failed",
		"error":  message,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
