package store

import (
	"context"
	"fmt"
	"time"
)

// RecoverRepoAnalysisRunsAfterRestart 功能：daemon 启动时将遗留的 queued/running repo analysis run 标记为 failed。
// 参数/返回：ctx 控制超时；返回被修正的 run 数量与错误信息。
// 失败场景：查询或更新失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_runs`。
func (s *Store) RecoverRepoAnalysisRunsAfterRestart(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	now := time.Now().UnixMilli()
	reason := "daemon_restarted"
	res, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_runs SET status = 'failed', error_message = ?, ended_at = COALESCE(ended_at, ?), updated_at = ? WHERE status IN ('queued', 'running');`, reason, now, now)
	if err != nil {
		return 0, fmt.Errorf("recover repo analysis runs: %w", err)
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}
