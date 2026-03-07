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

	"vibe-tree/backend/internal/id"
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

type RepoSnapshot struct {
	ID                 string  `json:"snapshot_id"`
	RepoSourceID       string  `json:"repository_id"`
	RequestedRef       string  `json:"requested_ref"`
	ResolvedRef        *string `json:"resolved_ref,omitempty"`
	CommitSHA          *string `json:"commit_sha,omitempty"`
	StoragePath        string  `json:"storage_path"`
	ReportPath         *string `json:"report_path,omitempty"`
	SubagentResultsPath *string `json:"subagent_results_path,omitempty"`
	CreatedAt          int64   `json:"created_at"`
	UpdatedAt          int64   `json:"updated_at"`
}

type RepoAnalysisRun struct {
	ID            string   `json:"analysis_run_id"`
	RepoSourceID  string   `json:"repository_id"`
	RepoSnapshotID string  `json:"snapshot_id"`
	ExecutionID   *string  `json:"execution_id,omitempty"`
	Status        string   `json:"status"`
	Language      string   `json:"language"`
	Depth         string   `json:"depth"`
	AgentMode     string   `json:"agent_mode"`
	Features      []string `json:"features"`
	Summary       *string  `json:"summary,omitempty"`
	ErrorMessage  *string  `json:"error_message,omitempty"`
	ResultJSON    *string  `json:"result_json,omitempty"`
	ReportPath    *string  `json:"report_path,omitempty"`
	StartedAt     *int64   `json:"started_at,omitempty"`
	EndedAt       *int64   `json:"ended_at,omitempty"`
	CreatedAt     int64    `json:"created_at"`
	UpdatedAt     int64    `json:"updated_at"`
}

type RepoKnowledgeCard struct {
	ID            string   `json:"card_id"`
	RepoSourceID  string   `json:"repository_id"`
	RepoSnapshotID string  `json:"snapshot_id"`
	AnalysisRunID string   `json:"analysis_run_id"`
	Title         string   `json:"title"`
	CardType      string   `json:"card_type"`
	Summary       string   `json:"summary"`
	Mechanism     *string  `json:"mechanism,omitempty"`
	Confidence    *string  `json:"confidence,omitempty"`
	Tags          []string `json:"tags"`
	SectionTitle  *string  `json:"section_title,omitempty"`
	SortIndex     int      `json:"sort_index"`
	CreatedAt     int64    `json:"created_at"`
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

type RepoSourceSummary struct {
	RepoSource
	LatestSnapshotID      *string `json:"latest_snapshot_id,omitempty"`
	LatestCommitSHA       *string `json:"latest_commit_sha,omitempty"`
	LatestResolvedRef     *string `json:"latest_resolved_ref,omitempty"`
	LatestAnalysisRunID   *string `json:"latest_analysis_run_id,omitempty"`
	LatestAnalysisStatus  *string `json:"latest_analysis_status,omitempty"`
	LatestAnalysisUpdated *int64  `json:"latest_analysis_updated_at,omitempty"`
	CardsCount            int64   `json:"cards_count"`
}

type RepoLibraryDetail struct {
	Repository RepoSource          `json:"repository"`
	Snapshots  []RepoSnapshot      `json:"snapshots"`
	Runs       []RepoAnalysisRun   `json:"analysis_runs"`
	Cards      []RepoKnowledgeCard `json:"cards"`
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

type CreateRepoSnapshotParams struct {
	SnapshotID    string
	RepoSourceID string
	RequestedRef string
	StoragePath  string
}

type UpdateRepoSnapshotParams struct {
	SnapshotID          string
	StoragePath         *string
	ResolvedRef         *string
	CommitSHA           *string
	ReportPath          *string
	SubagentResultsPath *string
}

type CreateRepoAnalysisRunParams struct {
	RepoSourceID   string
	RepoSnapshotID string
	Language       string
	Depth          string
	AgentMode      string
	Features       []string
}

type StartRepoAnalysisRunParams struct {
	RunID       string
	ExecutionID string
}

type FinalizeRepoAnalysisRunParams struct {
	RunID        string
	Status       string
	Summary      *string
	ErrorMessage *string
	ResultJSON   *string
	ReportPath   *string
}

type ReplaceRepoKnowledgeParams struct {
	RepoSourceID   string
	RepoSnapshotID string
	AnalysisRunID  string
	Cards          []RepoKnowledgeCardInput
}

type RepoKnowledgeCardInput struct {
	Title        string
	CardType     string
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
	RepoSourceID   string
	RepoSnapshotID string
	AnalysisRunID  string
	Limit          int
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

// CreateRepoSnapshot 功能：为 Repo Library 创建一次待分析快照记录。
// 参数/返回：params 提供来源仓库、请求 ref 与存储路径；返回 RepoSnapshot。
// 失败场景：关键字段缺失或写库失败时返回 error。
// 副作用：写入 SQLite `repo_snapshots`。
func (s *Store) CreateRepoSnapshot(ctx context.Context, params CreateRepoSnapshotParams) (RepoSnapshot, error) {
	if s == nil || s.db == nil {
		return RepoSnapshot{}, fmt.Errorf("store not initialized")
	}
	repoSourceID := strings.TrimSpace(params.RepoSourceID)
	requestedRef := strings.TrimSpace(params.RequestedRef)
	storagePath := strings.TrimSpace(params.StoragePath)
	if repoSourceID == "" || requestedRef == "" || storagePath == "" {
		return RepoSnapshot{}, fmt.Errorf("%w: repo_source_id/requested_ref/storage_path are required", ErrValidation)
	}
	now := time.Now().UnixMilli()
	snapshotID := strings.TrimSpace(params.SnapshotID)
	if snapshotID == "" {
		snapshotID = id.New("rp_")
	}
	snapshot := RepoSnapshot{ID: snapshotID, RepoSourceID: repoSourceID, RequestedRef: requestedRef, StoragePath: storagePath, CreatedAt: now, UpdatedAt: now}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repo_snapshots (id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path, created_at, updated_at) VALUES (?, ?, ?, NULL, NULL, ?, NULL, NULL, ?, ?);`, snapshot.ID, snapshot.RepoSourceID, snapshot.RequestedRef, snapshot.StoragePath, snapshot.CreatedAt, snapshot.UpdatedAt); err != nil {
		return RepoSnapshot{}, fmt.Errorf("insert repo snapshot: %w", err)
	}
	return snapshot, nil
}

// UpdateRepoSnapshot 功能：更新 Repo Library 快照的解析结果路径与 commit 信息。
// 参数/返回：params 指定 snapshot id 与解析字段；返回更新后的 RepoSnapshot。
// 失败场景：snapshot 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_snapshots`。
func (s *Store) UpdateRepoSnapshot(ctx context.Context, params UpdateRepoSnapshotParams) (RepoSnapshot, error) {
	if s == nil || s.db == nil {
		return RepoSnapshot{}, fmt.Errorf("store not initialized")
	}
	snapshotID := strings.TrimSpace(params.SnapshotID)
	if snapshotID == "" {
		return RepoSnapshot{}, fmt.Errorf("%w: snapshot_id is required", ErrValidation)
	}
	snapshot, err := s.GetRepoSnapshot(ctx, snapshotID)
	if err != nil {
		return RepoSnapshot{}, err
	}
	if storagePath := trimOrNil(params.StoragePath); storagePath != nil {
		snapshot.StoragePath = *storagePath
	}
	snapshot.ResolvedRef = trimOrNil(params.ResolvedRef)
	snapshot.CommitSHA = trimOrNil(params.CommitSHA)
	snapshot.ReportPath = trimOrNil(params.ReportPath)
	snapshot.SubagentResultsPath = trimOrNil(params.SubagentResultsPath)
	snapshot.UpdatedAt = time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_snapshots SET storage_path = ?, resolved_ref = ?, commit_sha = ?, report_path = ?, subagent_results_path = ?, updated_at = ? WHERE id = ?;`, snapshot.StoragePath, snapshot.ResolvedRef, snapshot.CommitSHA, snapshot.ReportPath, snapshot.SubagentResultsPath, snapshot.UpdatedAt, snapshot.ID); err != nil {
		return RepoSnapshot{}, fmt.Errorf("update repo snapshot: %w", err)
	}
	return snapshot, nil
}

// CreateRepoAnalysisRun 功能：为 Repo Library 创建分析运行记录。
// 参数/返回：params 提供 snapshot、语言、深度与 feature 列表；返回 RepoAnalysisRun。
// 失败场景：关键字段缺失、features 为空或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_runs`。
func (s *Store) CreateRepoAnalysisRun(ctx context.Context, params CreateRepoAnalysisRunParams) (RepoAnalysisRun, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisRun{}, fmt.Errorf("store not initialized")
	}
	if strings.TrimSpace(params.RepoSourceID) == "" || strings.TrimSpace(params.RepoSnapshotID) == "" {
		return RepoAnalysisRun{}, fmt.Errorf("%w: repo_source_id/repo_snapshot_id are required", ErrValidation)
	}
	features := uniqueTrimmedStrings(params.Features)
	if len(features) == 0 {
		return RepoAnalysisRun{}, fmt.Errorf("%w: at least one feature is required", ErrValidation)
	}
	featuresJSON, err := json.Marshal(features)
	if err != nil {
		return RepoAnalysisRun{}, fmt.Errorf("marshal features: %w", err)
	}
	now := time.Now().UnixMilli()
	run := RepoAnalysisRun{
		ID:             id.New("rr_"),
		RepoSourceID:   strings.TrimSpace(params.RepoSourceID),
		RepoSnapshotID: strings.TrimSpace(params.RepoSnapshotID),
		Status:         string(RepoAnalysisStatusQueued),
		Language:       firstNonEmptyString(params.Language, "zh"),
		Depth:          firstNonEmptyString(params.Depth, "standard"),
		AgentMode:      firstNonEmptyString(params.AgentMode, "single"),
		Features:       features,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO repo_analysis_runs (id, repo_source_id, repo_snapshot_id, execution_id, status, language, depth, agent_mode, features_json, summary, error_message, result_json, report_path, started_at, ended_at, created_at, updated_at) VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, NULL, NULL, ?, ?);`, run.ID, run.RepoSourceID, run.RepoSnapshotID, run.Status, run.Language, run.Depth, run.AgentMode, string(featuresJSON), run.CreatedAt, run.UpdatedAt); err != nil {
		return RepoAnalysisRun{}, fmt.Errorf("insert repo analysis run: %w", err)
	}
	return run, nil
}

// StartRepoAnalysisRun 功能：将分析运行标记为 running，并记录 execution_id。
// 参数/返回：params 指定 run id 与 execution id；返回更新后的 RepoAnalysisRun。
// 失败场景：run 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_runs`。
func (s *Store) StartRepoAnalysisRun(ctx context.Context, params StartRepoAnalysisRunParams) (RepoAnalysisRun, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisRun{}, fmt.Errorf("store not initialized")
	}
	runID := strings.TrimSpace(params.RunID)
	executionID := strings.TrimSpace(params.ExecutionID)
	if runID == "" || executionID == "" {
		return RepoAnalysisRun{}, fmt.Errorf("%w: run_id/execution_id are required", ErrValidation)
	}
	run, err := s.GetRepoAnalysisRun(ctx, runID)
	if err != nil {
		return RepoAnalysisRun{}, err
	}
	now := time.Now().UnixMilli()
	run.ExecutionID = &executionID
	run.Status = string(RepoAnalysisStatusRunning)
	run.StartedAt = &now
	run.UpdatedAt = now
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_runs SET execution_id = ?, status = ?, started_at = ?, updated_at = ? WHERE id = ?;`, executionID, run.Status, run.StartedAt, run.UpdatedAt, run.ID); err != nil {
		return RepoAnalysisRun{}, fmt.Errorf("start repo analysis run: %w", err)
	}
	return run, nil
}

// FinalizeRepoAnalysisRun 功能：收敛分析运行终态并写入结果路径摘要。
// 参数/返回：params 指定终态、摘要与错误；返回更新后的 RepoAnalysisRun。
// 失败场景：run 不存在或写库失败时返回 error。
// 副作用：写入 SQLite `repo_analysis_runs`。
func (s *Store) FinalizeRepoAnalysisRun(ctx context.Context, params FinalizeRepoAnalysisRunParams) (RepoAnalysisRun, error) {
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
	now := time.Now().UnixMilli()
	run.Status = strings.TrimSpace(params.Status)
	if run.Status == "" {
		run.Status = string(RepoAnalysisStatusFailed)
	}
	run.Summary = trimOrNil(params.Summary)
	run.ErrorMessage = trimOrNil(params.ErrorMessage)
	run.ResultJSON = trimOrNil(params.ResultJSON)
	run.ReportPath = trimOrNil(params.ReportPath)
	run.EndedAt = &now
	run.UpdatedAt = now
	if _, err := s.db.ExecContext(ctx, `UPDATE repo_analysis_runs SET status = ?, summary = ?, error_message = ?, result_json = ?, report_path = ?, ended_at = ?, updated_at = ? WHERE id = ?;`, run.Status, run.Summary, run.ErrorMessage, run.ResultJSON, run.ReportPath, run.EndedAt, run.UpdatedAt, run.ID); err != nil {
		return RepoAnalysisRun{}, fmt.Errorf("finalize repo analysis run: %w", err)
	}
	return run, nil
}

// ReplaceRepoKnowledge 功能：用最新抽取结果替换某次分析运行下的知识卡片与证据。
// 参数/返回：params 提供 source/snapshot/run 与卡片输入；无返回值。
// 失败场景：写库失败时返回 error。
// 副作用：删除旧卡片与 evidence，并插入新结果。
func (s *Store) ReplaceRepoKnowledge(ctx context.Context, params ReplaceRepoKnowledgeParams) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if strings.TrimSpace(params.RepoSourceID) == "" || strings.TrimSpace(params.RepoSnapshotID) == "" || strings.TrimSpace(params.AnalysisRunID) == "" {
		return fmt.Errorf("%w: repo_source_id/repo_snapshot_id/analysis_run_id are required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_knowledge_evidence WHERE card_id IN (SELECT id FROM repo_knowledge_cards WHERE analysis_run_id = ?);`, params.AnalysisRunID); err != nil {
		return fmt.Errorf("delete repo evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_knowledge_cards WHERE analysis_run_id = ?;`, params.AnalysisRunID); err != nil {
		return fmt.Errorf("delete repo cards: %w", err)
	}
	now := time.Now().UnixMilli()
	for idx, input := range params.Cards {
		card := RepoKnowledgeCard{
			ID:             id.New("rc_"),
			RepoSourceID:   params.RepoSourceID,
			RepoSnapshotID: params.RepoSnapshotID,
			AnalysisRunID:  params.AnalysisRunID,
			Title:          strings.TrimSpace(input.Title),
			CardType:       strings.TrimSpace(input.CardType),
			Summary:        strings.TrimSpace(input.Summary),
			Mechanism:      trimOrNil(input.Mechanism),
			Confidence:     trimOrNil(input.Confidence),
			Tags:           uniqueTrimmedStrings(input.Tags),
			SectionTitle:   trimOrNil(input.SectionTitle),
			SortIndex:      input.SortIndex,
			CreatedAt:      now,
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO repo_knowledge_cards (id, repo_source_id, repo_snapshot_id, analysis_run_id, title, card_type, summary, mechanism, confidence, tags_json, section_title, sort_index, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, card.ID, card.RepoSourceID, card.RepoSnapshotID, card.AnalysisRunID, card.Title, card.CardType, card.Summary, card.Mechanism, card.Confidence, string(tagsJSON), card.SectionTitle, card.SortIndex, card.CreatedAt); err != nil {
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
    SELECT id FROM repo_snapshots s
    WHERE s.repo_source_id = rs.id
    ORDER BY s.updated_at DESC, s.created_at DESC
    LIMIT 1
  ) AS latest_snapshot_id,
  (
    SELECT commit_sha FROM repo_snapshots s
    WHERE s.repo_source_id = rs.id
    ORDER BY s.updated_at DESC, s.created_at DESC
    LIMIT 1
  ) AS latest_commit_sha,
  (
    SELECT resolved_ref FROM repo_snapshots s
    WHERE s.repo_source_id = rs.id
    ORDER BY s.updated_at DESC, s.created_at DESC
    LIMIT 1
  ) AS latest_resolved_ref,
  (
    SELECT id FROM repo_analysis_runs r
    WHERE r.repo_source_id = rs.id
    ORDER BY r.updated_at DESC, r.created_at DESC
    LIMIT 1
  ) AS latest_analysis_run_id,
  (
    SELECT status FROM repo_analysis_runs r
    WHERE r.repo_source_id = rs.id
    ORDER BY r.updated_at DESC, r.created_at DESC
    LIMIT 1
  ) AS latest_analysis_status,
  (
    SELECT updated_at FROM repo_analysis_runs r
    WHERE r.repo_source_id = rs.id
    ORDER BY r.updated_at DESC, r.created_at DESC
    LIMIT 1
  ) AS latest_analysis_updated_at,
  (
    SELECT COUNT(*) FROM repo_knowledge_cards c WHERE c.repo_source_id = rs.id
  ) AS cards_count
FROM repo_sources rs
ORDER BY COALESCE((SELECT MAX(updated_at) FROM repo_analysis_runs r WHERE r.repo_source_id = rs.id), rs.updated_at) DESC,
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

// GetRepoSnapshot 功能：读取单个 Repo Library 快照。
// 参数/返回：snapshotID 为目标 id；返回 RepoSnapshot。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoSnapshot(ctx context.Context, snapshotID string) (RepoSnapshot, error) {
	if s == nil || s.db == nil {
		return RepoSnapshot{}, fmt.Errorf("store not initialized")
	}
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return RepoSnapshot{}, fmt.Errorf("%w: snapshot_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path, created_at, updated_at FROM repo_snapshots WHERE id = ? LIMIT 1;`, snapshotID)
	snapshot, err := scanRepoSnapshot(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoSnapshot{}, os.ErrNotExist
		}
		return RepoSnapshot{}, fmt.Errorf("query repo snapshot: %w", err)
	}
	return snapshot, nil
}

// GetRepoAnalysisRun 功能：读取单个 Repo Library 分析运行。
// 参数/返回：runID 为目标 id；返回 RepoAnalysisRun。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetRepoAnalysisRun(ctx context.Context, runID string) (RepoAnalysisRun, error) {
	if s == nil || s.db == nil {
		return RepoAnalysisRun{}, fmt.Errorf("store not initialized")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RepoAnalysisRun{}, fmt.Errorf("%w: analysis_run_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_source_id, repo_snapshot_id, execution_id, status, language, depth, agent_mode, features_json, summary, error_message, result_json, report_path, started_at, ended_at, created_at, updated_at FROM repo_analysis_runs WHERE id = ? LIMIT 1;`, runID)
	run, err := scanRepoAnalysisRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RepoAnalysisRun{}, os.ErrNotExist
		}
		return RepoAnalysisRun{}, fmt.Errorf("query repo analysis run: %w", err)
	}
	return run, nil
}

// ListRepoSnapshotsBySource 功能：返回某个仓库来源的快照列表。
// 参数/返回：repoSourceID 为目标仓库；返回按时间倒序的快照列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoSnapshotsBySource(ctx context.Context, repoSourceID string, limit int) ([]RepoSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, repo_source_id, requested_ref, resolved_ref, commit_sha, storage_path, report_path, subagent_results_path, created_at, updated_at FROM repo_snapshots WHERE repo_source_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?;`, strings.TrimSpace(repoSourceID), limit)
	if err != nil {
		return nil, fmt.Errorf("query repo snapshots: %w", err)
	}
	defer rows.Close()
	out := make([]RepoSnapshot, 0)
	for rows.Next() {
		item, err := scanRepoSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo snapshots: %w", err)
	}
	return out, nil
}

// ListRepoAnalysisRunsBySource 功能：返回某个仓库来源的分析运行列表。
// 参数/返回：repoSourceID 为目标仓库；返回按时间倒序的分析运行列表。
// 失败场景：查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRepoAnalysisRunsBySource(ctx context.Context, repoSourceID string, limit int) ([]RepoAnalysisRun, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, repo_source_id, repo_snapshot_id, execution_id, status, language, depth, agent_mode, features_json, summary, error_message, result_json, report_path, started_at, ended_at, created_at, updated_at FROM repo_analysis_runs WHERE repo_source_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?;`, strings.TrimSpace(repoSourceID), limit)
	if err != nil {
		return nil, fmt.Errorf("query repo analysis runs: %w", err)
	}
	defer rows.Close()
	out := make([]RepoAnalysisRun, 0)
	for rows.Next() {
		item, err := scanRepoAnalysisRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo analysis runs: %w", err)
	}
	return out, nil
}

// ListRepoCards 功能：按仓库、快照或运行过滤返回知识卡片列表。
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
	query := `SELECT id, repo_source_id, repo_snapshot_id, analysis_run_id, title, card_type, summary, mechanism, confidence, tags_json, section_title, sort_index, created_at FROM repo_knowledge_cards`
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if strings.TrimSpace(params.RepoSourceID) != "" {
		clauses = append(clauses, "repo_source_id = ?")
		args = append(args, strings.TrimSpace(params.RepoSourceID))
	}
	if strings.TrimSpace(params.RepoSnapshotID) != "" {
		clauses = append(clauses, "repo_snapshot_id = ?")
		args = append(args, strings.TrimSpace(params.RepoSnapshotID))
	}
	if strings.TrimSpace(params.AnalysisRunID) != "" {
		clauses = append(clauses, "analysis_run_id = ?")
		args = append(args, strings.TrimSpace(params.AnalysisRunID))
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
	row := s.db.QueryRowContext(ctx, `SELECT id, repo_source_id, repo_snapshot_id, analysis_run_id, title, card_type, summary, mechanism, confidence, tags_json, section_title, sort_index, created_at FROM repo_knowledge_cards WHERE id = ? LIMIT 1;`, cardID)
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
	snapshots, err := s.ListRepoSnapshotsBySource(ctx, repoSourceID, 50)
	if err != nil {
		return RepoLibraryDetail{}, err
	}
	runs, err := s.ListRepoAnalysisRunsBySource(ctx, repoSourceID, 50)
	if err != nil {
		return RepoLibraryDetail{}, err
	}
	cards, err := s.ListRepoCards(ctx, ListRepoCardsParams{RepoSourceID: repoSourceID, Limit: 500})
	if err != nil {
		return RepoLibraryDetail{}, err
	}
	return RepoLibraryDetail{Repository: repository, Snapshots: snapshots, Runs: runs, Cards: cards}, nil
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

func scanRepoSnapshot(scanner repoLibraryScanner) (RepoSnapshot, error) {
	var snapshot RepoSnapshot
	var resolvedRef sql.NullString
	var commitSHA sql.NullString
	var reportPath sql.NullString
	var subagentResultsPath sql.NullString
	if err := scanner.Scan(&snapshot.ID, &snapshot.RepoSourceID, &snapshot.RequestedRef, &resolvedRef, &commitSHA, &snapshot.StoragePath, &reportPath, &subagentResultsPath, &snapshot.CreatedAt, &snapshot.UpdatedAt); err != nil {
		return RepoSnapshot{}, err
	}
	snapshot.ResolvedRef = nullStringPtr(resolvedRef)
	snapshot.CommitSHA = nullStringPtr(commitSHA)
	snapshot.ReportPath = nullStringPtr(reportPath)
	snapshot.SubagentResultsPath = nullStringPtr(subagentResultsPath)
	return snapshot, nil
}

func scanRepoAnalysisRun(scanner repoLibraryScanner) (RepoAnalysisRun, error) {
	var run RepoAnalysisRun
	var executionID sql.NullString
	var featuresJSON string
	var summary sql.NullString
	var errorMessage sql.NullString
	var resultJSON sql.NullString
	var reportPath sql.NullString
	var startedAt sql.NullInt64
	var endedAt sql.NullInt64
	if err := scanner.Scan(&run.ID, &run.RepoSourceID, &run.RepoSnapshotID, &executionID, &run.Status, &run.Language, &run.Depth, &run.AgentMode, &featuresJSON, &summary, &errorMessage, &resultJSON, &reportPath, &startedAt, &endedAt, &run.CreatedAt, &run.UpdatedAt); err != nil {
		return RepoAnalysisRun{}, err
	}
	run.ExecutionID = nullStringPtr(executionID)
	run.Summary = nullStringPtr(summary)
	run.ErrorMessage = nullStringPtr(errorMessage)
	run.ResultJSON = nullStringPtr(resultJSON)
	run.ReportPath = nullStringPtr(reportPath)
	run.StartedAt = nullInt64Ptr(startedAt)
	run.EndedAt = nullInt64Ptr(endedAt)
	if err := json.Unmarshal([]byte(featuresJSON), &run.Features); err != nil {
		run.Features = nil
	}
	return run, nil
}

func scanRepoKnowledgeCard(scanner repoLibraryScanner) (RepoKnowledgeCard, error) {
	var card RepoKnowledgeCard
	var mechanism sql.NullString
	var confidence sql.NullString
	var tagsJSON sql.NullString
	var sectionTitle sql.NullString
	if err := scanner.Scan(&card.ID, &card.RepoSourceID, &card.RepoSnapshotID, &card.AnalysisRunID, &card.Title, &card.CardType, &card.Summary, &mechanism, &confidence, &tagsJSON, &sectionTitle, &card.SortIndex, &card.CreatedAt); err != nil {
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
	var latestSnapshotID sql.NullString
	var latestCommitSHA sql.NullString
	var latestResolvedRef sql.NullString
	var latestAnalysisRunID sql.NullString
	var latestAnalysisStatus sql.NullString
	var latestAnalysisUpdated sql.NullInt64
	if err := scanner.Scan(&item.ID, &item.RepoURL, &item.Owner, &item.Repo, &item.RepoKey, &defaultBranch, &visibility, &item.CreatedAt, &item.UpdatedAt, &latestSnapshotID, &latestCommitSHA, &latestResolvedRef, &latestAnalysisRunID, &latestAnalysisStatus, &latestAnalysisUpdated, &item.CardsCount); err != nil {
		return RepoSourceSummary{}, err
	}
	item.DefaultBranch = nullStringPtr(defaultBranch)
	item.Visibility = nullStringPtr(visibility)
	item.LatestSnapshotID = nullStringPtr(latestSnapshotID)
	item.LatestCommitSHA = nullStringPtr(latestCommitSHA)
	item.LatestResolvedRef = nullStringPtr(latestResolvedRef)
	item.LatestAnalysisRunID = nullStringPtr(latestAnalysisRunID)
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
