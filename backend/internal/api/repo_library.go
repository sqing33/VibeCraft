package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/repolib"
	"vibe-tree/backend/internal/store"
)

type createRepoAnalysisRequest struct {
	RepoURL   string   `json:"repo_url"`
	Ref       string   `json:"ref"`
	Features  []string `json:"features"`
	Depth     string   `json:"depth"`
	Language  string   `json:"language"`
	AgentMode string   `json:"agent_mode"`
}

type repoLibrarySearchRequest struct {
	Query       string   `json:"query"`
	RepoFilters []string `json:"repo_filters"`
	Mode        string   `json:"mode"`
	TopK        int      `json:"top_k"`
}

// createRepoAnalysisHandler 功能：创建一条 Repo Library 分析运行。
// 参数/返回：依赖 repo library service；成功返回 repository/snapshot/run 聚合结果。
// 失败场景：依赖缺失、请求体非法或创建失败时返回 4xx/5xx。
// 副作用：写入 SQLite，并启动后台 Python analyzer 进程。
func createRepoAnalysisHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		var req createRepoAnalysisRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		result, err := deps.RepoLibrary.CreateAnalysis(c.Request.Context(), repolibCreateParams(req))
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// listRepoRepositoriesHandler 功能：读取 Repo Library 仓库列表摘要。
// 参数/返回：依赖 repo library service；返回仓库摘要数组。
// 失败场景：依赖缺失或查询失败时返回 500。
// 副作用：读取 SQLite。
func listRepoRepositoriesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		limit := queryLimit(c, 100)
		items, err := deps.RepoLibrary.ListRepositories(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

// getRepoRepositoryHandler 功能：读取单个 Repo Library 仓库详情。
// 参数/返回：依赖 repo library service；返回 repository/snapshots/runs/cards 聚合结果。
// 失败场景：未命中返回 404，其他失败返回 500。
// 副作用：读取 SQLite。
func getRepoRepositoryHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		detail, err := deps.RepoLibrary.GetRepositoryDetail(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

// listRepoSnapshotsHandler 功能：读取某仓库的快照列表。
// 参数/返回：依赖 repo library service；返回 RepoSnapshot[]。
// 失败场景：依赖缺失或查询失败时返回 500。
// 副作用：读取 SQLite。
func listRepoSnapshotsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		items, err := deps.RepoLibrary.ListRepositorySnapshots(c.Request.Context(), c.Param("id"), queryLimit(c, 100))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

// listRepoCardsHandler 功能：按过滤条件读取 Repo Library 卡片列表。
// 参数/返回：依赖 repo library service；返回 RepoKnowledgeCard[]。
// 失败场景：依赖缺失或查询失败时返回 500。
// 副作用：读取 SQLite。
func listRepoCardsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		items, err := deps.RepoLibrary.ListCards(c.Request.Context(), store.ListRepoCardsParams{
			RepoSourceID:   strings.TrimSpace(c.Query("repository_id")),
			RepoSnapshotID: strings.TrimSpace(c.Query("snapshot_id")),
			AnalysisRunID:  strings.TrimSpace(c.Query("analysis_run_id")),
			Limit:          queryLimit(c, 500),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

// getRepoCardHandler 功能：读取单张 Repo Library 卡片。
// 参数/返回：依赖 repo library service；返回 RepoKnowledgeCard。
// 失败场景：未命中返回 404，其他失败返回 500。
// 副作用：读取 SQLite。
func getRepoCardHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		card, err := deps.RepoLibrary.GetCard(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, card)
	}
}

// listRepoCardEvidenceHandler 功能：读取某张卡片的证据链列表。
// 参数/返回：依赖 repo library service；返回 RepoKnowledgeEvidence[]。
// 失败场景：依赖缺失或查询失败时返回 500。
// 副作用：读取 SQLite。
func listRepoCardEvidenceHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		items, err := deps.RepoLibrary.ListCardEvidence(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

// getRepoSnapshotReportHandler 功能：读取某个快照的 Markdown 报告内容。
// 参数/返回：依赖 repo library service；返回 report markdown 文本。
// 失败场景：未命中返回 404，其他失败返回 500。
// 副作用：读取磁盘文件。
func getRepoSnapshotReportHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		report, err := deps.RepoLibrary.GetSnapshotReport(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"snapshot_id": c.Param("id"), "report_markdown": report})
	}
}

// searchRepoLibraryHandler 功能：执行 Repo Library 语义搜索。
// 参数/返回：依赖 repo library service；返回搜索结果 JSON。
// 失败场景：请求体非法、校验失败或执行失败时返回 4xx/5xx。
// 副作用：调用 Python 搜索引擎并记录搜索历史。
func searchRepoLibraryHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.RepoLibrary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "repo library service not configured"})
			return
		}
		var req repoLibrarySearchRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		result, err := deps.RepoLibrary.Search(c.Request.Context(), repolibSearchParams(req))
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func queryLimit(c *gin.Context, fallback int) int {
	limit := fallback
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			limit = value
		}
	}
	return limit
}

func repolibCreateParams(req createRepoAnalysisRequest) repolib.CreateAnalysisParams {
	return repolib.CreateAnalysisParams{
		RepoURL:   strings.TrimSpace(req.RepoURL),
		Ref:       strings.TrimSpace(req.Ref),
		Features:  req.Features,
		Depth:     strings.TrimSpace(req.Depth),
		Language:  strings.TrimSpace(req.Language),
		AgentMode: strings.TrimSpace(req.AgentMode),
	}
}

func repolibSearchParams(req repoLibrarySearchRequest) repolib.SearchParams {
	return repolib.SearchParams{
		Query:       strings.TrimSpace(req.Query),
		RepoFilters: req.RepoFilters,
		Mode:        strings.TrimSpace(req.Mode),
		TopK:        req.TopK,
	}
}
