package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type UpdateNodeParams struct {
	Prompt   *string
	ExpertID *string
}

// UpdateNode 功能：更新 node 的 prompt/expert_id（用于 manual approval 期间编辑）。
// 参数/返回：nodeID 为目标；patch 为可选字段；返回更新后的 Node。
// 失败场景：node 不存在返回 os.ErrNotExist；node 状态不允许修改返回 ErrConflict；参数非法或写库失败返回 error。
// 副作用：写入 SQLite nodes/workflows/events。
func (s *Store) UpdateNode(ctx context.Context, nodeID string, patch UpdateNodeParams) (Node, error) {
	if s == nil || s.db == nil {
		return Node{}, fmt.Errorf("store not initialized")
	}
	if nodeID == "" {
		return Node{}, fmt.Errorf("%w: node_id is required", ErrValidation)
	}
	if patch.Prompt == nil && patch.ExpertID == nil {
		return Node{}, fmt.Errorf("%w: empty patch", ErrValidation)
	}
	if patch.Prompt != nil && strings.TrimSpace(*patch.Prompt) == "" {
		return Node{}, fmt.Errorf("%w: prompt is required", ErrValidation)
	}
	if patch.ExpertID != nil && strings.TrimSpace(*patch.ExpertID) == "" {
		return Node{}, fmt.Errorf("%w: expert_id is required", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var n Node
	err = tx.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
		        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
		   FROM nodes
		  WHERE id = ?
		  LIMIT 1;`,
		nodeID,
	).Scan(
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
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, os.ErrNotExist
		}
		return Node{}, fmt.Errorf("query node: %w", err)
	}

	switch n.Status {
	case "draft", "pending_approval", "queued":
		// ok
	default:
		return Node{}, fmt.Errorf("%w: node status %q not editable", ErrConflict, n.Status)
	}

	oldPrompt := n.Prompt
	oldExpert := n.ExpertID

	if patch.Prompt != nil {
		n.Prompt = *patch.Prompt
	}
	if patch.ExpertID != nil {
		n.ExpertID = *patch.ExpertID
	}
	now := time.Now().UnixMilli()
	n.UpdatedAt = now

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE nodes
		    SET prompt = ?, expert_id = ?, updated_at = ?
		  WHERE id = ?;`,
		n.Prompt, n.ExpertID, n.UpdatedAt, n.ID,
	); err != nil {
		return Node{}, fmt.Errorf("update node: %w", err)
	}

	if patch.Prompt != nil && oldPrompt != n.Prompt {
		if err := insertEvent(ctx, tx, n.WorkflowID, "node", n.ID, "prompt.updated", now, map[string]any{
			"node_id":     n.ID,
			"old_prompt":  truncateForEvent(oldPrompt),
			"new_prompt":  truncateForEvent(n.Prompt),
			"old_expert":  oldExpert,
			"new_expert":  n.ExpertID,
			"changed_by":  "user",
			"change_type": "patch",
		}); err != nil {
			return Node{}, err
		}
	}

	fields := make([]string, 0, 2)
	if patch.Prompt != nil && oldPrompt != n.Prompt {
		fields = append(fields, "prompt")
	}
	if patch.ExpertID != nil && oldExpert != n.ExpertID {
		fields = append(fields, "expert_id")
	}
	if err := insertEvent(ctx, tx, n.WorkflowID, "node", n.ID, "node.updated", now, map[string]any{
		"action":  "patched",
		"node_id": n.ID,
		"fields":  fields,
	}); err != nil {
		return Node{}, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET updated_at = ? WHERE id = ?;`, now, n.WorkflowID); err != nil {
		return Node{}, fmt.Errorf("touch workflow updated_at: %w", err)
	}
	if err := insertEvent(ctx, tx, n.WorkflowID, "workflow", n.WorkflowID, "workflow.updated", now, map[string]any{
		"action": "node.patched",
	}); err != nil {
		return Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("commit: %w", err)
	}
	return n, nil
}

func truncateForEvent(s string) string {
	const max = 180
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
