package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MarkRepoAnalysisRunStartedParams struct {
	RunID     string
	Summary   *string
	StartedAt int64
}

type UpdateRepoAnalysisChatBindingParams struct {
	RunID                 string
	ChatSessionID         *string
	ChatUserMessageID     *string
	ChatAssistantMessageID *string
	RuntimeKind           *string
	CLIToolID             *string
	ModelID               *string
	Summary               *string
}

// MarkRepoAnalysisRunStarted 功能：在无 execution_id 的场景下将 Repo analysis run 标记为 running。
// 参数/返回：params 指定 run id、可选摘要与 started_at；返回更新后的 RepoAnalysisRun。
// 失败场景：run 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_runs`。
func (s *Store) MarkRepoAnalysisRunStarted(ctx context.Context, params MarkRepoAnalysisRunStartedParams) (RepoAnalysisRun, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisRun{}, fmt.Errorf("store not initialized")
	}
	runID := strings.TrimSpace(params.RunID)
	if runID == "" {
		return RepoAnalysisRun{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}
	run, err := s.GetRepoAnalysisRun(ctx, runID)
	if err != nil {
		return RepoAnalysisRun{}, err
	}
	startedAt := params.StartedAt
	if startedAt <= 0 {
		startedAt = time.Now().UnixMilli()
	}
	run.Status = string(RepoAnalysisStatusRunning)
	run.StartedAt = &startedAt
	run.UpdatedAt = startedAt
	if params.Summary != nil {
		run.Summary = trimOrNil(params.Summary)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_runs SET status = ?, summary = ?, started_at = ?, updated_at = ? WHERE id = ?;`, run.Status, run.Summary, run.StartedAt, run.UpdatedAt, run.ID); err != nil {
		return RepoAnalysisRun{}, fmt.Errorf("mark repo analysis run started: %w", err)
	}
	return run, nil
}

// UpdateRepoAnalysisChatBinding 功能：为 Repo analysis run 记录关联 chat session 与 runtime/tool/model 元数据。
// 参数/返回：params 指定 run id 与关联字段；返回更新后的 RepoAnalysisRun。
// 失败场景：run 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_runs`。
func (s *Store) UpdateRepoAnalysisChatBinding(ctx context.Context, params UpdateRepoAnalysisChatBindingParams) (RepoAnalysisRun, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisRun{}, fmt.Errorf("store not initialized")
	}
	runID := strings.TrimSpace(params.RunID)
	if runID == "" {
		return RepoAnalysisRun{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}
	run, err := s.GetRepoAnalysisRun(ctx, runID)
	if err != nil {
		return RepoAnalysisRun{}, err
	}
	if params.ChatSessionID != nil {
		run.ChatSessionID = trimOrNil(params.ChatSessionID)
	}
	if params.ChatUserMessageID != nil {
		run.ChatUserMessageID = trimOrNil(params.ChatUserMessageID)
	}
	if params.ChatAssistantMessageID != nil {
		run.ChatAssistantMessageID = trimOrNil(params.ChatAssistantMessageID)
	}
	if params.RuntimeKind != nil {
		run.RuntimeKind = trimOrNil(params.RuntimeKind)
	}
	if params.CLIToolID != nil {
		run.CLIToolID = trimOrNil(params.CLIToolID)
	}
	if params.ModelID != nil {
		run.ModelID = trimOrNil(params.ModelID)
	}
	if params.Summary != nil {
		run.Summary = trimOrNil(params.Summary)
	}
	run.UpdatedAt = time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_runs SET chat_session_id = ?, chat_user_message_id = ?, chat_assistant_message_id = ?, runtime_kind = ?, cli_tool_id = ?, model_id = ?, summary = ?, updated_at = ? WHERE id = ?;`, run.ChatSessionID, run.ChatUserMessageID, run.ChatAssistantMessageID, run.RuntimeKind, run.CLIToolID, run.ModelID, run.Summary, run.UpdatedAt, run.ID); err != nil {
		return RepoAnalysisRun{}, fmt.Errorf("update repo analysis chat binding: %w", err)
	}
	return run, nil
}
