package repolib

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"vibe-tree/backend/internal/store"
)

// SyncAnalysisFromChat 功能：将关联分析 Chat 的最新 assistant 回复同步回 Repo Library report/cards/search。
// 参数/返回：analysisID 为分析结果 id；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在、未绑定 chat session、无 assistant 消息或后处理失败时返回 error。
// 副作用：写入 report.md、更新 SQLite 分析记录，并刷新 cards/search 索引。
func (s *Service) SyncAnalysisFromChat(ctx context.Context, analysisID string) (store.RepoAnalysisResult, error) {
	if s == nil || s.store == nil || s.experts == nil || s.chat == nil {
		return store.RepoAnalysisResult{}, fmt.Errorf("repo library service not configured")
	}
	analysis, err := s.store.GetRepoAnalysisResult(ctx, analysisID)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	if analysis.ChatSessionID == nil || strings.TrimSpace(*analysis.ChatSessionID) == "" {
		return store.RepoAnalysisResult{}, fmt.Errorf("%w: analysis is not linked to a chat session", store.ErrValidation)
	}
	source, err := s.store.GetRepoSource(ctx, analysis.RepoSourceID)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	messages, err := s.store.ListChatMessages(ctx, *analysis.ChatSessionID, 1000)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	var latestAssistant *store.ChatMessage
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			latestAssistant = &messages[i]
			break
		}
	}
	if latestAssistant == nil {
		return store.RepoAnalysisResult{}, fmt.Errorf("%w: no assistant message found in linked chat session", store.ErrValidation)
	}
	reportText := sanitizeReportMarkdown(latestAssistant.ContentText)
	if reportText == "" {
		return store.RepoAnalysisResult{}, fmt.Errorf("%w: latest assistant message is empty", store.ErrValidation)
	}

	layout, err := layoutForAnalysisResult(analysis)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	session, err := s.store.GetChatSession(ctx, *analysis.ChatSessionID)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	resolved, err := s.resolveAnalysisTurnRuntime(session, analysis)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	prepared := analysisPrepareResult{
		ResolvedRef:   firstNonEmpty(pointerValue(analysis.ResolvedRef), analysis.RequestedRef),
		CommitSHA:     pointerValue(analysis.CommitSHA),
		CodeIndexPath: filepath.Join(layout.ArtifactsDir, "code_index.json"),
		AnalysisDir:   layout.AnalysisDir,
		SourceDir:     session.WorkspacePath,
		ReportPath:    layout.ReportPath,
	}

	// 1) Validate latest reply directly; 2) Only if invalid, ask AI for a format repair.
	validated, err := s.validateAndFinalizeFormalReport(ctx, source, analysis, prepared, layout, session, resolved, reportCandidate{
		AssistantMessageID: latestAssistant.ID,
		Markdown:           reportText,
	})
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	cardsCount, evidenceCount, refreshSummary, err := s.postProcessAIReport(ctx, source, analysis, layout)
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}

	resultPayload := map[string]any{
		"runtime_kind":                firstNonEmptyStringPtr(analysis.RuntimeKind, "ai_chat"),
		"chat_session_id":             analysis.ChatSessionID,
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
	analysis, err = s.store.FinalizeRepoAnalysisResult(ctx, store.FinalizeRepoAnalysisResultParams{
		AnalysisID: analysis.ID,
		Status:     string(store.RepoAnalysisStatusSucceeded),
		Summary:    &summary,
		ResultJSON: pointer(string(resultJSON)),
		ReportPath: &layout.ReportPath,
	})
	if err != nil {
		return store.RepoAnalysisResult{}, err
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		AnalysisID:             analysis.ID,
		ChatSessionID:          analysis.ChatSessionID,
		ChatUserMessageID:      validated.Candidate.UserMessageID,
		ChatAssistantMessageID: stringPtrIfNotEmpty(validated.Candidate.AssistantMessageID),
		Summary:                &summary,
	})
	return s.store.GetRepoAnalysisResult(ctx, analysis.ID)
}

func layoutForAnalysisResult(analysis store.RepoAnalysisResult) (analysisLayout, error) {
	analysisDir := strings.TrimSpace(analysis.StoragePath)
	if analysisDir == "" {
		return analysisLayout{}, fmt.Errorf("%w: analysis storage_path is empty", store.ErrValidation)
	}
	reportPath := strings.TrimSpace(pointerValue(analysis.ReportPath))
	if reportPath == "" {
		reportPath = filepath.Join(analysisDir, "report.md")
	}
	return analysisLayout{
		AnalysisDir:      analysisDir,
		SourceDir:        "",
		ArtifactsDir:     filepath.Join(analysisDir, "artifacts"),
		DerivedDir:       filepath.Join(analysisDir, "derived"),
		ReportPath:       reportPath,
		ResultPath:       filepath.Join(analysisDir, "derived", "pipeline-result.json"),
		CardsPath:        filepath.Join(analysisDir, "derived", "cards.json"),
		SearchOutputPath: filepath.Join(analysisDir, "derived", "search.json"),
	}, nil
}

func firstNonEmptyStringPtr(value *string, fallback string) string {
	if value != nil && strings.TrimSpace(*value) != "" {
		return strings.TrimSpace(*value)
	}
	return fallback
}

