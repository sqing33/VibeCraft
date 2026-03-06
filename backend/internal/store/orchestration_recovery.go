package store

import (
	"context"
	"fmt"
	"time"
)

// RecoverOrchestrationsAfterRestart 功能：daemon 启动时扫描遗留的 running agent_run_executions，并标记为 failed。
// 参数/返回：ctx 控制超时；返回被修正的 agent run execution 数量与错误信息。
// 失败场景：查询或更新失败时返回 error。
// 副作用：写入 SQLite agent_run_executions/agent_runs/orchestration_rounds/orchestrations/orchestration_events。
func (s *Store) RecoverOrchestrationsAfterRestart(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.id, e.agent_run_id, r.round_id, r.orchestration_id FROM agent_run_executions e JOIN agent_runs r ON r.id = e.agent_run_id WHERE e.status = 'running';`)
	if err != nil {
		return 0, fmt.Errorf("query running agent_run_executions: %w", err)
	}
	defer rows.Close()
	type row struct{ executionID, agentRunID, roundID, orchestrationID string }
	running := make([]row, 0)
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.executionID, &item.agentRunID, &item.roundID, &item.orchestrationID); err != nil {
			return 0, fmt.Errorf("scan running agent_run_execution: %w", err)
		}
		running = append(running, item)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate running agent_run_executions: %w", err)
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
	updated := 0
	touchedRounds := map[string]string{}
	touchedOrchestrations := map[string]struct{}{}
	for _, item := range running {
		res, err := tx.ExecContext(ctx, `UPDATE agent_run_executions SET status = 'failed', ended_at = ?, error_message = ? WHERE id = ? AND status = 'running';`, now, reason, item.executionID)
		if err != nil {
			return updated, fmt.Errorf("update agent_run_execution %s: %w", item.executionID, err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			continue
		}
		updated++
		if _, err := tx.ExecContext(ctx, `UPDATE agent_runs SET status = 'failed', updated_at = ?, error_message = ? WHERE id = ?;`, now, reason, item.agentRunID); err != nil {
			return updated, fmt.Errorf("update agent run %s: %w", item.agentRunID, err)
		}
		touchedRounds[item.roundID] = item.orchestrationID
		touchedOrchestrations[item.orchestrationID] = struct{}{}
		if err := insertOrchestrationEvent(ctx, tx, item.orchestrationID, "agent_run", item.agentRunID, "orchestration.agent_run.updated", now, map[string]any{"action": "daemon_restarted", "status": "failed"}); err != nil {
			return updated, err
		}
	}
	for roundID, orchestrationID := range touchedRounds {
		if _, err := tx.ExecContext(ctx, `UPDATE orchestration_rounds SET status = ?, updated_at = ? WHERE id = ?;`, string(RoundStatusRetryable), now, roundID); err != nil {
			return updated, fmt.Errorf("update orchestration round %s: %w", roundID, err)
		}
		if err := insertOrchestrationEvent(ctx, tx, orchestrationID, "round", roundID, "orchestration.round.updated", now, map[string]any{"action": "daemon_restarted", "status": string(RoundStatusRetryable)}); err != nil {
			return updated, err
		}
	}
	for orchestrationID := range touchedOrchestrations {
		if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET status = ?, updated_at = ?, error_message = ? WHERE id = ?;`, string(OrchestrationStatusFailed), now, reason, orchestrationID); err != nil {
			return updated, fmt.Errorf("update orchestration %s: %w", orchestrationID, err)
		}
		if err := insertOrchestrationEvent(ctx, tx, orchestrationID, "orchestration", orchestrationID, "orchestration.updated", now, map[string]any{"action": "daemon_restarted", "status": string(OrchestrationStatusFailed)}); err != nil {
			return updated, err
		}
	}
	if err := tx.Commit(); err != nil {
		return updated, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}
