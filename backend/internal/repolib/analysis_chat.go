package repolib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/store"
)

func (s *Service) runAIChatAnalysis(ctx context.Context, source store.RepoSource, initial store.RepoAnalysisResult, layout analysisLayout, params CreateAnalysisParams) {
	ctx = firstContext(ctx)
	startedSummary := pointer("正在准备仓库与代码索引…")
	analysis := initial
	analysis, err := s.store.MarkRepoAnalysisStarted(ctx, store.MarkRepoAnalysisStartedParams{AnalysisID: analysis.ID, Summary: startedSummary})
	if err != nil {
		logx.Warn("repo-library", "analysis", "标记 Repo analysis 为 running 失败", "analysis_id", analysis.ID, "err", err)
		return
	}
	s.broadcastAnalysisStatus(source.ID, analysis)
	prepared, err := s.prepareRepositoryForAI(ctx, source, analysis, layout)
	if err != nil {
		s.failAnalysis(ctx, analysis.ID, err)
		return
	}
	analysis, err = s.store.UpdateRepoAnalysisResult(ctx, store.UpdateRepoAnalysisResultParams{
		AnalysisID:          analysis.ID,
		StoragePath:         &layout.AnalysisDir,
		ResolvedRef:         stringPtrIfNotEmpty(prepared.ResolvedRef),
		CommitSHA:           stringPtrIfNotEmpty(prepared.CommitSHA),
		ReportPath:          stringPtrIfNotEmpty(prepared.ReportPath),
		SubagentResultsPath: nil,
	})
	if err != nil {
		s.failAnalysis(ctx, analysis.ID, err)
		return
	}
	session, resolved, err := s.createAnalysisChatSession(ctx, source, prepared, params)
	if err != nil {
		s.failAnalysis(ctx, analysis.ID, err)
		return
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		AnalysisID:    analysis.ID,
		ChatSessionID: &session.ID,
		RuntimeKind:   pointer("ai_chat"),
		CLIToolID:     stringPtrIfNotEmpty(firstNonEmptyTrimmed(resolved.ToolID, params.CLIToolID)),
		ModelID:       stringPtrIfNotEmpty(firstNonEmptyTrimmed(resolved.PrimaryModelID, params.ModelID)),
		Summary:       pointer("仓库准备完成，AI 正在产出最终报告…"),
	})

	finalPrompt := buildFinalReportTurnPrompt(source, analysis, prepared)
	finalTurn, err := s.runAutomatedAnalysisTurn(ctx, session, resolved, finalPrompt)
	if err != nil {
		s.failAnalysis(ctx, analysis.ID, err)
		return
	}
	validated, err := s.validateAndFinalizeFormalReport(ctx, source, analysis, prepared, layout, session, resolved, reportCandidate{
		UserMessageID:      &finalTurn.UserMessage.ID,
		AssistantMessageID: finalTurn.AssistantMessage.ID,
		Markdown:           finalTurn.AssistantMessage.ContentText,
	})
	if err != nil {
		s.failAnalysis(ctx, analysis.ID, err)
		return
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		AnalysisID:             analysis.ID,
		ChatSessionID:          &session.ID,
		ChatUserMessageID:      validated.Candidate.UserMessageID,
		ChatAssistantMessageID: stringPtrIfNotEmpty(validated.Candidate.AssistantMessageID),
		Summary:                pointer("正式报告已通过校验，正在抽取知识卡片并刷新检索索引…"),
	})

	cardsCount, evidenceCount, searchRefresh, err := s.postProcessAIReport(ctx, source, analysis, layout)
	if err != nil {
		s.failAnalysis(ctx, analysis.ID, err)
		return
	}
	resultPayload := map[string]any{
		"runtime_kind":               "ai_chat",
		"chat_session_id":            session.ID,
		"final_user_message_id":      pointerValue(validated.Candidate.UserMessageID),
		"final_assistant_message_id": validated.Candidate.AssistantMessageID,
		"report_path":                layout.ReportPath,
		"report_validation_path":     filepath.Join(layout.DerivedDir, "report.validation.json"),
		"report_attempts":            validated.Attempts,
		"cards_path":                 layout.CardsPath,
		"card_count":                 cardsCount,
		"evidence_count":             evidenceCount,
		"search_refresh":             searchRefresh,
	}
	resultJSON, _ := json.Marshal(resultPayload)
	summary := fmt.Sprintf("AI Chat 分析完成：正式报告已通过校验，抽取 %d 张卡片、%d 条证据。", cardsCount, evidenceCount)
	finalizeParams := store.FinalizeRepoAnalysisResultParams{
		AnalysisID: analysis.ID,
		Status:     string(store.RepoAnalysisStatusSucceeded),
		Summary:    &summary,
		ResultJSON: pointer(string(resultJSON)),
		ReportPath: &layout.ReportPath,
	}
	var finalizeErr error
	for attempt := 0; attempt < 5; attempt++ {
		_, finalizeErr = s.store.FinalizeRepoAnalysisResult(ctx, finalizeParams)
		if finalizeErr == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 300 * time.Millisecond)
	}
	if finalizeErr != nil {
		// Avoid leaving the analysis stuck in `running`.
		s.failAnalysis(ctx, analysis.ID, finalizeErr)
		return
	}
	// best-effort: reload latest record for status/updated_at and broadcast
	if updated, updateErr := s.store.GetRepoAnalysisResult(ctx, analysis.ID); updateErr == nil {
		s.broadcastAnalysisStatus(source.ID, updated)
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		AnalysisID:             analysis.ID,
		ChatSessionID:          &session.ID,
		ChatUserMessageID:      validated.Candidate.UserMessageID,
		ChatAssistantMessageID: stringPtrIfNotEmpty(validated.Candidate.AssistantMessageID),
		Summary:                &summary,
	})
}

func (s *Service) prepareRepositoryForAI(ctx context.Context, source store.RepoSource, analysis store.RepoAnalysisResult, layout analysisLayout) (analysisPrepareResult, error) {
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return analysisPrepareResult{}, err
	}
	args := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"prepare",
		"--repo-url", source.RepoURL,
		"--ref", analysis.RequestedRef,
		"--storage-root", repoLibraryDir,
		"--run-id", analysis.ID,
		"--snapshot-dir", layout.AnalysisDir,
		"--output", layout.ResultPath,
	}
	if _, err := runBlockingCommand(ctx, s.projectRoot, s.pythonBin, args); err != nil {
		return analysisPrepareResult{}, err
	}
	payload, err := os.ReadFile(layout.ResultPath)
	if err != nil {
		return analysisPrepareResult{}, err
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return analysisPrepareResult{}, err
	}
	prepared := analysisPrepareResult{
		ResolvedRef:   extractString(result, "resolved_ref", "snapshot.resolved_ref"),
		CommitSHA:     extractString(result, "commit_sha", "snapshot.commit_sha"),
		CodeIndexPath: extractString(result, "snapshot.code_index_path", "ingest.code_index.output"),
		AnalysisDir:   extractString(result, "snapshot.path", "snapshot_path"),
		SourceDir:     extractString(result, "snapshot.source_dir"),
		ReportPath:    extractString(result, "snapshot.report_path", "report_path"),
	}
	if prepared.AnalysisDir == "" {
		prepared.AnalysisDir = layout.AnalysisDir
	}
	if prepared.SourceDir == "" {
		prepared.SourceDir = layout.SourceDir
	}
	if prepared.ReportPath == "" {
		prepared.ReportPath = layout.ReportPath
	}
	return prepared, nil
}

func (s *Service) createAnalysisChatSession(ctx context.Context, source store.RepoSource, prepared analysisPrepareResult, params CreateAnalysisParams) (store.ChatSession, expert.Resolved, error) {
	toolID := firstNonEmptyTrimmed(params.CLIToolID, "codex")
	resolved, err := s.experts.ResolveWithOptions(toolID, "", prepared.SourceDir, expert.ResolveOptions{CLIToolID: toolID, ModelID: strings.TrimSpace(params.ModelID)})
	if err != nil {
		return store.ChatSession{}, expert.Resolved{}, err
	}
	if resolved.HelperOnly || resolved.Provider == "process" {
		return store.ChatSession{}, expert.Resolved{}, fmt.Errorf("selected CLI tool is not chat-capable")
	}
	title := fmt.Sprintf("Repo Analysis · %s/%s @ %s", source.Owner, source.Repo, firstNonEmpty(prepared.ResolvedRef, source.RepoKey))
	session, err := s.store.CreateChatSession(ctx, store.CreateChatSessionParams{
		Title:         title,
		ExpertID:      firstNonEmptyTrimmed(resolved.ExpertID, toolID),
		Provider:      firstNonEmptyTrimmed(resolved.ProtocolFamily, resolved.Provider),
		Model:         resolved.Model,
		WorkspacePath: prepared.SourceDir,
	})
	if err != nil {
		return store.ChatSession{}, expert.Resolved{}, err
	}
	return session, resolved, nil
}

func (s *Service) runAutomatedAnalysisTurn(ctx context.Context, session store.ChatSession, resolved expert.Resolved, prompt string) (chat.TurnResult, error) {
	return s.chat.RunTurn(ctx, chat.TurnParams{
		Session:     session,
		ExpertID:    firstNonEmptyTrimmed(resolved.ExpertID, session.ExpertID),
		UserInput:   prompt,
		ModelInput:  prompt,
		Attachments: nil,
		Spec:        resolved.Spec,
		Provider:    firstNonEmptyTrimmed(resolved.ProtocolFamily, resolved.Provider),
		Model:       resolved.Model,
	})
}

func (s *Service) postProcessAIReport(ctx context.Context, source store.RepoSource, analysis store.RepoAnalysisResult, layout analysisLayout) (int, int, map[string]any, error) {
	cardsArgs := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"extract-cards",
		"--report-path", layout.ReportPath,
		"--repo-url", source.RepoURL,
		"--repo-key", source.RepoKey,
		"--snapshot-id", analysis.ID,
		"--snapshot-dir", layout.AnalysisDir,
		"--run-id", analysis.ID,
		"--output", layout.CardsPath,
	}
	if _, err := runBlockingCommand(ctx, s.projectRoot, s.pythonBin, cardsArgs); err != nil {
		return 0, 0, nil, err
	}
	cards, err := loadCardsFile(layout.CardsPath)
	if err != nil {
		return 0, 0, nil, err
	}
	if err := s.store.ReplaceRepoKnowledge(ctx, store.ReplaceRepoKnowledgeParams{RepoSourceID: source.ID, AnalysisID: analysis.ID, Cards: cards}); err != nil {
		return 0, 0, nil, err
	}

	// Refresh local search index (go-searchdb). This is best-effort: search index
	// failure should not invalidate a successful report/cards extraction.
	refreshPayload, err := s.refreshSearchIndexForAnalysis(ctx, source, analysis)
	if err != nil {
		logx.Warn("repo-library", "search-index", "刷新 go-searchdb 检索索引失败", "analysis_id", analysis.ID, "err", err)
		refreshPayload = map[string]any{
			"status": "error",
			"engine": "go-searchdb",
			"error":  err.Error(),
		}
	}
	return len(cards), countEvidence(cards), refreshPayload, nil
}

func countEvidence(cards []store.RepoKnowledgeCardInput) int {
	total := 0
	for _, card := range cards {
		total += len(card.Evidence)
	}
	return total
}

func (s *Service) failAnalysis(ctx context.Context, analysisID string, err error) {
	message := err.Error()
	_, finalizeErr := s.store.FinalizeRepoAnalysisResult(ctx, store.FinalizeRepoAnalysisResultParams{AnalysisID: analysisID, Status: string(store.RepoAnalysisStatusFailed), ErrorMessage: &message})
	if finalizeErr != nil {
		logx.Warn("repo-library", "analysis", "标记 Repo analysis 失败时二次失败", "analysis_id", analysisID, "err", finalizeErr)
	}
	if updated, updateErr := s.store.GetRepoAnalysisResult(ctx, analysisID); updateErr == nil {
		s.broadcastAnalysisStatus(updated.RepoSourceID, updated)
	}
}

func mustRepoLibraryDir() string {
	dir, _ := paths.RepoLibraryDir()
	return dir
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
