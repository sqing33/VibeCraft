package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"vibe-tree/backend/internal/id"
)

type Node struct {
	ID            string  `json:"node_id"`
	WorkflowID    string  `json:"workflow_id"`
	NodeType      string  `json:"node_type"`
	ExpertID      string  `json:"expert_id"`
	Title         string  `json:"title"`
	Prompt        string  `json:"prompt"`
	Status        string  `json:"status"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
	LastExecution *string `json:"last_execution_id,omitempty"`
	ResultSummary *string `json:"result_summary,omitempty"`
	ResultJSON    *string `json:"result_json,omitempty"`
	ErrorMessage  *string `json:"error_message,omitempty"`
}

type StartWorkflowMasterParams struct {
	ExpertID string
	Title    string
	Prompt   string
}

// StartWorkflowMaster 功能：为指定 workflow 创建 master node，并将 workflow 状态置为 running（MVP：一个 workflow 仅允许一个 master node）。
// 参数/返回：workflowID 为目标；params 可选覆盖 expert/title/prompt；返回更新后的 Workflow 与新建 Node。
// 失败场景：workflow 不存在返回 os.ErrNotExist；已存在 master node 或 workflow 已在 running 返回 ErrConflict；写库失败返回 error。
// 副作用：写入 SQLite workflows/nodes/events。
func (s *Store) StartWorkflowMaster(ctx context.Context, workflowID string, params StartWorkflowMasterParams) (Workflow, Node, error) {
	if s == nil || s.db == nil {
		return Workflow{}, Node{}, fmt.Errorf("store not initialized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, Node{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wf, err := getWorkflowTx(ctx, tx, workflowID)
	if err != nil {
		return Workflow{}, Node{}, err
	}
	if wf.Status == string(WorkflowStatusRunning) {
		return Workflow{}, Node{}, fmt.Errorf("%w: workflow already running", ErrConflict)
	}

	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM nodes WHERE workflow_id = ? AND node_type = 'master' LIMIT 1;`, workflowID).Scan(&existing)
	if err == nil {
		return Workflow{}, Node{}, fmt.Errorf("%w: master node already exists", ErrConflict)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Workflow{}, Node{}, fmt.Errorf("query existing master node: %w", err)
	}

	now := time.Now().UnixMilli()
	node := Node{
		ID:         id.New("nd_"),
		WorkflowID: workflowID,
		NodeType:   "master",
		ExpertID:   params.ExpertID,
		Title:      params.Title,
		Prompt:     params.Prompt,
		Status:     "running",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if node.ExpertID == "" {
		node.ExpertID = "bash"
	}
	if node.Title == "" {
		node.Title = "Master"
	}
	if node.Prompt == "" {
		node.Prompt = "vibe-tree master (stub)"
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO nodes (
			id, workflow_id, node_type, expert_id, title, prompt, status,
			created_at, updated_at,
			last_execution_id, result_summary, result_json, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL);`,
		node.ID, node.WorkflowID, node.NodeType, node.ExpertID, node.Title, node.Prompt, node.Status,
		node.CreatedAt, node.UpdatedAt,
	)
	if err != nil {
		return Workflow{}, Node{}, fmt.Errorf("insert master node: %w", err)
	}

	wf.Status = string(WorkflowStatusRunning)
	wf.UpdatedAt = now
	res, err := tx.ExecContext(ctx, `UPDATE workflows SET status = ?, updated_at = ? WHERE id = ?;`, wf.Status, wf.UpdatedAt, workflowID)
	if err != nil {
		return Workflow{}, Node{}, fmt.Errorf("update workflow status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return Workflow{}, Node{}, os.ErrNotExist
	}

	if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", now, map[string]any{
		"action": "started",
		"status": wf.Status,
	}); err != nil {
		return Workflow{}, Node{}, err
	}
	if err := insertEvent(ctx, tx, workflowID, "node", node.ID, "node.updated", now, map[string]any{
		"action":  "created",
		"node_id": node.ID,
		"status":  node.Status,
	}); err != nil {
		return Workflow{}, Node{}, err
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, Node{}, fmt.Errorf("commit: %w", err)
	}
	return wf, node, nil
}

// ListNodes 功能：列出某个 workflow 下的 nodes（按创建时间升序）。
// 参数/返回：workflowID 为目标；返回 Node 列表。
// 失败场景：查询失败或扫描失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListNodes(ctx context.Context, workflowID string) ([]Node, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, workflow_id, node_type, expert_id, title, prompt, status,
		        created_at, updated_at, last_execution_id, result_summary, result_json, error_message
		   FROM nodes
		  WHERE workflow_id = ?
		  ORDER BY created_at ASC;`,
		workflowID,
	)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()

	var out []Node
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
			return nil, fmt.Errorf("scan node: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return out, nil
}
