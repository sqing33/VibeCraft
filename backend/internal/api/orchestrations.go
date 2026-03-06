package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/store"
)

type createOrchestrationRequest struct {
	Title         string `json:"title"`
	Goal          string `json:"goal"`
	WorkspacePath string `json:"workspace_path"`
}

// createOrchestrationHandler 功能：直接根据自然语言 goal 创建一条 orchestration。
// 参数/返回：依赖 orchestration manager；返回 orchestration 详情。
// 失败场景：依赖缺失、参数非法或创建失败时返回 4xx/5xx。
// 副作用：写入 SQLite orchestration 相关表，并触发后续调度。
func createOrchestrationHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Orchestration == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestration manager not configured"})
			return
		}
		var req createOrchestrationRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		detail, err := deps.Orchestration.Create(c.Request.Context(), req.Title, req.Goal, req.WorkspacePath)
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

// listOrchestrationsHandler 功能：读取 orchestration 列表。
// 参数/返回：依赖 store；返回 orchestration 列表。
// 失败场景：store 未配置或查询失败时返回 500。
// 副作用：读取 SQLite。
func listOrchestrationsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		limit := 50
		if raw := c.Query("limit"); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 {
				limit = v
			}
		}
		items, err := deps.Store.ListOrchestrations(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

// getOrchestrationHandler 功能：读取 orchestration 详情。
// 参数/返回：依赖 store；返回 orchestration detail。
// 失败场景：未命中返回 404，查询失败返回 500。
// 副作用：读取 SQLite。
func getOrchestrationHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		detail, err := deps.Store.GetOrchestrationDetail(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "orchestration not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

// cancelOrchestrationHandler 功能：取消一条 orchestration。
// 参数/返回：依赖 orchestration manager；返回最新 orchestration。
// 失败场景：未命中返回 404，状态冲突返回 409，其他失败返回 500。
// 副作用：写入 SQLite，并尝试取消运行中的 executions。
func cancelOrchestrationHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Orchestration == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestration manager not configured"})
			return
		}
		orch, err := deps.Orchestration.Cancel(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "orchestration not found"})
				return
			}
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, orch)
	}
}

// continueOrchestrationHandler 功能：在 waiting_continue 状态下进入下一轮 orchestration。
// 参数/返回：依赖 orchestration manager；返回完整 orchestration detail。
// 失败场景：未命中返回 404，状态冲突返回 409，其他失败返回 500。
// 副作用：写入 SQLite，并创建下一轮 round/agent runs。
func continueOrchestrationHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Orchestration == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestration manager not configured"})
			return
		}
		detail, err := deps.Orchestration.Continue(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "orchestration not found"})
				return
			}
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

// retryAgentRunHandler 功能：重试失败的 agent run。
// 参数/返回：依赖 orchestration manager；返回 orchestration/round/agent run。
// 失败场景：未命中返回 404，状态冲突返回 409，其他失败返回 500。
// 副作用：写入 SQLite，并等待后续 Tick 重新调度执行。
func retryAgentRunHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Orchestration == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "orchestration manager not configured"})
			return
		}
		orch, round, run, err := deps.Orchestration.RetryAgentRun(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "agent run not found"})
				return
			}
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"orchestration": orch, "round": round, "agent_run": run})
	}
}
