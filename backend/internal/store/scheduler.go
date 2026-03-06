package store

import (
	"context"
	"fmt"
)

type RunnableNode struct {
	Node
	WorkspacePath string `json:"workspace_path"`
}

// CountRunningWorkerNodes 功能：统计当前处于 running 的 worker nodes 数量（用于全局并发控制）。
// 参数/返回：返回 running worker node 数量与错误信息。
// 失败场景：SQL 查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) CountRunningWorkerNodes(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}

	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM nodes WHERE node_type != 'master' AND status = 'running';`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count running worker nodes: %w", err)
	}
	var agentRuns int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM agent_runs WHERE status = 'running';`).Scan(&agentRuns); err != nil {
		return 0, fmt.Errorf("count running agent runs: %w", err)
	}
	return n + agentRuns, nil
}

// ListRunnableQueuedWorkerNodes 功能：列出当前可运行的 queued worker nodes（依赖全部 succeeded，且 workflow.status=running）。
// 参数/返回：limit 为最大条数；返回包含 workspace_path 的 RunnableNode 列表。
// 失败场景：SQL 查询或扫描失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRunnableQueuedWorkerNodes(ctx context.Context, limit int) ([]RunnableNode, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT n.id, n.workflow_id, n.node_type, n.expert_id, n.title, n.prompt, n.status,
		        n.created_at, n.updated_at, n.last_execution_id, n.result_summary, n.result_json, n.error_message,
		        w.workspace_path
		   FROM nodes n
		   JOIN workflows w ON w.id = n.workflow_id
		  WHERE n.node_type != 'master'
		    AND n.status = 'queued'
		    AND w.status = 'running'
		    AND NOT EXISTS (
		      SELECT 1
		        FROM edges e
		        JOIN nodes dep ON dep.id = e.from_node_id
		       WHERE e.workflow_id = n.workflow_id
		         AND e.to_node_id = n.id
		         AND e.type = 'success'
		         AND dep.status != 'succeeded'
		    )
		  ORDER BY n.created_at ASC
		  LIMIT ?;`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query runnable queued nodes: %w", err)
	}
	defer rows.Close()

	out := make([]RunnableNode, 0)
	for rows.Next() {
		var r RunnableNode
		if err := rows.Scan(
			&r.ID,
			&r.WorkflowID,
			&r.NodeType,
			&r.ExpertID,
			&r.Title,
			&r.Prompt,
			&r.Status,
			&r.CreatedAt,
			&r.UpdatedAt,
			&r.LastExecution,
			&r.ResultSummary,
			&r.ResultJSON,
			&r.ErrorMessage,
			&r.WorkspacePath,
		); err != nil {
			return nil, fmt.Errorf("scan runnable node: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runnable nodes: %w", err)
	}
	return out, nil
}
