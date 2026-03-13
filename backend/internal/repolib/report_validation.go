package repolib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vibecraft/backend/internal/expert"
	"vibecraft/backend/internal/logx"
	"vibecraft/backend/internal/store"
)

const maxFormalReportValidationRetries = 5

type reportCandidate struct {
	UserMessageID      *string
	AssistantMessageID string
	Markdown           string
}

type reportValidationResult struct {
	Status               string         `json:"status"`
	Command              string         `json:"command,omitempty"`
	Valid                bool           `json:"valid"`
	Errors               []string       `json:"errors,omitempty"`
	Warnings             []string       `json:"warnings,omitempty"`
	ReportPath           string         `json:"report_path,omitempty"`
	FeatureCountExpected int            `json:"feature_count_expected,omitempty"`
	FeatureCountFound    int            `json:"feature_count_found,omitempty"`
	TableCount           int            `json:"table_count,omitempty"`
	CardCount            int            `json:"card_count,omitempty"`
	EvidenceCount        int            `json:"evidence_count,omitempty"`
	TypeCounts           map[string]int `json:"type_counts,omitempty"`
	Error                *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type validatedReportResult struct {
	Candidate  reportCandidate
	Validation reportValidationResult
	Attempts   int
}

func (s *Service) resolveAnalysisTurnRuntime(session store.ChatSession, analysis store.RepoAnalysisResult) (expert.Resolved, error) {
	toolID := firstNonEmptyTrimmed(pointerValue(session.CLIToolID), pointerValue(analysis.CLIToolID), session.ExpertID)
	modelID := firstNonEmptyTrimmed(pointerValue(session.ModelID), pointerValue(analysis.ModelID))
	return s.experts.ResolveWithOptions(session.ExpertID, "", session.WorkspacePath, expert.ResolveOptions{CLIToolID: toolID, ModelID: modelID})
}

func (s *Service) validateAndFinalizeFormalReport(ctx context.Context, source store.RepoSource, analysis store.RepoAnalysisResult, prepared analysisPrepareResult, layout analysisLayout, session store.ChatSession, resolved expert.Resolved, initial reportCandidate) (validatedReportResult, error) {
	if err := os.MkdirAll(layout.DerivedDir, 0o755); err != nil {
		return validatedReportResult{}, err
	}
	candidate := initial
	maxAttempts := maxFormalReportValidationRetries + 1
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if _, err := s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{
			AnalysisID:             analysis.ID,
			ChatSessionID:          &session.ID,
			ChatUserMessageID:      candidate.UserMessageID,
			ChatAssistantMessageID: stringPtrIfNotEmpty(candidate.AssistantMessageID),
		}); err != nil {
			logx.Warn("repo-library", "report-validation", "更新分析会话绑定失败", "analysis_id", analysis.ID, "attempt", attempt, "err", err)
		}
		reportMarkdown := sanitizeReportMarkdown(candidate.Markdown)
		attemptBase := filepath.Join(layout.DerivedDir, fmt.Sprintf("report.attempt-%d", attempt))
		attemptReportPath := attemptBase + ".md"
		attemptValidationPath := attemptBase + ".validation.json"
		validation, err := s.validateCandidateReport(ctx, source, analysis, layout, reportMarkdown, attemptReportPath, attemptValidationPath)
		if err != nil {
			return validatedReportResult{}, err
		}
		if validation.Valid {
			if err := os.WriteFile(layout.ReportPath, []byte(reportMarkdown+"\n"), 0o644); err != nil {
				return validatedReportResult{}, err
			}
			if err := copyFile(attemptValidationPath, filepath.Join(layout.DerivedDir, "report.validation.json")); err != nil {
				return validatedReportResult{}, err
			}
			return validatedReportResult{
				Candidate:  reportCandidate{UserMessageID: candidate.UserMessageID, AssistantMessageID: candidate.AssistantMessageID, Markdown: reportMarkdown},
				Validation: validation,
				Attempts:   attempt,
			}, nil
		}
		if attempt == maxAttempts {
			invalidPath := filepath.Join(layout.DerivedDir, "report.invalid.md")
			if err := os.WriteFile(invalidPath, []byte(reportMarkdown+"\n"), 0o644); err != nil {
				return validatedReportResult{}, err
			}
			if err := copyFile(attemptValidationPath, filepath.Join(layout.DerivedDir, "report.validation.json")); err != nil {
				return validatedReportResult{}, err
			}
			return validatedReportResult{}, fmt.Errorf("formal report validation failed after %d attempts: %s", attempt, strings.Join(validation.Errors, "; "))
		}
		summary := fmt.Sprintf("正式报告未通过校验，正在请求 AI 修订（%d/%d）…", attempt, maxFormalReportValidationRetries)
		if _, err := s.store.UpdateRepoAnalysisChatBinding(ctx, store.UpdateRepoAnalysisChatBindingParams{AnalysisID: analysis.ID, ChatSessionID: &session.ID, Summary: &summary}); err != nil {
			logx.Warn("repo-library", "report-validation", "更新修订状态摘要失败", "analysis_id", analysis.ID, "attempt", attempt, "err", err)
		}
		logx.Warn("repo-library", "report-validation", "正式报告未通过校验，准备请求 AI 修订", "analysis_id", analysis.ID, "attempt", attempt, "error_count", len(validation.Errors))
		repairPrompt := buildReportRepairTurnPrompt(source, analysis, prepared, validation.Errors, attempt, maxFormalReportValidationRetries)
		turn, err := s.runAutomatedAnalysisTurn(ctx, session, resolved, repairPrompt)
		if err != nil {
			return validatedReportResult{}, err
		}
		candidate = reportCandidate{UserMessageID: &turn.UserMessage.ID, AssistantMessageID: turn.AssistantMessage.ID, Markdown: turn.AssistantMessage.ContentText}
	}
	return validatedReportResult{}, fmt.Errorf("formal report validation loop exited unexpectedly")
}

func (s *Service) validateCandidateReport(ctx context.Context, source store.RepoSource, analysis store.RepoAnalysisResult, layout analysisLayout, reportMarkdown, reportPath, outputPath string) (reportValidationResult, error) {
	if strings.TrimSpace(reportMarkdown) == "" {
		validation := reportValidationResult{
			Status:     "ok",
			Command:    "validate-report",
			Valid:      false,
			Errors:     []string{"最终报告为空，必须返回完整 Markdown 正式报告。"},
			ReportPath: reportPath,
		}
		if err := os.WriteFile(reportPath, []byte("\n"), 0o644); err != nil {
			return reportValidationResult{}, err
		}
		if err := writeValidationJSON(outputPath, validation); err != nil {
			return reportValidationResult{}, err
		}
		return validation, nil
	}
	if err := os.WriteFile(reportPath, []byte(reportMarkdown+"\n"), 0o644); err != nil {
		return reportValidationResult{}, err
	}
	args := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"validate-report",
		"--report-path", reportPath,
		"--repo-url", source.RepoURL,
		"--repo-key", source.RepoKey,
		"--snapshot-id", analysis.ID,
		"--snapshot-dir", layout.AnalysisDir,
		"--run-id", analysis.ID,
		"--output", outputPath,
	}
	for _, feature := range analysis.Features {
		trimmed := strings.TrimSpace(feature)
		if trimmed == "" {
			continue
		}
		args = append(args, "--feature", trimmed)
	}
	if _, err := runBlockingCommand(ctx, s.projectRoot, s.pythonBin, args); err != nil {
		return reportValidationResult{}, err
	}
	payload, err := os.ReadFile(outputPath)
	if err != nil {
		return reportValidationResult{}, err
	}
	var result reportValidationResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return reportValidationResult{}, err
	}
	if result.Status != "ok" {
		if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
			return reportValidationResult{}, fmt.Errorf("%s", result.Error.Message)
		}
		return reportValidationResult{}, fmt.Errorf("validate-report command failed")
	}
	return result, nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func writeValidationJSON(path string, payload reportValidationResult) error {
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(bytes, '\n'), 0o644)
}

func copyFile(src, dst string) error {
	payload, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, payload, 0o644)
}
