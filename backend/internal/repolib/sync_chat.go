package repolib

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"vibe-tree/backend/internal/store"
)

// SyncAnalysisFromChat 功能：将关联分析 chat 的最新 assistant 回复同步回 Repo Library report/cards/search。
// 参数/返回：runID 为分析运行 id；返回更新后的 RepoAnalysisRun。
// 失败场景：run 不存在、未绑定 chat session、无 assistant 消息或后处理失败时返回 error。
// 副作用：写入 report.md、更新 SQLite 分析记录，并刷新 cards/search 索引。
func (s *Service) SyncAnalysisFromChat(ctx context.Context, runID string) (store.RepoAnalysisRun, error) {
	if s == nil || s.store == nil {
		return store.RepoAnalysisRun{}, fmt.Errorf("repo library service not configured")
	}
	run, err := s.store.GetRepoAnalysisRun(ctx, runID)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	if run.ChatSessionID == nil || *run.ChatSessionID == "" {
		return store.RepoAnalysisRun{}, fmt.Errorf("%w: analysis run is not linked to a chat session", store.ErrValidation)
	}
	snapshot, err := s.store.GetRepoSnapshot(ctx, run.RepoSnapshotID)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	source, err := s.store.GetRepoSource(ctx, run.RepoSourceID)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	messages, err := s.store.ListChatMessages(ctx, *run.ChatSessionID, 1000)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	var latestAssistant *store.ChatMessage
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			latestAssistant = &messages[i]
			break
		}
	}
	if latestAssistant == nil {
		return store.RepoAnalysisRun{}, fmt.Errorf("%w: no assistant message found in linked chat session", store.ErrValidation)
	}
	reportText := sanitizeReportMarkdown(latestAssistant.ContentText)
	if reportText == "" {
		return store.RepoAnalysisRun{}, fmt.Errorf("%w: latest assistant message is empty", store.ErrValidation)
	}
	layout, err := s.prepareSnapshotLayoutForID(source.RepoKey, snapshot.ID)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	session, err := s.store.GetChatSession(ctx, *run.ChatSessionID)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	resolved, err := s.resolveAnalysisTurnRuntime(session, run)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	prepared := analysisPrepareResult{
		ResolvedRef:   firstNonEmpty(pointerValue(snapshot.ResolvedRef), snapshot.RequestedRef),
		CommitSHA:     pointerValue(snapshot.CommitSHA),
		CodeIndexPath: filepath.Join(layout.ArtifactsDir, "code_index.json"),
		SnapshotDir:   layout.SnapshotDir,
		SourceDir:     session.WorkspacePath,
		ReportPath:    layout.ReportPath,
	}
	validated, err := s.validateAndFinalizeFormalReport(ctx, source, snapshot, run, prepared, layout, session, resolved, reportCandidate{
		AssistantMessageID: latestAssistant.ID,
		Markdown:           reportText,
	})
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	cardsCount, evidenceCount, refreshSummary, err := s.postProcessAIReport(ctx, source, snapshot, run, layout)
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	resultPayload := map[string]any{
		"runtime_kind":                firstNonEmptyStringPtr(run.RuntimeKind, "ai_chat"),
		"chat_session_id":             run.ChatSessionID,
		"synced_from_chat_message_id": validated.Candidate.AssistantMessageID,
		"report_path":                 layout.ReportPath,
		"report_validation_path":      filepath.Join(layout.DerivedDir, "report.validation.json"),
		"report_attempts":             validated.Attempts,
		"cards_path":                  layout.CardsPath,
		"card_count":                  cardsCount,
		"evidence_count":              evidenceCount,
		"search_refresh":              refreshSummary,
		"synced_at":                   time.Now().UnixMilli(),
	}
	resultJSON, _ := json.Marshal(resultPayload)
	summary := fmt.Sprintf("已从 Chat 最新回复同步分析结果：正式报告已通过校验，生成 %d 张卡片、%d 条证据。", cardsCount, evidenceCount)
	run, err = s.store.FinalizeRepoAnalysisRun(ctx, store.FinalizeRepoAnalysisRunParams{RunID: run.ID, Status: string(store.RepoAnalysisStatusSucceeded), Summary: &summary, ResultJSON: pointer(string(resultJSON)), ReportPath: &layout.ReportPath})
	if err != nil {
		return store.RepoAnalysisRun{}, err
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{RunID: run.ID, ChatSessionID: run.ChatSessionID, ChatUserMessageID: validated.Candidate.UserMessageID, ChatAssistantMessageID: stringPtrIfNotEmpty(validated.Candidate.AssistantMessageID), Summary: &summary})
	return s.store.GetRepoAnalysisRun(ctx, run.ID)
}

func firstNonEmptyStringPtr(value *string, fallback string) string {
	if value != nil && *value != "" {
		return *value
	}
	return fallback
}
