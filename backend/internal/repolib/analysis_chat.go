package repolib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/store"
)

func (s *Service) runAIChatAnalysis(ctx context.Context, source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, layout pipelineLayout, params CreateAnalysisParams) {
	ctx = firstContext(ctx)
	startedSummary := pointer("正在准备仓库快照与代码索引…")
	run, err := s.store.MarkRepoAnalysisRunStarted(ctx, store.MarkRepoAnalysisRunStartedParams{RunID: run.ID, Summary: startedSummary})
	if err != nil {
		logx.Warn("repo-library", "analysis", "标记 Repo analysis run 为 running 失败", "run_id", run.ID, "err", err)
		return
	}
	prepared, err := s.prepareRepositoryForAI(ctx, source, snapshot, run, layout)
	if err != nil {
		s.failRun(ctx, run.ID, err)
		return
	}
	snapshot, err = s.store.UpdateRepoSnapshot(ctx, store.UpdateRepoSnapshotParams{
		SnapshotID:          snapshot.ID,
		StoragePath:         &layout.SnapshotDir,
		ResolvedRef:         stringPtrIfNotEmpty(prepared.ResolvedRef),
		CommitSHA:           stringPtrIfNotEmpty(prepared.CommitSHA),
		ReportPath:          stringPtrIfNotEmpty(prepared.ReportPath),
		SubagentResultsPath: nil,
	})
	if err != nil {
		s.failRun(ctx, run.ID, err)
		return
	}
	session, resolved, err := s.createAnalysisChatSession(ctx, source, prepared, params)
	if err != nil {
		s.failRun(ctx, run.ID, err)
		return
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		RunID:         run.ID,
		ChatSessionID: &session.ID,
		RuntimeKind:   pointer("ai_chat"),
		CLIToolID:     stringPtrIfNotEmpty(firstNonEmptyTrimmed(resolved.ToolID, params.CLIToolID)),
		ModelID:       stringPtrIfNotEmpty(firstNonEmptyTrimmed(resolved.PrimaryModelID, params.ModelID)),
		Summary:       pointer("仓库准备完成，AI 正在产出最终报告…"),
	})

	finalPrompt := buildFinalReportTurnPrompt(source, snapshot, run, prepared)
	finalTurn, err := s.runAutomatedAnalysisTurn(ctx, session, resolved, finalPrompt)
	if err != nil {
		s.failRun(ctx, run.ID, err)
		return
	}
	reportMarkdown := sanitizeReportMarkdown(finalTurn.AssistantMessage.ContentText)
	if strings.TrimSpace(reportMarkdown) == "" {
		s.failRun(ctx, run.ID, fmt.Errorf("final analysis report is empty"))
		return
	}
	if err := os.WriteFile(layout.ReportPath, []byte(reportMarkdown+"\n"), 0o644); err != nil {
		s.failRun(ctx, run.ID, err)
		return
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		RunID:                 run.ID,
		ChatSessionID:         &session.ID,
		ChatUserMessageID:     &finalTurn.UserMessage.ID,
		ChatAssistantMessageID: &finalTurn.AssistantMessage.ID,
		Summary:               pointer("最终报告已生成，正在抽取知识卡片并刷新检索索引…"),
	})

	cardsCount, evidenceCount, searchRefresh, err := s.postProcessAIReport(ctx, source, snapshot, run, layout)
	if err != nil {
		s.failRun(ctx, run.ID, err)
		return
	}
	resultPayload := map[string]any{
		"runtime_kind": "ai_chat",
		"chat_session_id": session.ID,
		"final_user_message_id": finalTurn.UserMessage.ID,
		"final_assistant_message_id": finalTurn.AssistantMessage.ID,
		"report_path": layout.ReportPath,
		"cards_path": layout.CardsPath,
		"card_count": cardsCount,
		"evidence_count": evidenceCount,
		"search_refresh": searchRefresh,
	}
	resultJSON, _ := json.Marshal(resultPayload)
	summary := fmt.Sprintf("AI Chat 分析完成：已生成报告，抽取 %d 张卡片、%d 条证据。", cardsCount, evidenceCount)
	_, err = s.store.FinalizeRepoAnalysisRun(ctx, store.FinalizeRepoAnalysisRunParams{
		RunID:      run.ID,
		Status:     string(store.RepoAnalysisStatusSucceeded),
		Summary:    &summary,
		ResultJSON: pointer(string(resultJSON)),
		ReportPath: &layout.ReportPath,
	})
	if err != nil {
		logx.Warn("repo-library", "analysis", "收敛 AI Chat Repo analysis 失败", "run_id", run.ID, "err", err)
		return
	}
	_, _ = s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
		RunID:                 run.ID,
		ChatSessionID:         &session.ID,
		ChatUserMessageID:     &finalTurn.UserMessage.ID,
		ChatAssistantMessageID: &finalTurn.AssistantMessage.ID,
		Summary:               &summary,
	})
}

func (s *Service) prepareRepositoryForAI(ctx context.Context, source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, layout pipelineLayout) (analysisPrepareResult, error) {
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return analysisPrepareResult{}, err
	}
	args := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"prepare",
		"--repo-url", source.RepoURL,
		"--ref", snapshot.RequestedRef,
		"--storage-root", repoLibraryDir,
		"--run-id", run.ID,
		"--snapshot-dir", layout.SnapshotDir,
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
		SnapshotDir:   extractString(result, "snapshot.path", "snapshot_path"),
		SourceDir:     extractString(result, "snapshot.source_dir"),
		ReportPath:    extractString(result, "snapshot.report_path", "report_path"),
	}
	if prepared.SnapshotDir == "" {
		prepared.SnapshotDir = layout.SnapshotDir
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

func (s *Service) postProcessAIReport(ctx context.Context, source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, layout pipelineLayout) (int, int, map[string]any, error) {
	cardsArgs := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"extract-cards",
		"--report-path", layout.ReportPath,
		"--repo-url", source.RepoURL,
		"--repo-key", source.RepoKey,
		"--snapshot-id", snapshot.ID,
		"--snapshot-dir", layout.SnapshotDir,
		"--run-id", run.ID,
		"--output", layout.CardsPath,
	}
	if _, err := runBlockingCommand(ctx, s.projectRoot, s.pythonBin, cardsArgs); err != nil {
		return 0, 0, nil, err
	}
	cards, err := loadCardsFile(layout.CardsPath)
	if err != nil {
		return 0, 0, nil, err
	}
	if err := s.store.ReplaceRepoKnowledge(ctx, store.ReplaceRepoKnowledgeParams{RepoSourceID: source.ID, RepoSnapshotID: snapshot.ID, AnalysisRunID: run.ID, Cards: cards}); err != nil {
		return 0, 0, nil, err
	}
	searchArgs := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"search",
		"--storage-root", mustRepoLibraryDir(),
		"--refresh", "auto",
		"--repo-url", source.RepoURL,
		"--repo-key", source.RepoKey,
		"--snapshot-id", snapshot.ID,
		"--snapshot-dir", layout.SnapshotDir,
		"--run-id", run.ID,
		"--report-path", layout.ReportPath,
		"--cards-path", layout.CardsPath,
		"--output", layout.SearchOutputPath,
	}
	if _, err := runBlockingCommand(ctx, s.projectRoot, s.pythonBin, searchArgs); err != nil {
		return len(cards), countEvidence(cards), nil, err
	}
	refreshPayload := map[string]any{}
	if payload, err := os.ReadFile(layout.SearchOutputPath); err == nil {
		_ = json.Unmarshal(payload, &refreshPayload)
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

func (s *Service) failRun(ctx context.Context, runID string, err error) {
	message := err.Error()
	_, finalizeErr := s.store.FinalizeRepoAnalysisRun(ctx, store.FinalizeRepoAnalysisRunParams{RunID: runID, Status: string(store.RepoAnalysisStatusFailed), ErrorMessage: &message})
	if finalizeErr != nil {
		logx.Warn("repo-library", "analysis", "标记 Repo analysis run 失败时二次失败", "run_id", runID, "err", finalizeErr)
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
