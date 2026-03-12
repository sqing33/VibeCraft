package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MarkRepoAnalysisStartedParams struct {
	AnalysisID string
	Summary    *string
	StartedAt  int64
}

type UpdateRepoAnalysisChatBindingParams struct {
	AnalysisID             string
	ChatSessionID          *string
	ChatUserMessageID      *string
	ChatAssistantMessageID *string
	RuntimeKind            *string
	CLIToolID              *string
	ModelID                *string
	Summary                *string
}

// MarkRepoAnalysisStarted 功能：在无 execution_id 的场景下将 Repo analysis 标记为 running。
// 参数/返回：params 指定 analysis id、可选摘要与 started_at；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) MarkRepoAnalysisStarted(ctx context.Context, params MarkRepoAnalysisStartedParams) (RepoAnalysisResult, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisResult{}, fmt.Errorf("store not initialized")
	}
	analysisID := strings.TrimSpace(params.AnalysisID)
	if analysisID == "" {
		return RepoAnalysisResult{}, fmt.Errorf("%w: analysis_id is required", ErrValidation)
	}
	result, err := s.GetRepoAnalysisResult(ctx, analysisID)
	if err != nil {
		return RepoAnalysisResult{}, err
	}
	startedAt := params.StartedAt
	if startedAt <= 0 {
		startedAt = time.Now().UnixMilli()
	}
	result.Status = string(RepoAnalysisStatusRunning)
	result.StartedAt = &startedAt
	result.UpdatedAt = startedAt
	if params.Summary != nil {
		result.Summary = trimOrNil(params.Summary)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_results SET status = ?, summary = ?, started_at = ?, updated_at = ? WHERE id = ?;`, result.Status, result.Summary, result.StartedAt, result.UpdatedAt, result.ID); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("mark repo analysis started: %w", err)
	}
	return result, nil
}

// UpdateRepoAnalysisChatBinding 功能：为 Repo analysis 记录关联 chat session 与 runtime/tool/model 元数据。
// 参数/返回：params 指定 analysis id 与关联字段；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) UpdateRepoAnalysisChatBinding(ctx context.Context, params UpdateRepoAnalysisChatBindingParams) (RepoAnalysisResult, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisResult{}, fmt.Errorf("store not initialized")
	}
	analysisID := strings.TrimSpace(params.AnalysisID)
	if analysisID == "" {
		return RepoAnalysisResult{}, fmt.Errorf("%w: analysis_id is required", ErrValidation)
	}
	result, err := s.GetRepoAnalysisResult(ctx, analysisID)
	if err != nil {
		return RepoAnalysisResult{}, err
	}
	if params.ChatSessionID != nil {
		result.ChatSessionID = trimOrNil(params.ChatSessionID)
	}
	if params.ChatUserMessageID != nil {
		result.ChatUserMessageID = trimOrNil(params.ChatUserMessageID)
	}
	if params.ChatAssistantMessageID != nil {
		result.ChatAssistantMessageID = trimOrNil(params.ChatAssistantMessageID)
	}
	if params.RuntimeKind != nil {
		result.RuntimeKind = trimOrNil(params.RuntimeKind)
	}
	if params.CLIToolID != nil {
		result.CLIToolID = trimOrNil(params.CLIToolID)
	}
	if params.ModelID != nil {
		result.ModelID = trimOrNil(params.ModelID)
	}
	if params.Summary != nil {
		result.Summary = trimOrNil(params.Summary)
	}
	result.UpdatedAt = time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_results SET chat_session_id = ?, chat_user_message_id = ?, chat_assistant_message_id = ?, runtime_kind = ?, cli_tool_id = ?, model_id = ?, summary = ?, updated_at = ? WHERE id = ?;`, result.ChatSessionID, result.ChatUserMessageID, result.ChatAssistantMessageID, result.RuntimeKind, result.CLIToolID, result.ModelID, result.Summary, result.UpdatedAt, result.ID); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("update repo analysis chat binding: %w", err)
	}
	return result, nil
}

