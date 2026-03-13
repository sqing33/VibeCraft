package store

import (
	"context"
	"fmt"
	"time"

	"vibecraft/backend/internal/dag"
	"vibecraft/backend/internal/id"
)

type Edge struct {
	ID           string  `json:"edge_id"`
	WorkflowID   string  `json:"workflow_id"`
	FromNodeID   string  `json:"from_node_id"`
	ToNodeID     string  `json:"to_node_id"`
	SourceHandle *string `json:"source_handle,omitempty"`
	TargetHandle *string `json:"target_handle,omitempty"`
	Type         string  `json:"type"`
}

// ApplyDAG 功能：将 master 输出的 DAG 落库为 worker nodes 与 edges，并写入 `dag.generated` 审计事件。
// 参数/返回：workflowID 为目标；d 为已校验 DAG；返回创建的 nodes/edges 与更新后的 workflow。
// 失败场景：workflow 不存在返回 os.ErrNotExist；已存在 worker nodes 返回 ErrConflict；写库失败返回 error。
// 副作用：写入 SQLite nodes/edges/workflows/events。
func (s *Store) ApplyDAG(ctx context.Context, workflowID string, d dag.DAG) (Workflow, []Node, []Edge, error) {
	if s == nil || s.db == nil {
		return Workflow{}, nil, nil, fmt.Errorf("store not initialized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workflow{}, nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wf, err := getWorkflowTx(ctx, tx, workflowID)
	if err != nil {
		return Workflow{}, nil, nil, err
	}

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM nodes WHERE workflow_id = ? AND node_type != 'master';`, workflowID).Scan(&existing); err != nil {
		return Workflow{}, nil, nil, fmt.Errorf("count existing worker nodes: %w", err)
	}
	if existing > 0 {
		return Workflow{}, nil, nil, fmt.Errorf("%w: dag already applied", ErrConflict)
	}

	now := time.Now().UnixMilli()

	// 可选：让 master 输出覆盖 workflow title（MVP：有值则直接写）。
	if d.WorkflowTitle != "" && d.WorkflowTitle != wf.Title {
		wf.Title = d.WorkflowTitle
		wf.UpdatedAt = now
		if _, err := tx.ExecContext(ctx, `UPDATE workflows SET title = ?, updated_at = ? WHERE id = ?;`, wf.Title, wf.UpdatedAt, workflowID); err != nil {
			return Workflow{}, nil, nil, fmt.Errorf("update workflow title: %w", err)
		}
		if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "workflow.updated", now, map[string]any{
			"action": "title.overridden",
		}); err != nil {
			return Workflow{}, nil, nil, err
		}
	}

	nodeStatus := "pending_approval"
	if wf.Mode == string(WorkflowModeAuto) {
		nodeStatus = "queued"
	}

	// Mapping DAG node ID -> internal node ID.
	idMap := make(map[string]string, len(d.Nodes))
	createdNodes := make([]Node, 0, len(d.Nodes))
	for _, dn := range d.Nodes {
		newID := id.New("nd_")
		idMap[dn.ID] = newID

		n := Node{
			ID:         newID,
			WorkflowID: workflowID,
			NodeType:   "worker",
			ExpertID:   dn.ExpertID,
			Title:      dn.Title,
			Prompt:     dn.Prompt,
			Status:     nodeStatus,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if n.Title == "" {
			n.Title = dn.ID
		}

		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO nodes (
				id, workflow_id, node_type, expert_id, title, prompt, status,
				created_at, updated_at,
				last_execution_id, result_summary, result_json, error_message
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL);`,
			n.ID, n.WorkflowID, n.NodeType, n.ExpertID, n.Title, n.Prompt, n.Status, n.CreatedAt, n.UpdatedAt,
		)
		if err != nil {
			return Workflow{}, nil, nil, fmt.Errorf("insert worker node: %w", err)
		}

		if err := insertEvent(ctx, tx, workflowID, "node", n.ID, "node.updated", now, map[string]any{
			"action":  "created",
			"node_id": n.ID,
			"status":  n.Status,
		}); err != nil {
			return Workflow{}, nil, nil, err
		}

		createdNodes = append(createdNodes, n)
	}

	createdEdges := make([]Edge, 0, len(d.Edges))
	for _, de := range d.Edges {
		fromID, ok1 := idMap[de.From]
		toID, ok2 := idMap[de.To]
		if !ok1 || !ok2 {
			return Workflow{}, nil, nil, fmt.Errorf("%w: edge references unknown node", ErrValidation)
		}
		edgeType := de.Type
		if edgeType == "" {
			edgeType = "success"
		}
		e := Edge{
			ID:           id.New("ed_"),
			WorkflowID:   workflowID,
			FromNodeID:   fromID,
			ToNodeID:     toID,
			SourceHandle: de.SourceHandle,
			TargetHandle: de.TargetHandle,
			Type:         edgeType,
		}

		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO edges (id, workflow_id, from_node_id, to_node_id, source_handle, target_handle, type)
			 VALUES (?, ?, ?, ?, ?, ?, ?);`,
			e.ID, e.WorkflowID, e.FromNodeID, e.ToNodeID, e.SourceHandle, e.TargetHandle, e.Type,
		)
		if err != nil {
			return Workflow{}, nil, nil, fmt.Errorf("insert edge: %w", err)
		}
		createdEdges = append(createdEdges, e)
	}

	if err := insertEvent(ctx, tx, workflowID, "workflow", workflowID, "dag.generated", now, map[string]any{
		"nodes": len(createdNodes),
		"edges": len(createdEdges),
	}); err != nil {
		return Workflow{}, nil, nil, err
	}

	// 保底更新 updated_at，确保 UI 列表刷新。
	wf.UpdatedAt = now
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET updated_at = ? WHERE id = ?;`, wf.UpdatedAt, workflowID); err != nil {
		return Workflow{}, nil, nil, fmt.Errorf("touch workflow updated_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Workflow{}, nil, nil, fmt.Errorf("commit: %w", err)
	}
	return wf, createdNodes, createdEdges, nil
}

// ListEdges 功能：列出 workflow 下的 edges（按创建顺序/插入顺序返回）。
// 参数/返回：workflowID 为目标；返回 Edge 列表。
// 失败场景：查询失败或扫描失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListEdges(ctx context.Context, workflowID string) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, workflow_id, from_node_id, to_node_id, source_handle, target_handle, type
		   FROM edges
		  WHERE workflow_id = ?
		  ORDER BY rowid ASC;`,
		workflowID,
	)
	if err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}
	defer rows.Close()

	out := make([]Edge, 0)
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.WorkflowID, &e.FromNodeID, &e.ToNodeID, &e.SourceHandle, &e.TargetHandle, &e.Type); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edges: %w", err)
	}
	return out, nil
}
