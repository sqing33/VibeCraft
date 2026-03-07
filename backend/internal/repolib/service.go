package repolib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/id"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/store"
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
	Repository store.RepoSource      `json:"repository"`
	Snapshot   store.RepoSnapshot    `json:"snapshot"`
	Run        store.RepoAnalysisRun `json:"analysis_run"`
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
}

type pipelineLayout struct {
	SnapshotDir      string
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
func NewService(stateStore *store.Store, execMgr *execution.Manager, chatMgr *chat.Manager, experts *expert.Registry) (*Service, error) {
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
	return &Service{store: stateStore, executions: execMgr, chat: chatMgr, experts: experts, projectRoot: projectRoot, pythonBin: "python3"}, nil
}

// CreateAnalysis 功能：创建并启动一条 Repo Library 分析运行。
// 参数/返回：params 提供仓库、ref、features 与 CLI tool/model 选择；返回 repository/snapshot/run 聚合结果。
// 失败场景：校验失败、依赖缺失或初始化失败时返回 error。
// 副作用：写入 SQLite、准备 Repo Library snapshot，并在后台启动真实 AI Chat 分析流程。
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
	snapshotID := id.New("rp_")
	layout, err := s.prepareSnapshotLayoutForID(source.RepoKey, snapshotID)
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	for _, dir := range []string{layout.SnapshotDir, layout.SourceDir, layout.ArtifactsDir, layout.DerivedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return CreateAnalysisResult{}, err
		}
	}
	snapshot, err := s.store.CreateRepoSnapshot(ctx, store.CreateRepoSnapshotParams{
		SnapshotID:    snapshotID,
		RepoSourceID: source.ID,
		RequestedRef: ref,
		StoragePath:  layout.SnapshotDir,
	})
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	snapshot, err = s.store.UpdateRepoSnapshot(ctx, store.UpdateRepoSnapshotParams{SnapshotID: snapshot.ID, StoragePath: &layout.SnapshotDir, ReportPath: &layout.ReportPath})
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	runtimeKind := pointer("ai_chat")
	run, err := s.store.CreateRepoAnalysisRun(ctx, store.CreateRepoAnalysisRunParams{
		RepoSourceID:   source.ID,
		RepoSnapshotID: snapshot.ID,
		Language:       firstNonEmpty(params.Language, "zh"),
		Depth:          firstNonEmpty(params.Depth, "standard"),
		AgentMode:      firstNonEmpty(params.AgentMode, "single"),
		Features:       features,
		RuntimeKind:    runtimeKind,
		CLIToolID:      stringPtrIfNotEmpty(params.CLIToolID),
		ModelID:        stringPtrIfNotEmpty(params.ModelID),
	})
	if err != nil {
		return CreateAnalysisResult{}, err
	}
	go s.runAIChatAnalysis(context.Background(), source, snapshot, run, layout, params)
	return CreateAnalysisResult{Repository: source, Snapshot: snapshot, Run: run}, nil
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
	return s.store.GetRepoLibraryDetail(ctx, repoSourceID)
}

// ListRepositorySnapshots 功能：返回某仓库的快照列表。
// 参数/返回：repoSourceID 为仓库 id；返回 RepoSnapshot 列表。
// 失败场景：service 未配置或查询失败时返回 error。
// 副作用：读取 SQLite。
func (s *Service) ListRepositorySnapshots(ctx context.Context, repoSourceID string, limit int) ([]store.RepoSnapshot, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	return s.store.ListRepoSnapshotsBySource(ctx, repoSourceID, limit)
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

// GetSnapshotReport 功能：读取某个 Repo Library 快照的 Markdown 报告内容。
// 参数/返回：snapshotID 为快照 id；返回 Markdown 文本。
// 失败场景：快照不存在、无 report_path 或文件读取失败时返回 error。
// 副作用：读取磁盘文件。
func (s *Service) GetSnapshotReport(ctx context.Context, snapshotID string) (string, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("repo library service not configured")
	}
	snapshot, err := s.store.GetRepoSnapshot(ctx, snapshotID)
	if err != nil {
		return "", err
	}
	if snapshot.ReportPath == nil || strings.TrimSpace(*snapshot.ReportPath) == "" {
		return "", os.ErrNotExist
	}
	body, err := os.ReadFile(strings.TrimSpace(*snapshot.ReportPath))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// Search 功能：执行 Repo Library 语义检索，并记录搜索历史。
// 参数/返回：params 提供 query、repo 过滤器、mode 与 top_k；返回规范化后的搜索结果 JSON。
// 失败场景：校验失败、Python 引擎执行失败或结果解析失败时返回 error。
// 副作用：调用 Python CLI、可能刷新向量索引并写入搜索历史。
func (s *Service) Search(ctx context.Context, params SearchParams) (map[string]any, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, fmt.Errorf("%w: query is required", store.ErrValidation)
	}
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return nil, err
	}
	outputPath := filepath.Join(repoLibraryDir, "queries", fmt.Sprintf("%d-search.json", time.Now().UnixNano()))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	args := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"search",
		"--storage-root", repoLibraryDir,
		"--query", query,
		"--mode", firstNonEmpty(params.Mode, "semi"),
		"--limit", fmt.Sprintf("%d", max(params.TopK, 20)),
		"--output", outputPath,
	}
	for _, repo := range uniqueTrimmed(params.RepoFilters) {
		args = append(args, "--repo", repo)
	}
	result, err := runBlockingCommand(ctx, s.projectRoot, s.pythonBin, args)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(result.StdErr)) > 0 {
		logx.Info("repo-library", "search", "Repo Library search stderr", "stderr", strings.TrimSpace(result.StdErr))
	}
	payload, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("parse search result: %w", err)
	}
	s.enrichSearchResults(ctx, response)
	resultJSON := string(payload)
	if normalized, err := json.Marshal(response); err == nil {
		resultJSON = string(normalized)
	}
	if _, err := s.store.RecordRepoSimilarityQuery(ctx, store.RecordRepoSimilarityQueryParams{QueryText: query, RepoFilters: params.RepoFilters, Mode: firstNonEmpty(params.Mode, "semi"), TopK: max(params.TopK, 20), ResultJSON: &resultJSON}); err != nil {
		logx.Warn("repo-library", "search", "记录 Repo Library 搜索历史失败", "err", err)
	}
	return response, nil
}

func (s *Service) enrichSearchResults(ctx context.Context, response map[string]any) {
	rawResults, ok := response["results"].([]any)
	if !ok {
		return
	}
	for _, raw := range rawResults {
		result, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		repository, ok := result["repository"].(map[string]any)
		if !ok {
			continue
		}
		repoKey := extractString(repository, "repository_id", "repo_key")
		if strings.TrimSpace(repoKey) == "" {
			continue
		}
		source, err := s.store.GetRepoSourceByRepoKey(ctx, repoKey)
		if err != nil {
			continue
		}
		result["repository_id"] = source.ID
		repository["repository_id"] = source.ID
		repository["owner"] = source.Owner
		repository["repo"] = source.Repo
		repository["full_name"] = fmt.Sprintf("%s/%s", source.Owner, source.Repo)
		repository["default_branch"] = source.DefaultBranch
		repository["repo_url"] = source.RepoURL
		if snapshot, ok := result["snapshot"].(map[string]any); ok {
			snapshot["repository_id"] = source.ID
		}
	}
}

func (s *Service) completeAnalysisRun(runID, snapshotID, repoSourceID string, layout pipelineLayout, final execution.Execution) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	status := string(store.RepoAnalysisStatusFailed)
	if final.Status == execution.StatusSucceeded {
		status = string(store.RepoAnalysisStatusSucceeded)
	}
	resultJSONText := ""
	var summary *string
	var reportPath *string
	var errorMessage *string
	if final.Status != execution.StatusSucceeded {
		msg := executionErrorMessage(final)
		errorMessage = &msg
	}
	payload, err := os.ReadFile(layout.ResultPath)
	if err == nil && len(payload) > 0 {
		resultJSONText = string(payload)
		resultMap := map[string]any{}
		if unmarshalErr := json.Unmarshal(payload, &resultMap); unmarshalErr == nil {
			summary = extractSummary(resultMap)
			reportPath = extractReportPath(resultMap, layout.ReportPath)
			resolvedRef := extractString(resultMap, "resolved_ref", "snapshot.resolved_ref")
			commitSHA := extractString(resultMap, "commit_sha", "snapshot.commit_sha")
			subagentResultsPath := extractString(resultMap, "subagent_results_path", "snapshot.subagent_results_path")
			if _, updateErr := s.store.UpdateRepoSnapshot(ctx, store.UpdateRepoSnapshotParams{SnapshotID: snapshotID, ResolvedRef: stringPtrIfNotEmpty(resolvedRef), CommitSHA: stringPtrIfNotEmpty(commitSHA), ReportPath: reportPath, SubagentResultsPath: stringPtrIfNotEmpty(subagentResultsPath)}); updateErr != nil {
				logx.Warn("repo-library", "finalize", "更新 Repo Library snapshot 失败", "snapshot_id", snapshotID, "err", updateErr)
			}
			cards, parseErr := loadCardsFile(layout.CardsPath)
			if parseErr != nil && !errors.Is(parseErr, os.ErrNotExist) {
				logx.Warn("repo-library", "finalize", "读取 Repo Library cards 失败", "run_id", runID, "err", parseErr)
			} else if len(cards) > 0 {
				if replaceErr := s.store.ReplaceRepoKnowledge(ctx, store.ReplaceRepoKnowledgeParams{RepoSourceID: repoSourceID, RepoSnapshotID: snapshotID, AnalysisRunID: runID, Cards: cards}); replaceErr != nil {
					logx.Warn("repo-library", "finalize", "写入 Repo Library cards 失败", "run_id", runID, "err", replaceErr)
				}
			}
		}
	}
	if reportPath == nil {
		reportPath = stringPtrIfNotEmpty(layout.ReportPath)
	}
	var resultJSON *string
	if strings.TrimSpace(resultJSONText) != "" {
		resultJSON = &resultJSONText
	}
	if _, err := s.store.FinalizeRepoAnalysisRun(ctx, store.FinalizeRepoAnalysisRunParams{RunID: runID, Status: status, Summary: summary, ErrorMessage: errorMessage, ResultJSON: resultJSON, ReportPath: reportPath}); err != nil {
		logx.Warn("repo-library", "finalize", "收敛 Repo Library run 失败", "run_id", runID, "err", err)
	}
}

func (s *Service) buildPipelineArgs(params CreateAnalysisParams, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, layout pipelineLayout) []string {
	repoLibraryDir, _ := paths.RepoLibraryDir()
	args := []string{
		filepath.Join(s.projectRoot, "services", "repo-analyzer", "app", "cli.py"),
		"pipeline",
		"--repo-url", strings.TrimSpace(params.RepoURL),
		"--ref", snapshot.RequestedRef,
		"--storage-root", repoLibraryDir,
		"--depth", run.Depth,
		"--language", run.Language,
		"--run-id", run.ID,
		"--snapshot-dir", layout.SnapshotDir,
		"--output", layout.ResultPath,
	}
	for _, feature := range run.Features {
		args = append(args, "--feature", feature)
	}
	return args
}

func (s *Service) prepareSnapshotLayoutForID(repoKey string, snapshotID string) (pipelineLayout, error) {
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return pipelineLayout{}, err
	}
	snapshotDir := filepath.Join(repoLibraryDir, "repositories", repoKey, "snapshots", snapshotID)
	return pipelineLayout{
		SnapshotDir:      snapshotDir,
		SourceDir:        filepath.Join(snapshotDir, "source"),
		ArtifactsDir:     filepath.Join(snapshotDir, "artifacts"),
		DerivedDir:       filepath.Join(snapshotDir, "derived"),
		ReportPath:       filepath.Join(snapshotDir, "report.md"),
		ResultPath:       filepath.Join(snapshotDir, "derived", "pipeline-result.json"),
		CardsPath:        filepath.Join(snapshotDir, "derived", "cards.json"),
		SearchOutputPath: filepath.Join(snapshotDir, "derived", "search.json"),
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
