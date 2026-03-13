package repolib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"vibecraft/backend/internal/chat"
	"vibecraft/backend/internal/execution"
	"vibecraft/backend/internal/expert"
	"vibecraft/backend/internal/id"
	"vibecraft/backend/internal/logx"
	"vibecraft/backend/internal/paths"
	"vibecraft/backend/internal/repolib/searchdb"
	"vibecraft/backend/internal/store"
)

type CreateAnalysisParams struct {
	RepoURL   string
	Ref       string
	Features  []string
	Depth     string
	Language  string
	AgentMode string
	CLIToolID string
	ModelID   string
}

type CreateAnalysisResult struct {
	Repository store.RepoSource         `json:"repository"`
	Analysis   store.RepoAnalysisResult `json:"analysis"`
}

type analysisStatusUpdatePayload struct {
	RepositoryID string `json:"repository_id"`
	AnalysisID   string `json:"analysis_id"`
	Status       string `json:"status"`
	UpdatedAt    int64  `json:"updated_at"`
}

type SearchParams struct {
	Query       string
	RepoFilters []string
	Mode        string
	TopK        int
}

type Service struct {
	store       *store.Store
	executions  *execution.Manager
	chat        *chat.Manager
	experts     *expert.Registry
	projectRoot string
	pythonBin   string
	searchIdx   *searchIndex
	stream      *SSEBroker
}

type analysisLayout struct {
	AnalysisDir      string
	SourceDir        string
	ArtifactsDir     string
	DerivedDir       string
	ReportPath       string
	ResultPath       string
	CardsPath        string
	SearchOutputPath string
}

// NewService 功能：创建 Repo Library service，用于提交分析、读取详情与执行搜索。
// 参数/返回：依赖 state store 与 execution manager；返回初始化后的 Service。
// 失败场景：项目根目录或 Repo Library 根目录解析失败时返回 error。
// 副作用：确保 Repo Library 数据目录存在。
func NewService(stateStore *store.Store, execMgr *execution.Manager, chatMgr *chat.Manager, experts *expert.Registry, stream *SSEBroker) (*Service, error) {
	projectRoot, err := discoverProjectRoot()
	if err != nil {
		return nil, err
	}
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return nil, err
	}
	if err := paths.EnsureDir(repoLibraryDir); err != nil {
		return nil, err
	}
	return &Service{store: stateStore, executions: execMgr, chat: chatMgr, experts: experts, projectRoot: projectRoot, pythonBin: "python3", stream: stream}, nil
}

// CreateAnalysis 功能：创建并启动一条 Repo Library 分析结果。
// 参数/返回：params 提供仓库、ref、features 与 CLI tool/model 选择；返回 repository/analysis 聚合结果。
// 失败场景：校验失败、依赖缺失或初始化失败时返回 error。
// 副作用：写入 SQLite、准备 Repo Library analysis 存储目录，并在后台启动真实 AI Chat 分析流程。
func (s *Service) CreateAnalysis(ctx context.Context, params CreateAnalysisParams) (CreateAnalysisResult, error) {
	if s == nil || s.store == nil || s.chat == nil || s.experts == nil {
		return CreateAnalysisResult{}, fmt.Errorf("repo library service not configured")
	}
	parsed, err := parseGitHubRepoURL(params.RepoURL)
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	features := uniqueTrimmed(params.Features)
	if len(features) == 0 {
		return CreateAnalysisResult{}, fmt.Errorf("%w: at least one feature is required", store.ErrValidation)
	}
	ref := strings.TrimSpace(params.Ref)
	if ref == "" {
		ref = "main"
	}
	source, err := s.store.UpsertRepoSource(ctx, store.UpsertRepoSourceParams{
		RepoURL:       parsed.RepoURL,
		Owner:         parsed.Owner,
		Repo:          parsed.Repo,
		RepoKey:       parsed.RepoKey,
		Visibility:    pointer("public"),
		DefaultBranch: nil,
	})
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	analysisID := id.New("ra_")
	layout, err := s.prepareAnalysisLayoutForID(source.RepoKey, analysisID)
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	for _, dir := range []string{layout.AnalysisDir, layout.SourceDir, layout.ArtifactsDir, layout.DerivedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return CreateAnalysisResult{}, err
		}
	}
	runtimeKind := pointer("ai_chat")
	analysis, err := s.store.CreateRepoAnalysisResult(ctx, store.CreateRepoAnalysisResultParams{
		AnalysisID:   analysisID,
		RepoSourceID: source.ID,
		RequestedRef: ref,
		StoragePath:  layout.AnalysisDir,
		ReportPath:   &layout.ReportPath,
		Language:     firstNonEmpty(params.Language, "zh"),
		Depth:        firstNonEmpty(params.Depth, "standard"),
		AgentMode:    firstNonEmpty(params.AgentMode, "single"),
		Features:     features,
		RuntimeKind:  runtimeKind,
		CLIToolID:    stringPtrIfNotEmpty(params.CLIToolID),
		ModelID:      stringPtrIfNotEmpty(params.ModelID),
	})
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	s.broadcastAnalysisStatus(source.ID, analysis)
	go s.runAIChatAnalysis(context.Background(), source, analysis, layout, params)
	return CreateAnalysisResult{Repository: source, Analysis: analysis}, nil
}

func (s *Service) broadcastAnalysisStatus(repoSourceID string, analysis store.RepoAnalysisResult) {
	if s == nil || s.stream == nil || repoSourceID == "" || analysis.ID == "" {
		return
	}
	s.stream.Broadcast("repo_library.analysis.updated", analysisStatusUpdatePayload{
		RepositoryID: repoSourceID,
		AnalysisID:   analysis.ID,
		Status:       analysis.Status,
		UpdatedAt:    analysis.UpdatedAt,
	})
}

// ListRepositories 功能：返回 Repo Library 仓库摘要列表。
// 参数/返回：limit 控制返回数量；返回 RepoSourceSummary 列表。
// 失败场景：service 未配置或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) ListRepositories(ctx context.Context, limit int) ([]store.RepoSourceSummary, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	return s.store.ListRepoSources(ctx, limit)
}

// GetRepositoryDetail 功能：返回 Repo Library 仓库详情。
// 参数/返回：repoSourceID 为仓库 id；返回 RepoLibraryDetail。
// 失败场景：未命中或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) GetRepositoryDetail(ctx context.Context, repoSourceID string) (store.RepoLibraryDetail, error) {
	if s == nil || s.store == nil {
		return store.RepoLibraryDetail{}, fmt.Errorf("repo library service not configured")
	}
	detail, err := s.store.GetRepoLibraryDetail(ctx, repoSourceID)
	if err != nil {
		return store.RepoLibraryDetail{}, err
	}
	detail.Analyses = enrichAnalysesWithReportContext(detail.Analyses)
	return detail, nil
}

// ListRepositoryAnalyses 功能：返回某仓库的分析结果列表。
// 参数/返回：repoSourceID 为仓库 id；返回 RepoAnalysisResult 列表。
// 失败场景：service 未配置或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) ListRepositoryAnalyses(ctx context.Context, repoSourceID string, limit int) ([]store.RepoAnalysisResult, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	analyses, err := s.store.ListRepoAnalysisResultsBySource(ctx, repoSourceID, limit)
	if err != nil {
		return nil, err
	}
	return enrichAnalysesWithReportContext(analyses), nil
}

// ListCards 功能：按过滤条件读取知识卡片列表。
// 参数/返回：params 提供 repository/snapshot/run 过滤器；返回卡片列表。
// 失败场景：service 未配置或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) ListCards(ctx context.Context, params store.ListRepoCardsParams) ([]store.RepoKnowledgeCard, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	return s.store.ListRepoCards(ctx, params)
}

// GetCard 功能：读取单张知识卡片。
// 参数/返回：cardID 为卡片 id；返回 RepoKnowledgeCard。
// 失败场景：未命中或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) GetCard(ctx context.Context, cardID string) (store.RepoKnowledgeCard, error) {
	if s == nil || s.store == nil {
		return store.RepoKnowledgeCard{}, fmt.Errorf("repo library service not configured")
	}
	return s.store.GetRepoCard(ctx, cardID)
}

// ListCardEvidence 功能：读取单张知识卡片的证据链列表。
// 参数/返回：cardID 为卡片 id；返回 RepoKnowledgeEvidence 列表。
// 失败场景：service 未配置或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) ListCardEvidence(ctx context.Context, cardID string) ([]store.RepoKnowledgeEvidence, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	return s.store.ListRepoEvidenceByCard(ctx, cardID)
}

// GetAnalysisReport 功能：读取某个 Repo Library 分析结果的 Markdown 报告内容。
// 参数/返回：analysisID 为分析 id；返回 Markdown 文本。
// 失败场景：analysis 不存在、无 report_path 或文件读取失败时返回 error。
// 副作用：读取磁盘文件。
func (s *Service) GetAnalysisReport(ctx context.Context, analysisID string) (string, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("repo library service not configured")
	}
	analysis, err := s.store.GetRepoAnalysisResult(ctx, analysisID)
	if err != nil {
		return "", err
	}
	reportPath := strings.TrimSpace(pointerValue(analysis.ReportPath))
	if reportPath == "" {
		// Best-effort fallback for historical/partial rows.
		reportPath = strings.TrimSpace(filepath.Join(analysis.StoragePath, "report.md"))
	}
	if reportPath == "" {
		return "", os.ErrNotExist
	}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type DeleteAnalysisOptions struct {
	DeleteRepositoryIfLast bool
}

type DeleteAnalysisResult struct {
	RepositoryID      string `json:"repository_id"`
	DeletedRepository bool   `json:"deleted_repository"`
	DeletedAnalysisID string `json:"deleted_analysis_id"`
}

// DeleteAnalysis 功能：删除某次分析结果（报告/卡片/检索索引）并清理落盘目录；若为最后一份分析可选择删除仓库本身。
// 参数/返回：analysisID 为目标分析 id；opts 控制是否允许删除最后一份 analysis 后级联删除仓库；返回删除结果摘要。
// 失败场景：analysis 不存在、analysis 处于 queued/running、或删除最后一份 analysis 但未显式允许级联时返回 error。
// 副作用：写库删除、删除 searchdb chunks、删除磁盘目录。
func (s *Service) DeleteAnalysis(ctx context.Context, analysisID string, opts DeleteAnalysisOptions) (DeleteAnalysisResult, error) {
	if s == nil || s.store == nil {
		return DeleteAnalysisResult{}, fmt.Errorf("repo library service not configured")
	}
	analysisID = strings.TrimSpace(analysisID)
	if analysisID == "" {
		return DeleteAnalysisResult{}, fmt.Errorf("%w: analysis_id is required", store.ErrValidation)
	}

	outcome, err := s.store.DeleteRepoAnalysisResult(ctx, store.DeleteRepoAnalysisResultParams{
		AnalysisID:             analysisID,
		DeleteRepositoryIfLast: opts.DeleteRepositoryIfLast,
	})
	if err != nil {
		return DeleteAnalysisResult{}, err
	}

	// Best-effort: drop searchdb chunks for this analysis.
	if idx, idxErr := s.ensureSearchIndex(ctx); idxErr == nil && idx != nil && idx.sdb != nil {
		_ = idx.sdb.DeleteAnalysis(ctx, analysisID)
	}

	// Best-effort: remove on-disk artifacts.
	analysisDir := strings.TrimSpace(outcome.DeletedAnalysis.StoragePath)
	if analysisDir != "" {
		if outcome.DeletedRepository {
			// storage_path: .../repositories/<repo_key>/analyses/<analysis_id>
			repoDir := filepath.Dir(filepath.Dir(analysisDir))
			_ = os.RemoveAll(repoDir)
		} else {
			_ = os.RemoveAll(analysisDir)
		}
	}

	return DeleteAnalysisResult{
		RepositoryID:      outcome.RepositoryID,
		DeletedRepository: outcome.DeletedRepository,
		DeletedAnalysisID: analysisID,
	}, nil
}

type RepositoryViewMode string

const (
	RepositoryViewLite RepositoryViewMode = "lite"
	RepositoryViewFull RepositoryViewMode = "full"
)

type RepositoryViewLitePayload struct {
	Repositories                       []store.RepoSourceSummary     `json:"repositories"`
	Detail                             store.RepoLibraryDetail       `json:"detail"`
	SelectedAnalysisID                 string                        `json:"selected_analysis_id"`
	Cards                              []store.RepoKnowledgeCard     `json:"cards"`
	SelectedCardID                     string                        `json:"selected_card_id"`
	SelectedCard                       *store.RepoKnowledgeCard      `json:"selected_card,omitempty"`
	SelectedEvidence                   []store.RepoKnowledgeEvidence `json:"selected_evidence"`
	SelectedIntegrationSectionMarkdown string                        `json:"selected_integration_section_markdown,omitempty"`
}

type RepositoryCardHydration struct {
	Card     store.RepoKnowledgeCard       `json:"card"`
	Evidence []store.RepoKnowledgeEvidence `json:"evidence"`
}

type RepositoryViewFullPayload struct {
	AnalysisID     string                    `json:"analysis_id"`
	ReportMarkdown string                    `json:"report_markdown"`
	CardsFull      []RepositoryCardHydration `json:"cards_full"`
}

func pickLatestAnalysisID(analyses []store.RepoAnalysisResult) string {
	if len(analyses) == 0 {
		return ""
	}
	best := analyses[0]
	for _, item := range analyses[1:] {
		if item.UpdatedAt > best.UpdatedAt {
			best = item
		}
	}
	return strings.TrimSpace(best.ID)
}

func extractH2Section(reportText string, h2Title string) string {
	lines := strings.Split(reportText, "\n")
	target := "## " + strings.TrimSpace(h2Title)
	start := -1
	for idx, raw := range lines {
		if strings.TrimSpace(raw) == target {
			start = idx
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	section := strings.Join(lines[start:end], "\n")
	return strings.TrimSpace(section)
}

// GetRepositoryViewLite 功能：为仓库详情页返回“首屏可展示”的聚合数据，减少网络请求数。
// 参数/返回：repoSourceID 为仓库 id；analysisID 为可选分析 id（空则自动选择最新）；返回聚合 payload。
// 失败场景：仓库不存在或查询失败时返回 error。
// 副作用：读取 SQLite；可能读取 report.md（仅当选中卡片为集成提示时）。
func (s *Service) GetRepositoryViewLite(ctx context.Context, repoSourceID string, analysisID string) (RepositoryViewLitePayload, error) {
	if s == nil || s.store == nil {
		return RepositoryViewLitePayload{}, fmt.Errorf("repo library service not configured")
	}
	repositories, _ := s.store.ListRepoSources(ctx, 200)
	detail, err := s.GetRepositoryDetail(ctx, repoSourceID)
	if err != nil {
		return RepositoryViewLitePayload{}, err
	}
	selected := strings.TrimSpace(analysisID)
	if selected != "" {
		ok := false
		for _, item := range detail.Analyses {
			if item.ID == selected {
				ok = true
				break
			}
		}
		if !ok {
			selected = ""
		}
	}
	if selected == "" {
		selected = pickLatestAnalysisID(detail.Analyses)
	}
	cards := []store.RepoKnowledgeCard{}
	if selected != "" {
		cards, _ = s.store.ListRepoCards(ctx, store.ListRepoCardsParams{RepoSourceID: repoSourceID, AnalysisID: selected, Limit: 500})
	}
	selectedCardID := ""
	var selectedCard *store.RepoKnowledgeCard
	selectedEvidence := []store.RepoKnowledgeEvidence{}
	if len(cards) > 0 {
		selectedCardID = cards[0].ID
		card := cards[0]
		selectedCard = &card
		ev, _ := s.store.ListRepoEvidenceByCard(ctx, selectedCardID)
		selectedEvidence = ev
	}

	selectedIntegrationSectionMarkdown := ""
	if selectedCard != nil && selectedCard.CardType == "integration_note" && selected != "" {
		// Only extract the needed report section for the currently selected integration note.
		report, err := s.GetAnalysisReport(ctx, selected)
		if err == nil && strings.TrimSpace(report) != "" {
			if strings.TrimSpace(selectedCard.Title) == "项目用途与核心特点" {
				selectedIntegrationSectionMarkdown = extractH2Section(report, "第二部分：项目用途与核心特点")
			}
		}
	}

	return RepositoryViewLitePayload{
		Repositories:                       repositories,
		Detail:                             detail,
		SelectedAnalysisID:                 selected,
		Cards:                              cards,
		SelectedCardID:                     selectedCardID,
		SelectedCard:                       selectedCard,
		SelectedEvidence:                   selectedEvidence,
		SelectedIntegrationSectionMarkdown: selectedIntegrationSectionMarkdown,
	}, nil
}

// GetRepositoryViewFull 功能：返回仓库详情页的完整 hydration 数据，用于后台预取。
// 参数/返回：repoSourceID 为仓库 id；analysisID 为目标分析 id；返回 full payload。
// 失败场景：analysis 不存在、无 report 或查询失败时返回 error。
// 副作用：读取 SQLite 与 report.md。
func (s *Service) GetRepositoryViewFull(ctx context.Context, repoSourceID string, analysisID string) (RepositoryViewFullPayload, error) {
	if s == nil || s.store == nil {
		return RepositoryViewFullPayload{}, fmt.Errorf("repo library service not configured")
	}
	analysisID = strings.TrimSpace(analysisID)
	if analysisID == "" {
		return RepositoryViewFullPayload{}, fmt.Errorf("%w: analysis_id is required", store.ErrValidation)
	}
	report, _ := s.GetAnalysisReport(ctx, analysisID)
	cards, _ := s.store.ListRepoCards(ctx, store.ListRepoCardsParams{RepoSourceID: repoSourceID, AnalysisID: analysisID, Limit: 1000})
	allEvidence, _ := s.store.ListRepoEvidenceByAnalysis(ctx, analysisID, 20000)
	evidenceByCard := map[string][]store.RepoKnowledgeEvidence{}
	for _, item := range allEvidence {
		evidenceByCard[item.CardID] = append(evidenceByCard[item.CardID], item)
	}
	full := make([]RepositoryCardHydration, 0, len(cards))
	for _, card := range cards {
		full = append(full, RepositoryCardHydration{
			Card:     card,
			Evidence: evidenceByCard[card.ID],
		})
	}
	return RepositoryViewFullPayload{AnalysisID: analysisID, ReportMarkdown: report, CardsFull: full}, nil
}

// Search 功能：执行 Repo Library 语义检索，并记录搜索历史。
// 参数/返回：params 提供 query、repo 过滤器、mode 与 top_k；返回规范化后的搜索结果 JSON。
// 失败场景：校验失败、索引初始化失败或查询失败时返回 error。
// 副作用：读取/写入本地 searchdb（SQLite），并写入搜索历史。
func (s *Service) Search(ctx context.Context, params SearchParams) (map[string]any, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: query is required", store.ErrValidation)
	}

	topK := max(params.TopK, 20)
	mode := firstNonEmpty(params.Mode, "semi")

	// Resolve repository filters (repo_source_id / repo_key / analysis_id) into analysis filters.
	analysisFilters := []string{}
	for _, raw := range uniqueTrimmed(params.RepoFilters) {
		filter := strings.TrimSpace(raw)
		if filter == "" {
			continue
		}
		if strings.HasPrefix(filter, "ra_") || strings.HasPrefix(filter, "rp_") {
			analysisFilters = append(analysisFilters, filter)
			continue
		}
		repoSourceID := ""
		if strings.HasPrefix(filter, "rs_") {
			repoSourceID = filter
		} else if source, err := s.store.GetRepoSourceByRepoKey(ctx, filter); err == nil {
			repoSourceID = source.ID
		}
		if repoSourceID == "" {
			continue
		}
		analysisID := ""
		if analyses, err := s.store.ListRepoAnalysisResultsBySource(ctx, repoSourceID, 50); err == nil {
			for _, item := range analyses {
				if store.RepoAnalysisStatus(item.Status) == store.RepoAnalysisStatusSucceeded {
					analysisID = strings.TrimSpace(item.ID)
					break
				}
			}
			if analysisID == "" && len(analyses) > 0 {
				// Best-effort fallback: use latest even if not succeeded.
				analysisID = strings.TrimSpace(analyses[0].ID)
			}
		}
		if analysisID != "" {
			analysisFilters = append(analysisFilters, analysisID)
		}
	}
	analysisFilters = uniqueTrimmed(analysisFilters)

	idx, err := s.ensureSearchIndex(ctx)
	if err != nil {
		return nil, err
	}

	recall := max(topK*4, 30)
	keywordHits, err := idx.sdb.SearchKeyword(ctx, query, recall, analysisFilters, nil)
	if err != nil {
		return nil, err
	}
	titleHits, err := idx.sdb.SearchTitleMatches(ctx, query, recall, analysisFilters, nil)
	if err != nil {
		return nil, err
	}
	vectorHits := []searchdb.Hit{}
	if idx.sdb.VecEnabled() {
		if hits, err := idx.sdb.SearchVector(ctx, query, recall, analysisFilters, nil); err == nil {
			vectorHits = hits
		}
	}

	hits := idx.sdb.FuseAndTrim(topK, keywordHits, vectorHits, titleHits)
	hits = rerankSearchHits(query, hits)
	cardResults := s.collapseSearchHitsToCards(ctx, hits, topK)

	analysisCache := map[string]store.RepoAnalysisResult{}
	results := make([]map[string]any, 0, len(cardResults))
	for _, item := range cardResults {
		score := item.DisplayScore / 100.0
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}

		var analysis any = nil
		analysisID := strings.TrimSpace(item.AnalysisID)
		if analysisID != "" {
			if cached, ok := analysisCache[analysisID]; ok {
				analysis = cached
			} else if fetched, err := s.store.GetRepoAnalysisResult(ctx, analysisID); err == nil {
				analysisCache[analysisID] = fetched
				analysis = fetched
			}
		}
		results = append(results, map[string]any{
			"repository_id":    item.RepositoryID,
			"analysis_id":      item.AnalysisID,
			"card_id":          item.Card.ID,
			"score":            score,
			"title":            item.Card.Title,
			"summary":          item.Card.Summary,
			"match_sources":    item.MatchSources,
			"analysis":         analysis,
			"card":             item.Card,
			"evidence_preview": item.Evidence,
		})
	}

	response := map[string]any{
		"status":                 "ok",
		"engine":                 "go-searchdb",
		"generated_at":           time.Now().UTC().Format(time.RFC3339),
		"query":                  query,
		"mode":                   mode,
		"limit":                  topK,
		"repo_filters_requested": uniqueTrimmed(params.RepoFilters),
		"analysis_filters":       analysisFilters,
		"vec_enabled":            idx.sdb.VecEnabled(),
		"result_count":           len(results),
		"results":                results,
	}

	resultJSON := ""
	if normalized, err := json.Marshal(response); err == nil {
		resultJSON = string(normalized)
	}
	if resultJSON != "" {
		if _, err := s.store.RecordRepoSimilarityQuery(ctx, store.RecordRepoSimilarityQueryParams{
			QueryText:   query,
			RepoFilters: params.RepoFilters,
			Mode:        mode,
			TopK:        topK,
			ResultJSON:  &resultJSON,
		}); err != nil {
			logx.Warn("repo-library", "search", "记录 Repo Library 搜索历史失败", "err", err)
		}
	}
	return response, nil
}

// RebuildSearchIndex 功能：批量重建本地 go-searchdb 检索索引（按每个 repo 最新 succeeded run）。
// 参数/返回：无入参；返回重建 summary（indexed_count/errors 等）。
// 失败场景：store 不可用或读取仓库列表失败时返回 error。
// 副作用：写入 searchdb（SQLite），可能触发 embedder/sqlite-vec 的按需初始化。
func (s *Service) RebuildSearchIndex(ctx context.Context) (map[string]any, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}

	sources, err := s.store.ListRepoSources(ctx, 1000)
	if err != nil {
		return nil, err
	}

	indexed := 0
	errorsOut := make([]map[string]any, 0)

	for _, summary := range sources {
		sourceID := strings.TrimSpace(summary.ID)
		if sourceID == "" {
			continue
		}
		source, err := s.store.GetRepoSource(ctx, sourceID)
		if err != nil {
			errorsOut = append(errorsOut, map[string]any{
				"repository_id": sourceID,
				"error":         err.Error(),
			})
			continue
		}

		analyses, err := s.store.ListRepoAnalysisResultsBySource(ctx, source.ID, 50)
		if err != nil {
			errorsOut = append(errorsOut, map[string]any{
				"repository_id": source.ID,
				"repo_key":      source.RepoKey,
				"error":         err.Error(),
			})
			continue
		}
		var selected *store.RepoAnalysisResult
		for i := range analyses {
			if store.RepoAnalysisStatus(analyses[i].Status) == store.RepoAnalysisStatusSucceeded {
				selected = &analyses[i]
				break
			}
		}
		if selected == nil {
			continue
		}
		if _, err := s.refreshSearchIndexForAnalysis(ctx, source, *selected); err != nil {
			errorsOut = append(errorsOut, map[string]any{
				"repository_id": source.ID,
				"repo_key":      source.RepoKey,
				"analysis_id":   selected.ID,
				"error":         err.Error(),
			})
			continue
		}
		indexed++
	}

	return map[string]any{
		"status":        "ok",
		"engine":        "go-searchdb",
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"repo_count":    len(sources),
		"indexed_count": indexed,
		"error_count":   len(errorsOut),
		"errors":        errorsOut,
	}, nil
}

func (s *Service) prepareAnalysisLayoutForID(repoKey string, analysisID string) (analysisLayout, error) {
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return analysisLayout{}, err
	}
	analysisDir := filepath.Join(repoLibraryDir, "repositories", repoKey, "analyses", analysisID)
	return analysisLayout{
		AnalysisDir:      analysisDir,
		SourceDir:        filepath.Join(analysisDir, "source"),
		ArtifactsDir:     filepath.Join(analysisDir, "artifacts"),
		DerivedDir:       filepath.Join(analysisDir, "derived"),
		ReportPath:       filepath.Join(analysisDir, "report.md"),
		ResultPath:       filepath.Join(analysisDir, "derived", "pipeline-result.json"),
		CardsPath:        filepath.Join(analysisDir, "derived", "cards.json"),
		SearchOutputPath: filepath.Join(analysisDir, "derived", "search.json"),
	}, nil
}

type parsedRepoURL struct {
	RepoURL string
	Owner   string
	Repo    string
	RepoKey string
}

func parseGitHubRepoURL(raw string) (parsedRepoURL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsedRepoURL{}, fmt.Errorf("%w: repo_url is required", store.ErrValidation)
	}
	if strings.HasPrefix(trimmed, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(trimmed, "git@github.com:"), "/")
		if len(parts) < 2 {
			return parsedRepoURL{}, fmt.Errorf("%w: invalid GitHub repo URL", store.ErrValidation)
		}
		owner := strings.TrimSpace(parts[0])
		repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
		return parsedRepoURL{RepoURL: trimmed, Owner: owner, Repo: repo, RepoKey: normalizeRepoKey(owner, repo)}, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return parsedRepoURL{}, fmt.Errorf("%w: invalid repo_url", store.ErrValidation)
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return parsedRepoURL{}, fmt.Errorf("%w: only github.com repositories are supported", store.ErrValidation)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return parsedRepoURL{}, fmt.Errorf("%w: invalid GitHub repo URL", store.ErrValidation)
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
	if owner == "" || repo == "" {
		return parsedRepoURL{}, fmt.Errorf("%w: invalid GitHub repo URL", store.ErrValidation)
	}
	return parsedRepoURL{RepoURL: trimmed, Owner: owner, Repo: repo, RepoKey: normalizeRepoKey(owner, repo)}, nil
}

func normalizeRepoKey(owner string, repo string) string {
	value := strings.ToLower(strings.TrimSpace(owner + "-" + repo))
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", "_", "-", ".", "-")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	if value == "" {
		return "unknown-repo"
	}
	return value
}

func discoverProjectRoot() (string, error) {
	starts := []string{}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		starts = append(starts, filepath.Dir(file))
	}
	for _, start := range starts {
		current := start
		for {
			if current == "" || current == string(filepath.Separator) || current == "." {
				break
			}
			if fileExists(filepath.Join(current, "PROJECT_STRUCTURE.md")) || fileExists(filepath.Join(current, ".git")) {
				return current, nil
			}
			next := filepath.Dir(current)
			if next == current {
				break
			}
			current = next
		}
	}
	return "", fmt.Errorf("resolve project root: PROJECT_STRUCTURE.md not found")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type blockingCommandResult struct {
	StdOut string
	StdErr string
}

func runBlockingCommand(ctx context.Context, cwd string, command string, args []string) (blockingCommandResult, error) {
	ctx = firstContext(ctx)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = cwd
	stdout, err := cmd.Output()
	if err == nil {
		return blockingCommandResult{StdOut: string(stdout)}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return blockingCommandResult{StdOut: string(stdout), StdErr: string(exitErr.Stderr)}, fmt.Errorf("command failed: %s %s: %s", command, strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return blockingCommandResult{}, err
}

func executionErrorMessage(final execution.Execution) string {
	if final.Signal != "" {
		return fmt.Sprintf("signal=%s exit_code=%d", final.Signal, final.ExitCode)
	}
	return fmt.Sprintf("exit_code=%d", final.ExitCode)
}

func extractSummary(resultMap map[string]any) *string {
	value := extractString(resultMap, "summary", "analysis.summary")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func extractReportPath(resultMap map[string]any, fallback string) *string {
	value := extractString(resultMap, "report_path", "snapshot.report_path")
	if strings.TrimSpace(value) == "" {
		value = fallback
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func extractString(root map[string]any, paths ...string) string {
	for _, path := range paths {
		segments := strings.Split(path, ".")
		current := any(root)
		ok := true
		for _, segment := range segments {
			obj, isMap := current.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			value, exists := obj[segment]
			if !exists {
				ok = false
				break
			}
			current = value
		}
		if !ok {
			continue
		}
		if text, ok := current.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func loadCardsFile(path string) ([]store.RepoKnowledgeCardInput, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rawBody any
	if err := json.Unmarshal(payload, &rawBody); err != nil {
		return nil, fmt.Errorf("parse cards JSON: %w", err)
	}
	rawCards := []map[string]any{}
	evidenceByCard := map[string][]map[string]any{}
	switch typed := rawBody.(type) {
	case []any:
		for _, item := range typed {
			if cardMap, ok := item.(map[string]any); ok {
				rawCards = append(rawCards, cardMap)
			}
		}
	case map[string]any:
		if cardsAny, ok := typed["cards"].([]any); ok {
			for _, item := range cardsAny {
				if cardMap, ok := item.(map[string]any); ok {
					rawCards = append(rawCards, cardMap)
				}
			}
		}
		if evidenceAny, ok := typed["evidence"].([]any); ok {
			for _, rawEvidence := range evidenceAny {
				evidenceMap, ok := rawEvidence.(map[string]any)
				if !ok {
					continue
				}
				cardID := extractString(evidenceMap, "card_id")
				if strings.TrimSpace(cardID) == "" {
					continue
				}
				evidenceByCard[cardID] = append(evidenceByCard[cardID], evidenceMap)
			}
		}
	default:
		return nil, fmt.Errorf("parse cards JSON: unsupported payload shape")
	}
	cards := make([]store.RepoKnowledgeCardInput, 0, len(rawCards))
	for idx, item := range rawCards {
		cardID := extractString(item, "card_id")
		card := store.RepoKnowledgeCardInput{
			Title:        extractString(item, "title"),
			CardType:     firstNonEmpty(extractString(item, "card_type"), extractString(item, "type")),
			Summary:      extractString(item, "summary"),
			Mechanism:    stringPtrIfNotEmpty(firstNonEmpty(extractString(item, "mechanism"), extractString(item, "content"))),
			Confidence:   stringPtrIfNotEmpty(extractString(item, "confidence")),
			SectionTitle: stringPtrIfNotEmpty(extractString(item, "section_title")),
			SortIndex:    idx + 1,
		}
		if tags, ok := item["tags"].([]any); ok {
			for _, rawTag := range tags {
				if tag, ok := rawTag.(string); ok && strings.TrimSpace(tag) != "" {
					card.Tags = append(card.Tags, strings.TrimSpace(tag))
				}
			}
		}
		for evidenceIdx, evidenceMap := range evidenceByCard[cardID] {
			line := firstInt64(int64FromAny(evidenceMap["line"]), int64FromAny(evidenceMap["source_line"]))
			card.Evidence = append(card.Evidence, store.RepoKnowledgeEvidenceInput{
				Path:      firstNonEmpty(extractString(evidenceMap, "path"), extractString(evidenceMap, "source_path")),
				Line:      line,
				Snippet:   stringPtrIfNotEmpty(firstNonEmpty(extractString(evidenceMap, "snippet"), extractString(evidenceMap, "excerpt"))),
				Dimension: stringPtrIfNotEmpty(firstNonEmpty(extractString(evidenceMap, "dimension"), extractString(evidenceMap, "label"))),
				SortIndex: evidenceIdx + 1,
			})
		}
		if evidence, ok := item["evidence"].([]any); ok {
			for evidenceIdx, rawEvidence := range evidence {
				evidenceMap, ok := rawEvidence.(map[string]any)
				if !ok {
					continue
				}
				line := firstInt64(int64FromAny(evidenceMap["line"]), int64FromAny(evidenceMap["source_line"]))
				card.Evidence = append(card.Evidence, store.RepoKnowledgeEvidenceInput{
					Path:      firstNonEmpty(extractString(evidenceMap, "path"), extractString(evidenceMap, "source_path")),
					Line:      line,
					Snippet:   stringPtrIfNotEmpty(firstNonEmpty(extractString(evidenceMap, "snippet"), extractString(evidenceMap, "excerpt"))),
					Dimension: stringPtrIfNotEmpty(firstNonEmpty(extractString(evidenceMap, "dimension"), extractString(evidenceMap, "label"))),
					SortIndex: evidenceIdx + 1,
				})
			}
		}
		if strings.TrimSpace(card.Title) == "" || strings.TrimSpace(card.CardType) == "" || strings.TrimSpace(card.Summary) == "" {
			continue
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func int64FromAny(value any) *int64 {
	switch typed := value.(type) {
	case float64:
		v := int64(typed)
		return &v
	case int64:
		v := typed
		return &v
	case int:
		v := int64(typed)
		return &v
	default:
		return nil
	}
}

func firstInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func uniqueTrimmed(values []string) []string {
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

func firstNonEmpty(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func pointer(value string) *string {
	copy := value
	return &copy
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := strings.TrimSpace(value)
	return &copy
}

func max(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
