package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"vibecraft/backend/internal/id"
)

type RepoAnalysisStatus string

const (
	RepoAnalysisStatusQueued    RepoAnalysisStatus = "queued"
	RepoAnalysisStatusRunning   RepoAnalysisStatus = "running"
	RepoAnalysisStatusSucceeded RepoAnalysisStatus = "succeeded"
	RepoAnalysisStatusFailed    RepoAnalysisStatus = "failed"
)

type RepoSource struct {
	ID            string  `json:"repository_id"`
	RepoURL       string  `json:"repo_url"`
	Owner         string  `json:"owner"`
	Repo          string  `json:"repo"`
	RepoKey       string  `json:"repo_key"`
	DefaultBranch *string `json:"default_branch,omitempty"`
	Visibility    *string `json:"visibility,omitempty"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
}

type RepoReportContextSummary struct {
	GeneratedAt         *string `json:"generated_at,omitempty"`
	StackOverview       *string `json:"stack_overview,omitempty"`
	BackendSummary      *string `json:"backend_summary,omitempty"`
	FrontendSummary     *string `json:"frontend_summary,omitempty"`
	OtherModulesSummary *string `json:"other_modules_summary,omitempty"`
}

type RepoAnalysisResult struct {
	ID                     string                    `json:"analysis_id"`
	RepoSourceID           string                    `json:"repository_id"`
	RequestedRef           string                    `json:"requested_ref"`
	ResolvedRef            *string                   `json:"resolved_ref,omitempty"`
	CommitSHA              *string                   `json:"commit_sha,omitempty"`
	StoragePath            string                    `json:"storage_path"`
	ReportPath             *string                   `json:"report_path,omitempty"`
	SubagentResultsPath    *string                   `json:"subagent_results_path,omitempty"`
	ExecutionID            *string                   `json:"execution_id,omitempty"`
	ChatSessionID          *string                   `json:"chat_session_id,omitempty"`
	ChatUserMessageID      *string                   `json:"chat_user_message_id,omitempty"`
	ChatAssistantMessageID *string                   `json:"chat_assistant_message_id,omitempty"`
	RuntimeKind            *string                   `json:"runtime_kind,omitempty"`
	CLIToolID              *string                   `json:"cli_tool_id,omitempty"`
	ModelID                *string                   `json:"model_id,omitempty"`
	Status                 string                    `json:"status"`
	Language               string                    `json:"language"`
	Depth                  string                    `json:"depth"`
	AgentMode              string                    `json:"agent_mode"`
	Features               []string                  `json:"features"`
	Summary                *string                   `json:"summary,omitempty"`
	ErrorMessage           *string                   `json:"error_message,omitempty"`
	ResultJSON             *string                   `json:"result_json,omitempty"`
	StartedAt              *int64                    `json:"started_at,omitempty"`
	EndedAt                *int64                    `json:"ended_at,omitempty"`
	ReportContext          *RepoReportContextSummary `json:"report_context_summary,omitempty"`
	CreatedAt              int64                     `json:"created_at"`
	UpdatedAt              int64                     `json:"updated_at"`
}

type RepoKnowledgeCard struct {
	ID           string   `json:"card_id"`
	RepoSourceID string   `json:"repository_id"`
	AnalysisID   string   `json:"analysis_id"`
	Title        string   `json:"title"`
	CardType     string   `json:"card_type"`
	Conclusion   *string  `json:"conclusion,omitempty"`
	Summary      string   `json:"summary"`
	Mechanism    *string  `json:"mechanism,omitempty"`
	Confidence   *string  `json:"confidence,omitempty"`
	Tags         []string `json:"tags"`
	SectionTitle *string  `json:"section_title,omitempty"`
	SortIndex    int      `json:"sort_index"`
	CreatedAt    int64    `json:"created_at"`
}

type RepoKnowledgeEvidence struct {
	ID        string  `json:"evidence_id"`
	CardID    string  `json:"card_id"`
	Path      string  `json:"path"`
	Line      *int64  `json:"line,omitempty"`
	Snippet   *string `json:"snippet,omitempty"`
	Dimension *string `json:"dimension,omitempty"`
	SortIndex int     `json:"sort_index"`
	CreatedAt int64   `json:"created_at"`
}

// ListRepoEvidenceByAnalysis 功能：返回某个分析结果下所有卡片的 evidence（用于批量 hydration）。
// 参数/返回：analysisID 为目标分析 id；返回按 card_id + sort_index 排序的 evidence 列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoEvidenceByAnalysis(ctx context.Context, analysisID string, limit int) ([]RepoKnowledgeEvidence, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	analysisID = strings.TrimSpace(analysisID)
	if analysisID == "" {
		return nil, fmt.Errorf("%w: analysis_id is required", ErrValidation)
	}
	if limit <= 0 || limit > 20000 {
		limit = 20000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
  e.id,
  e.card_id,
  e.path,
  e.line,
  e.snippet,
  e.dimension,
  e.sort_index,
  e.created_at
FROM repo_knowledge_evidence e
JOIN repo_knowledge_cards c ON c.id = e.card_id
WHERE c.analysis_result_id = ?
ORDER BY e.card_id ASC, e.sort_index ASC, e.created_at ASC
LIMIT ?;`, analysisID, limit)
	if err != nil {
		return nil, fmt.Errorf("query repo evidence by analysis: %w", err)
	}
	defer rows.Close()
	out := make([]RepoKnowledgeEvidence, 0)
	for rows.Next() {
		item, err := scanRepoKnowledgeEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo evidence by analysis: %w", err)
	}
	return out, nil
}

type RepoSourceSummary struct {
	RepoSource
	LatestAnalysisID      *string `json:"latest_analysis_id,omitempty"`
	LatestCommitSHA       *string `json:"latest_commit_sha,omitempty"`
	LatestResolvedRef     *string `json:"latest_resolved_ref,omitempty"`
	LatestAnalysisStatus  *string `json:"latest_analysis_status,omitempty"`
	LatestAnalysisUpdated *int64  `json:"latest_analysis_updated_at,omitempty"`
	CardsCount            int64   `json:"cards_count"`
}

type RepoLibraryDetail struct {
	Repository RepoSource           `json:"repository"`
	Analyses   []RepoAnalysisResult `json:"analyses"`
	Cards      []RepoKnowledgeCard  `json:"cards"`
}

type RepoSimilarityQuery struct {
	ID          string   `json:"query_id"`
	QueryText   string   `json:"query_text"`
	RepoFilters []string `json:"repo_filters"`
	Mode        string   `json:"mode"`
	TopK        int      `json:"top_k"`
	ResultJSON  *string  `json:"result_json,omitempty"`
	CreatedAt   int64    `json:"created_at"`
}

type UpsertRepoSourceParams struct {
	RepoURL       string
	Owner         string
	Repo          string
	RepoKey       string
	DefaultBranch *string
	Visibility    *string
}

type CreateRepoAnalysisResultParams struct {
	AnalysisID   string
	RepoSourceID string
	RequestedRef string
	StoragePath  string
	ReportPath   *string
	Language     string
	Depth        string
	AgentMode    string
	Features     []string
	RuntimeKind  *string
	CLIToolID    *string
	ModelID      *string
}

type UpdateRepoAnalysisResultParams struct {
	AnalysisID          string
	StoragePath         *string
	ResolvedRef         *string
	CommitSHA           *string
	ReportPath          *string
	SubagentResultsPath *string
}

type StartRepoAnalysisResultParams struct {
	AnalysisID  string
	ExecutionID string
}

type FinalizeRepoAnalysisResultParams struct {
	AnalysisID   string
	Status       string
	Summary      *string
	ErrorMessage *string
	ResultJSON   *string
	ReportPath   *string
}

type ReplaceRepoKnowledgeParams struct {
	RepoSourceID string
	AnalysisID   string
	Cards        []RepoKnowledgeCardInput
}

type DeleteRepoAnalysisResultParams struct {
	AnalysisID             string
	DeleteRepositoryIfLast bool
}

type DeleteRepoAnalysisResultResult struct {
	RepositoryID      string
	DeletedRepository bool
	DeletedAnalysis   RepoAnalysisResult
}

type RepoKnowledgeCardInput struct {
	Title        string
	CardType     string
	Conclusion   *string
	Summary      string
	Mechanism    *string
	Confidence   *string
	Tags         []string
	SectionTitle *string
	SortIndex    int
	Evidence     []RepoKnowledgeEvidenceInput
}

type RepoKnowledgeEvidenceInput struct {
	Path      string
	Line      *int64
	Snippet   *string
	Dimension *string
	SortIndex int
}

type ListRepoCardsParams struct {
	RepoSourceID string
	AnalysisID   string
	Limit        int
}

type RecordRepoSimilarityQueryParams struct {
	QueryText   string
	RepoFilters []string
	Mode        string
	TopK        int
	ResultJSON  *string
}

type repoLibraryScanner interface {
	Scan(dest ...any) error
}

// UpsertRepoSource 功能：创建或更新 Repo Library 仓库来源记录。
// 参数/返回：params 提供规范化后的 repo 元数据；返回最新 RepoSource。
// 失败场景：关键字段缺失或写库失败时返回 error。
// 副作用：写入 SQLite `repo_sources`。
func (s *Store) UpsertRepoSource(ctx context.Context, params UpsertRepoSourceParams) (RepoSource, error) {
	if s == nil || s.db == nil {
		return RepoSource{}, fmt.Errorf("store not initialized")
	}
	repoURL := strings.TrimSpace(params.RepoURL)
	owner := strings.TrimSpace(params.Owner)
	repo := strings.TrimSpace(params.Repo)
	repoKey := strings.TrimSpace(params.RepoKey)
	if repoURL == "" || owner == "" || repo == "" || repoKey == "" {
		return RepoSource{}, fmt.Errorf("%w: repo_url/owner/repo/repo_key are required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_url, owner, repo, repo_key, default_branch, visibility, created_at, updated_at FROM repo_sources WHERE repo_url = ? LIMIT 1;`, repoURL)
	existing, err := scanRepoSource(row)
	if err == nil {
		now := time.Now().UnixMilli()
		existing.Owner = owner
		existing.Repo = repo
		existing.RepoKey = repoKey
		existing.DefaultBranch = trimOrNil(params.DefaultBranch)
		existing.Visibility = trimOrNil(params.Visibility)
		existing.UpdatedAt = now
		if _, err := s.db.ExecContext(ctx, `UPDATE repo_sources SET owner = ?, repo = ?, repo_key = ?, default_branch = ?, visibility = ?, updated_at = ? WHERE id = ?;`, existing.Owner, existing.Repo, existing.RepoKey, existing.DefaultBranch, existing.Visibility, existing.UpdatedAt, existing.ID); err != nil {
			return RepoSource{}, fmt.Errorf("update repo source: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RepoSource{}, fmt.Errorf("query repo source: %w", err)
	}
	now := time.Now().UnixMilli()
	source := RepoSource{
		ID:            id.New("rs_"),
		RepoURL:       repoURL,
		Owner:         owner,
		Repo:          repo,
		RepoKey:       repoKey,
		DefaultBranch: trimOrNil(params.DefaultBranch),
		Visibility:    trimOrNil(params.Visibility),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repo_sources (id, repo_url, owner, repo, repo_key, default_branch, visibility, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`, source.ID, source.RepoURL, source.Owner, source.Repo, source.RepoKey, source.DefaultBranch, source.Visibility, source.CreatedAt, source.UpdatedAt); err != nil {
		return RepoSource{}, fmt.Errorf("insert repo source: %w", err)
	}
	return source, nil
}

// CreateRepoAnalysisResult 功能：创建一条 Repo Library 分析结果记录（queued）。
// 参数/返回：params 提供仓库、ref、features 与存储路径；返回 RepoAnalysisResult。
// 失败场景：关键字段缺失、features 为空或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) CreateRepoAnalysisResult(ctx context.Context, params CreateRepoAnalysisResultParams) (RepoAnalysisResult, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisResult{}, fmt.Errorf("store not initialized")
	}
	repoSourceID := strings.TrimSpace(params.RepoSourceID)
	requestedRef := strings.TrimSpace(params.RequestedRef)
	storagePath := strings.TrimSpace(params.StoragePath)
	if repoSourceID == "" || requestedRef == "" || storagePath == "" {
		return RepoAnalysisResult{}, fmt.Errorf("%w: repo_source_id/requested_ref/storage_path are required", ErrValidation)
	}
	features := uniqueTrimmedStrings(params.Features)
	if len(features) == 0 {
		return RepoAnalysisResult{}, fmt.Errorf("%w: at least one feature is required", ErrValidation)
	}
	featuresJSON, err := json.Marshal(features)
	if err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("marshal features: %w", err)
	}
	now := time.Now().UnixMilli()
	analysisID := strings.TrimSpace(params.AnalysisID)
	if analysisID == "" {
		analysisID = id.New("ra_")
	}
	result := RepoAnalysisResult{
		ID:           analysisID,
		RepoSourceID: repoSourceID,
		RequestedRef: requestedRef,
		StoragePath:  storagePath,
		ReportPath:   trimOrNil(params.ReportPath),
		RuntimeKind:  trimOrNil(params.RuntimeKind),
		CLIToolID:    trimOrNil(params.CLIToolID),
		ModelID:      trimOrNil(params.ModelID),
		Status:       string(RepoAnalysisStatusQueued),
		Language:     firstNonEmptyString(params.Language, "zh"),
		Depth:        firstNonEmptyString(params.Depth, "standard"),
		AgentMode:    firstNonEmptyString(params.AgentMode, "single"),
		Features:     features,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO repo_analysis_results (
  id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path,
  execution_id, chat_session_id, chat_user_message_id, chat_assistant_message_id,
  runtime_kind, cli_tool_id, model_id,
  status, language, depth, agent_mode, features_json,
  summary, error_message, result_json, started_at, ended_at,
  created_at, updated_at
) VALUES (
  ?, ?, ?, NULL, NULL, ?, ?, NULL,
  NULL, NULL, NULL, NULL,
  ?, ?, ?,
  ?, ?, ?, ?, ?,
  NULL, NULL, NULL, NULL, NULL,
  ?, ?
);`, result.ID, result.RepoSourceID, result.RequestedRef, result.StoragePath, result.ReportPath, result.RuntimeKind, result.CLIToolID, result.ModelID, result.Status, result.Language, result.Depth, result.AgentMode, string(featuresJSON), result.CreatedAt, result.UpdatedAt); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("insert repo analysis result: %w", err)
	}
	return result, nil
}

// UpdateRepoAnalysisResult 功能：更新分析结果的 repo 解析信息与文件路径。
// 参数/返回：params 指定 analysis id 与解析字段；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) UpdateRepoAnalysisResult(ctx context.Context, params UpdateRepoAnalysisResultParams) (RepoAnalysisResult, error) {
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
	if storagePath := trimOrNil(params.StoragePath); storagePath != nil {
		result.StoragePath = *storagePath
	}
	result.ResolvedRef = trimOrNil(params.ResolvedRef)
	result.CommitSHA = trimOrNil(params.CommitSHA)
	result.ReportPath = trimOrNil(params.ReportPath)
	result.SubagentResultsPath = trimOrNil(params.SubagentResultsPath)
	result.UpdatedAt = time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_results SET storage_path = ?, resolved_ref = ?, commit_sha = ?, report_path = ?, subagent_results_path = ?, updated_at = ? WHERE id = ?;`, result.StoragePath, result.ResolvedRef, result.CommitSHA, result.ReportPath, result.SubagentResultsPath, result.UpdatedAt, result.ID); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("update repo analysis result: %w", err)
	}
	return result, nil
}

type UpdateRepoAnalysisResultChatLinkParams struct {
	AnalysisID             string
	ChatSessionID          *string
	ChatUserMessageID      *string
	ChatAssistantMessageID *string
}

// UpdateRepoAnalysisResultChatLink 功能：更新分析结果关联的 chat linkage 字段。
// 参数/返回：params 指定 analysis id 与 chat message ids；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) UpdateRepoAnalysisResultChatLink(ctx context.Context, params UpdateRepoAnalysisResultChatLinkParams) (RepoAnalysisResult, error) {
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
	result.ChatSessionID = trimOrNil(params.ChatSessionID)
	result.ChatUserMessageID = trimOrNil(params.ChatUserMessageID)
	result.ChatAssistantMessageID = trimOrNil(params.ChatAssistantMessageID)
	result.UpdatedAt = time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_results SET chat_session_id = ?, chat_user_message_id = ?, chat_assistant_message_id = ?, updated_at = ? WHERE id = ?;`, result.ChatSessionID, result.ChatUserMessageID, result.ChatAssistantMessageID, result.UpdatedAt, result.ID); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("update repo analysis result chat link: %w", err)
	}
	return result, nil
}

// StartRepoAnalysisResult 功能：将分析结果标记为 running，并记录 execution_id。
// 参数/返回：params 指定 analysis id 与 execution id；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) StartRepoAnalysisResult(ctx context.Context, params StartRepoAnalysisResultParams) (RepoAnalysisResult, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisResult{}, fmt.Errorf("store not initialized")
	}
	analysisID := strings.TrimSpace(params.AnalysisID)
	executionID := strings.TrimSpace(params.ExecutionID)
	if analysisID == "" || executionID == "" {
		return RepoAnalysisResult{}, fmt.Errorf("%w: analysis_id/execution_id are required", ErrValidation)
	}
	result, err := s.GetRepoAnalysisResult(ctx, analysisID)
	if err != nil {
		return RepoAnalysisResult{}, err
	}
	now := time.Now().UnixMilli()
	result.ExecutionID = &executionID
	result.Status = string(RepoAnalysisStatusRunning)
	result.StartedAt = &now
	result.UpdatedAt = now
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_results SET execution_id = ?, status = ?, started_at = ?, updated_at = ? WHERE id = ?;`, result.ExecutionID, result.Status, result.StartedAt, result.UpdatedAt, result.ID); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("start repo analysis result: %w", err)
	}
	return result, nil
}

// FinalizeRepoAnalysisResult 功能：收敛分析结果终态并写入结果路径摘要。
// 参数/返回：params 指定终态、摘要与错误；返回更新后的 RepoAnalysisResult。
// 失败场景：analysis 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_results`。
func (s *Store) FinalizeRepoAnalysisResult(ctx context.Context, params FinalizeRepoAnalysisResultParams) (RepoAnalysisResult, error) {
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
	now := time.Now().UnixMilli()
	result.Status = strings.TrimSpace(params.Status)
	if result.Status == "" {
		result.Status = string(RepoAnalysisStatusFailed)
	}
	result.Summary = trimOrNil(params.Summary)
	result.ErrorMessage = trimOrNil(params.ErrorMessage)
	result.ResultJSON = trimOrNil(params.ResultJSON)
	result.ReportPath = trimOrNil(params.ReportPath)
	result.EndedAt = &now
	result.UpdatedAt = now
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_results SET status = ?, summary = ?, error_message = ?, result_json = ?, report_path = ?, ended_at = ?, updated_at = ? WHERE id = ?;`, result.Status, result.Summary, result.ErrorMessage, result.ResultJSON, result.ReportPath, result.EndedAt, result.UpdatedAt, result.ID); err != nil {
		return RepoAnalysisResult{}, fmt.Errorf("finalize repo analysis result: %w", err)
	}
	return result, nil
}

// ReplaceRepoKnowledge 功能：用最新抽取结果替换某次分析结果下的知识卡片与证据。
// 参数/返回：params 提供 repository_id/analysis_id 与卡片输入；无返回值。
// 失败场景：写库失败时返回 error。
// 副作用：删除旧卡片与 evidence，并插入新结果。
func (s *Store) ReplaceRepoKnowledge(ctx context.Context, params ReplaceRepoKnowledgeParams) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if strings.TrimSpace(params.RepoSourceID) == "" || strings.TrimSpace(params.AnalysisID) == "" {
		return fmt.Errorf("%w: repo_source_id/analysis_id are required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_knowledge_evidence WHERE card_id IN (SELECT id FROM repo_knowledge_cards WHERE analysis_result_id = ?);`, strings.TrimSpace(params.AnalysisID)); err != nil {
		return fmt.Errorf("delete repo evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_knowledge_cards WHERE analysis_result_id = ?;`, strings.TrimSpace(params.AnalysisID)); err != nil {
		return fmt.Errorf("delete repo cards: %w", err)
	}
	now := time.Now().UnixMilli()
	for idx, input := range params.Cards {
		card := RepoKnowledgeCard{
			ID:           id.New("rc_"),
			RepoSourceID: params.RepoSourceID,
			AnalysisID:   strings.TrimSpace(params.AnalysisID),
			Title:        strings.TrimSpace(input.Title),
			CardType:     strings.TrimSpace(input.CardType),
			Summary:      strings.TrimSpace(input.Summary),
			Mechanism:    trimOrNil(input.Mechanism),
			Confidence:   trimOrNil(input.Confidence),
			Tags:         uniqueTrimmedStrings(input.Tags),
			SectionTitle: trimOrNil(input.SectionTitle),
			SortIndex:    input.SortIndex,
			CreatedAt:    now,
		}
		if card.Title == "" || card.CardType == "" || card.Summary == "" {
			continue
		}
		if card.SortIndex == 0 {
			card.SortIndex = idx + 1
		}
		tagsJSON, err := json.Marshal(card.Tags)
		if err != nil {
			return fmt.Errorf("marshal repo card tags: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO repo_knowledge_cards (id, repo_source_id, analysis_result_id, title, card_type, summary, mechanism, confidence, tags_json, section_title, sort_index, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, card.ID, card.RepoSourceID, card.AnalysisID, card.Title, card.CardType, card.Summary, card.Mechanism, card.Confidence, string(tagsJSON), card.SectionTitle, card.SortIndex, card.CreatedAt); err != nil {
			return fmt.Errorf("insert repo card: %w", err)
		}
		for evidenceIdx, evidence := range input.Evidence {
			path := strings.TrimSpace(evidence.Path)
			if path == "" {
				continue
			}
			sortIndex := evidence.SortIndex
			if sortIndex == 0 {
				sortIndex = evidenceIdx + 1
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO repo_knowledge_evidence (id, card_id, path, line, snippet, dimension, sort_index, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, id.New("re_"), card.ID, path, evidence.Line, trimOrNil(evidence.Snippet), trimOrNil(evidence.Dimension), sortIndex, now); err != nil {
				return fmt.Errorf("insert repo evidence: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repo knowledge replace: %w", err)
	}
	return nil
}

// DeleteRepoAnalysisResult 功能：删除指定 analysis result，并在其为最后一个 analysis 时按需级联删除仓库。
// 参数/返回：params 指定 analysis_id 与是否允许删除最后一份 analysis 后连带删除仓库；返回删除结果信息。
// 失败场景：analysis 不存在、analysis 正在 queued/running、或删除最后一份 analysis 但未显式允许级联时返回 error。
// 副作用：删除 repo_analysis_results 及其下的知识卡片/证据；可能删除 repo_sources。
func (s *Store) DeleteRepoAnalysisResult(ctx context.Context, params DeleteRepoAnalysisResultParams) (DeleteRepoAnalysisResultResult, error) {
	if s == nil || s.db == nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("store not initialized")
	}
	analysisID := strings.TrimSpace(params.AnalysisID)
	if analysisID == "" {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("%w: analysis_id is required", ErrValidation)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `SELECT id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path, execution_id, chat_session_id, chat_user_message_id, chat_assistant_message_id, runtime_kind, cli_tool_id, model_id, status, language, depth, agent_mode, features_json, summary, error_message, result_json, started_at, ended_at, created_at, updated_at FROM repo_analysis_results WHERE id = ? LIMIT 1;`, analysisID)
	analysis, err := scanRepoAnalysisResult(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeleteRepoAnalysisResultResult{}, os.ErrNotExist
		}
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("query repo analysis result: %w", err)
	}
	if analysis.Status == string(RepoAnalysisStatusQueued) || analysis.Status == string(RepoAnalysisStatusRunning) {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("%w: cannot delete a queued/running analysis", ErrValidation)
	}

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM repo_analysis_results WHERE repo_source_id = ?;`, analysis.RepoSourceID).Scan(&total); err != nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("count repo analyses: %w", err)
	}
	if total == 1 && !params.DeleteRepositoryIfLast {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("%w: deleting the last analysis requires delete_repository_if_last=true", ErrValidation)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_knowledge_evidence WHERE card_id IN (SELECT id FROM repo_knowledge_cards WHERE analysis_result_id = ?);`, analysis.ID); err != nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("delete repo evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_knowledge_cards WHERE analysis_result_id = ?;`, analysis.ID); err != nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("delete repo cards: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_analysis_results WHERE id = ?;`, analysis.ID); err != nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("delete repo analysis result: %w", err)
	}

	deletedRepository := false
	if total == 1 && params.DeleteRepositoryIfLast {
		if _, err := tx.ExecContext(ctx, `DELETE FROM repo_sources WHERE id = ?;`, analysis.RepoSourceID); err != nil {
			return DeleteRepoAnalysisResultResult{}, fmt.Errorf("delete repo source: %w", err)
		}
		deletedRepository = true
	}

	if err := tx.Commit(); err != nil {
		return DeleteRepoAnalysisResultResult{}, fmt.Errorf("commit delete repo analysis: %w", err)
	}

	return DeleteRepoAnalysisResultResult{
		RepositoryID:      analysis.RepoSourceID,
		DeletedRepository: deletedRepository,
		DeletedAnalysis:   analysis,
	}, nil
}

// RecordRepoSimilarityQuery 功能：记录一次 Repo Library 搜索请求与结果摘要。
// 参数/返回：params 指定 query、过滤器、模式与结果；返回 RepoSimilarityQuery。
// 失败场景：query 为空或写库失败时返回 error。
// 副作用：写入 SQLite `repo_similarity_queries`。
func (s *Store) RecordRepoSimilarityQuery(ctx context.Context, params RecordRepoSimilarityQueryParams) (RepoSimilarityQuery, error) {
	if s == nil || s.db == nil {
		return RepoSimilarityQuery{}, fmt.Errorf("store not initialized")
	}
	queryText := strings.TrimSpace(params.QueryText)
	if queryText == "" {
		return RepoSimilarityQuery{}, fmt.Errorf("%w: query_text is required", ErrValidation)
	}
	filters := uniqueTrimmedStrings(params.RepoFilters)
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return RepoSimilarityQuery{}, fmt.Errorf("marshal repo filters: %w", err)
	}
	now := time.Now().UnixMilli()
	query := RepoSimilarityQuery{ID: id.New("rq_"), QueryText: queryText, RepoFilters: filters, Mode: firstNonEmptyString(params.Mode, "semi"), TopK: maxInt(params.TopK, 10), ResultJSON: trimOrNil(params.ResultJSON), CreatedAt: now}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repo_similarity_queries (id, query_text, repo_filters_json, mode, top_k, result_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?);`, query.ID, query.QueryText, string(filtersJSON), query.Mode, query.TopK, query.ResultJSON, query.CreatedAt); err != nil {
		return RepoSimilarityQuery{}, fmt.Errorf("insert repo similarity query: %w", err)
	}
	return query, nil
}

// ListRepoSources 功能：返回 Repo Library 仓库列表摘要。
// 参数/返回：limit 控制返回数量；返回 RepoSourceSummary 列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoSources(ctx context.Context, limit int) ([]RepoSourceSummary, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
  rs.id,
  rs.repo_url,
  rs.owner,
  rs.repo,
  rs.repo_key,
  rs.default_branch,
  rs.visibility,
  rs.created_at,
  rs.updated_at,
  (
    SELECT id FROM repo_analysis_results a
    WHERE a.repo_source_id = rs.id
    ORDER BY a.updated_at DESC, a.created_at DESC
    LIMIT 1
  ) AS latest_analysis_id,
  (
    SELECT commit_sha FROM repo_analysis_results a
    WHERE a.repo_source_id = rs.id
    ORDER BY a.updated_at DESC, a.created_at DESC
    LIMIT 1
  ) AS latest_commit_sha,
  (
    SELECT resolved_ref FROM repo_analysis_results a
    WHERE a.repo_source_id = rs.id
    ORDER BY a.updated_at DESC, a.created_at DESC
    LIMIT 1
  ) AS latest_resolved_ref,
  (
    SELECT status FROM repo_analysis_results a
    WHERE a.repo_source_id = rs.id
    ORDER BY a.updated_at DESC, a.created_at DESC
    LIMIT 1
  ) AS latest_analysis_status,
  (
    SELECT updated_at FROM repo_analysis_results a
    WHERE a.repo_source_id = rs.id
    ORDER BY a.updated_at DESC, a.created_at DESC
    LIMIT 1
  ) AS latest_analysis_updated_at,
  (
    SELECT COUNT(*) FROM repo_knowledge_cards c WHERE c.repo_source_id = rs.id
  ) AS cards_count
FROM repo_sources rs
ORDER BY COALESCE((SELECT MAX(updated_at) FROM repo_analysis_results a WHERE a.repo_source_id = rs.id), rs.updated_at) DESC,
         rs.updated_at DESC
LIMIT ?;`, limit)
	if err != nil {
		return nil, fmt.Errorf("query repo sources: %w", err)
	}
	defer rows.Close()
	out := make([]RepoSourceSummary, 0)
	for rows.Next() {
		item, err := scanRepoSourceSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo sources: %w", err)
	}
	return out, nil
}

// GetRepoSource 功能：读取单个 Repo Library 仓库来源。
// 参数/返回：repoSourceID 为目标 id；返回 RepoSource。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoSource(ctx context.Context, repoSourceID string) (RepoSource, error) {
	if s == nil || s.db == nil {
		return RepoSource{}, fmt.Errorf("store not initialized")
	}
	repoSourceID = strings.TrimSpace(repoSourceID)
	if repoSourceID == "" {
		return RepoSource{}, fmt.Errorf("%w: repository_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_url, owner, repo, repo_key, default_branch, visibility, created_at, updated_at FROM repo_sources WHERE id = ? LIMIT 1;`, repoSourceID)
	source, err := scanRepoSource(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoSource{}, os.ErrNotExist
		}
		return RepoSource{}, fmt.Errorf("query repo source: %w", err)
	}
	return source, nil
}

// GetRepoSourceByRepoKey 功能：按 repo_key 读取单个 Repo Library 仓库来源。
// 参数/返回：repoKey 为规范化仓库键；返回 RepoSource。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoSourceByRepoKey(ctx context.Context, repoKey string) (RepoSource, error) {
	if s == nil || s.db == nil {
		return RepoSource{}, fmt.Errorf("store not initialized")
	}
	repoKey = strings.TrimSpace(repoKey)
	if repoKey == "" {
		return RepoSource{}, fmt.Errorf("%w: repo_key is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_url, owner, repo, repo_key, default_branch, visibility, created_at, updated_at FROM repo_sources WHERE repo_key = ? LIMIT 1;`, repoKey)
	source, err := scanRepoSource(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoSource{}, os.ErrNotExist
		}
		return RepoSource{}, fmt.Errorf("query repo source by repo_key: %w", err)
	}
	return source, nil
}

// GetRepoAnalysisResult 功能：读取单个 Repo Library 分析结果。
// 参数/返回：analysisID 为目标 id；返回 RepoAnalysisResult。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoAnalysisResult(ctx context.Context, analysisID string) (RepoAnalysisResult, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisResult{}, fmt.Errorf("store not initialized")
	}
	analysisID = strings.TrimSpace(analysisID)
	if analysisID == "" {
		return RepoAnalysisResult{}, fmt.Errorf("%w: analysis_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path, execution_id, chat_session_id, chat_user_message_id, chat_assistant_message_id, runtime_kind, cli_tool_id, model_id, status, language, depth, agent_mode, features_json, summary, error_message, result_json, started_at, ended_at, created_at, updated_at FROM repo_analysis_results WHERE id = ? LIMIT 1;`, analysisID)
	result, err := scanRepoAnalysisResult(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoAnalysisResult{}, os.ErrNotExist
		}
		return RepoAnalysisResult{}, fmt.Errorf("query repo analysis result: %w", err)
	}
	return result, nil
}

// ListRepoAnalysisResultsBySource 功能：返回某个仓库来源的分析结果列表。
// 参数/返回：repoSourceID 为目标仓库；返回按时间倒序的分析结果列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoAnalysisResultsBySource(ctx context.Context, repoSourceID string, limit int) ([]RepoAnalysisResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path, execution_id, chat_session_id, chat_user_message_id, chat_assistant_message_id, runtime_kind, cli_tool_id, model_id, status, language, depth, agent_mode, features_json, summary, error_message, result_json, started_at, ended_at, created_at, updated_at FROM repo_analysis_results WHERE repo_source_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?;`, strings.TrimSpace(repoSourceID), limit)
	if err != nil {
		return nil, fmt.Errorf("query repo analyses: %w", err)
	}
	defer rows.Close()
	out := make([]RepoAnalysisResult, 0)
	for rows.Next() {
		item, err := scanRepoAnalysisResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo analyses: %w", err)
	}
	return out, nil
}

// ListRepoCards 功能：按仓库或分析结果过滤返回知识卡片列表。
// 参数/返回：params 提供过滤条件与 limit；返回 RepoKnowledgeCard 列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoCards(ctx context.Context, params ListRepoCardsParams) ([]RepoKnowledgeCard, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	limit := params.Limit
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	query := `SELECT id, repo_source_id, analysis_result_id, title, card_type, summary, mechanism, confidence, tags_json, section_title, sort_index, created_at FROM repo_knowledge_cards`
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if strings.TrimSpace(params.RepoSourceID) != "" {
		clauses = append(clauses, "repo_source_id = ?")
		args = append(args, strings.TrimSpace(params.RepoSourceID))
	}
	if strings.TrimSpace(params.AnalysisID) != "" {
		clauses = append(clauses, "analysis_result_id = ?")
		args = append(args, strings.TrimSpace(params.AnalysisID))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY sort_index ASC, created_at ASC LIMIT ?;"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query repo cards: %w", err)
	}
	defer rows.Close()
	out := make([]RepoKnowledgeCard, 0)
	for rows.Next() {
		item, err := scanRepoKnowledgeCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo cards: %w", err)
	}
	return out, nil
}

// GetRepoCard 功能：读取单个知识卡片。
// 参数/返回：cardID 为目标卡片 id；返回 RepoKnowledgeCard。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoCard(ctx context.Context, cardID string) (RepoKnowledgeCard, error) {
	if s == nil || s.db == nil {
		return RepoKnowledgeCard{}, fmt.Errorf("store not initialized")
	}
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return RepoKnowledgeCard{}, fmt.Errorf("%w: card_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_source_id, analysis_result_id, title, card_type, summary, mechanism, confidence, tags_json, section_title, sort_index, created_at FROM repo_knowledge_cards WHERE id = ? LIMIT 1;`, cardID)
	card, err := scanRepoKnowledgeCard(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoKnowledgeCard{}, os.ErrNotExist
		}
		return RepoKnowledgeCard{}, fmt.Errorf("query repo card: %w", err)
	}
	return card, nil
}

// ListRepoEvidenceByCard 功能：返回某张知识卡片的证据链列表。
// 参数/返回：cardID 为目标卡片；返回按 sort_index 排序的 evidence 列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoEvidenceByCard(ctx context.Context, cardID string) ([]RepoKnowledgeEvidence, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, card_id, path, line, snippet, dimension, sort_index, created_at FROM repo_knowledge_evidence WHERE card_id = ? ORDER BY sort_index ASC, created_at ASC;`, strings.TrimSpace(cardID))
	if err != nil {
		return nil, fmt.Errorf("query repo evidence: %w", err)
	}
	defer rows.Close()
	out := make([]RepoKnowledgeEvidence, 0)
	for rows.Next() {
		item, err := scanRepoKnowledgeEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo evidence: %w", err)
	}
	return out, nil
}

// GetRepoLibraryDetail 功能：聚合返回某个仓库来源的 Repo Library 详情。
// 参数/返回：repoSourceID 为目标仓库；返回 RepoLibraryDetail。
// 失败场景：仓库不存在或子查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoLibraryDetail(ctx context.Context, repoSourceID string) (RepoLibraryDetail, error) {
	repository, err := s.GetRepoSource(ctx, repoSourceID)
	if err != nil {
		return RepoLibraryDetail{}, err
	}
	analyses, err := s.ListRepoAnalysisResultsBySource(ctx, repoSourceID, 50)
	if err != nil {
		return RepoLibraryDetail{}, err
	}
	cards, err := s.ListRepoCards(ctx, ListRepoCardsParams{RepoSourceID: repoSourceID, Limit: 500})
	if err != nil {
		return RepoLibraryDetail{}, err
	}
	return RepoLibraryDetail{Repository: repository, Analyses: analyses, Cards: cards}, nil
}

func scanRepoSource(scanner repoLibraryScanner) (RepoSource, error) {
	var source RepoSource
	var defaultBranch sql.NullString
	var visibility sql.NullString
	if err := scanner.Scan(&source.ID, &source.RepoURL, &source.Owner, &source.Repo, &source.RepoKey, &defaultBranch, &visibility, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return RepoSource{}, err
	}
	source.DefaultBranch = nullStringPtr(defaultBranch)
	source.Visibility = nullStringPtr(visibility)
	return source, nil
}

func scanRepoAnalysisResult(scanner repoLibraryScanner) (RepoAnalysisResult, error) {
	var result RepoAnalysisResult
	var resolvedRef sql.NullString
	var commitSHA sql.NullString
	var reportPath sql.NullString
	var subagentResultsPath sql.NullString
	var executionID sql.NullString
	var chatSessionID sql.NullString
	var chatUserMessageID sql.NullString
	var chatAssistantMessageID sql.NullString
	var runtimeKind sql.NullString
	var cliToolID sql.NullString
	var modelID sql.NullString
	var featuresJSON string
	var summary sql.NullString
	var errorMessage sql.NullString
	var resultJSON sql.NullString
	var startedAt sql.NullInt64
	var endedAt sql.NullInt64
	if err := scanner.Scan(&result.ID, &result.RepoSourceID, &result.RequestedRef, &resolvedRef, &commitSHA, &result.StoragePath, &reportPath, &subagentResultsPath, &executionID, &chatSessionID, &chatUserMessageID, &chatAssistantMessageID, &runtimeKind, &cliToolID, &modelID, &result.Status, &result.Language, &result.Depth, &result.AgentMode, &featuresJSON, &summary, &errorMessage, &resultJSON, &startedAt, &endedAt, &result.CreatedAt, &result.UpdatedAt); err != nil {
		return RepoAnalysisResult{}, err
	}
	result.ResolvedRef = nullStringPtr(resolvedRef)
	result.CommitSHA = nullStringPtr(commitSHA)
	result.ReportPath = nullStringPtr(reportPath)
	result.SubagentResultsPath = nullStringPtr(subagentResultsPath)
	result.ExecutionID = nullStringPtr(executionID)
	result.ChatSessionID = nullStringPtr(chatSessionID)
	result.ChatUserMessageID = nullStringPtr(chatUserMessageID)
	result.ChatAssistantMessageID = nullStringPtr(chatAssistantMessageID)
	result.RuntimeKind = nullStringPtr(runtimeKind)
	result.CLIToolID = nullStringPtr(cliToolID)
	result.ModelID = nullStringPtr(modelID)
	result.Summary = nullStringPtr(summary)
	result.ErrorMessage = nullStringPtr(errorMessage)
	result.ResultJSON = nullStringPtr(resultJSON)
	result.StartedAt = nullInt64Ptr(startedAt)
	result.EndedAt = nullInt64Ptr(endedAt)
	if err := json.Unmarshal([]byte(featuresJSON), &result.Features); err != nil {
		result.Features = nil
	}
	return result, nil
}

func scanRepoKnowledgeCard(scanner repoLibraryScanner) (RepoKnowledgeCard, error) {
	var card RepoKnowledgeCard
	var mechanism sql.NullString
	var confidence sql.NullString
	var tagsJSON sql.NullString
	var sectionTitle sql.NullString
	if err := scanner.Scan(&card.ID, &card.RepoSourceID, &card.AnalysisID, &card.Title, &card.CardType, &card.Summary, &mechanism, &confidence, &tagsJSON, &sectionTitle, &card.SortIndex, &card.CreatedAt); err != nil {
		return RepoKnowledgeCard{}, err
	}
	card.Mechanism = nullStringPtr(mechanism)
	card.Confidence = nullStringPtr(confidence)
	card.SectionTitle = nullStringPtr(sectionTitle)
	if tagsJSON.Valid && strings.TrimSpace(tagsJSON.String) != "" {
		if err := json.Unmarshal([]byte(tagsJSON.String), &card.Tags); err != nil {
			card.Tags = nil
		}
	}
	return card, nil
}

func scanRepoKnowledgeEvidence(scanner repoLibraryScanner) (RepoKnowledgeEvidence, error) {
	var evidence RepoKnowledgeEvidence
	var line sql.NullInt64
	var snippet sql.NullString
	var dimension sql.NullString
	if err := scanner.Scan(&evidence.ID, &evidence.CardID, &evidence.Path, &line, &snippet, &dimension, &evidence.SortIndex, &evidence.CreatedAt); err != nil {
		return RepoKnowledgeEvidence{}, err
	}
	evidence.Line = nullInt64Ptr(line)
	evidence.Snippet = nullStringPtr(snippet)
	evidence.Dimension = nullStringPtr(dimension)
	return evidence, nil
}

func scanRepoSourceSummary(scanner repoLibraryScanner) (RepoSourceSummary, error) {
	var item RepoSourceSummary
	var defaultBranch sql.NullString
	var visibility sql.NullString
	var latestAnalysisID sql.NullString
	var latestCommitSHA sql.NullString
	var latestResolvedRef sql.NullString
	var latestAnalysisStatus sql.NullString
	var latestAnalysisUpdated sql.NullInt64
	if err := scanner.Scan(&item.ID, &item.RepoURL, &item.Owner, &item.Repo, &item.RepoKey, &defaultBranch, &visibility, &item.CreatedAt, &item.UpdatedAt, &latestAnalysisID, &latestCommitSHA, &latestResolvedRef, &latestAnalysisStatus, &latestAnalysisUpdated, &item.CardsCount); err != nil {
		return RepoSourceSummary{}, err
	}
	item.DefaultBranch = nullStringPtr(defaultBranch)
	item.Visibility = nullStringPtr(visibility)
	item.LatestAnalysisID = nullStringPtr(latestAnalysisID)
	item.LatestCommitSHA = nullStringPtr(latestCommitSHA)
	item.LatestResolvedRef = nullStringPtr(latestResolvedRef)
	item.LatestAnalysisStatus = nullStringPtr(latestAnalysisStatus)
	item.LatestAnalysisUpdated = nullInt64Ptr(latestAnalysisUpdated)
	return item, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(v.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	copy := v.Int64
	return &copy
}

func uniqueTrimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func firstNonEmptyString(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
