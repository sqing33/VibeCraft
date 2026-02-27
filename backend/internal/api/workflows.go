package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

type createWorkflowRequest struct {
	Title         string `json:"title"`
	WorkspacePath string `json:"workspace_path"`
	Mode          string `json:"mode"`
}

// createWorkflowHandler 功能：创建 workflow（MVP：仅写入元数据，不启动执行）。
// 参数/返回：依赖 store；返回 gin.HandlerFunc。
// 失败场景：store 未配置、请求 JSON 非法或参数校验失败时返回 4xx/5xx。
// 副作用：写入 SQLite workflows/events，并广播 `workflow.updated` WS 事件（如 hub 已配置）。
func createWorkflowHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		var req createWorkflowRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		wf, err := deps.Store.CreateWorkflow(c.Request.Context(), store.CreateWorkflowParams{
			Title:         req.Title,
			WorkspacePath: req.WorkspacePath,
			Mode:          req.Mode,
		})
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		broadcastWorkflowUpdated(deps.Hub, wf)
		c.JSON(http.StatusOK, wf)
	}
}

// listWorkflowsHandler 功能：读取 workflows 列表（按 updated_at 倒序）。
// 参数/返回：依赖 store；返回 gin.HandlerFunc。
// 失败场景：store 未配置或查询失败时返回 500。
// 副作用：读取 SQLite。
func listWorkflowsHandler(deps Deps) gin.HandlerFunc {
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

		wfs, err := deps.Store.ListWorkflows(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, wfs)
	}
}

// getWorkflowHandler 功能：读取 workflow 详情。
// 参数/返回：依赖 store；返回 gin.HandlerFunc。
// 失败场景：workflow 不存在返回 404；查询失败返回 500。
// 副作用：读取 SQLite。
func getWorkflowHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		workflowID := c.Param("id")
		wf, err := deps.Store.GetWorkflow(c.Request.Context(), workflowID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, wf)
	}
}

type patchWorkflowRequest struct {
	Title         *string `json:"title"`
	WorkspacePath *string `json:"workspace_path"`
	Mode          *string `json:"mode"`
}

// patchWorkflowHandler 功能：更新 workflow 的可编辑字段（title/workspace/mode）。
// 参数/返回：依赖 store；返回 gin.HandlerFunc。
// 失败场景：workflow 不存在返回 404；参数非法返回 400；更新失败返回 500。
// 副作用：写入 SQLite workflows/events，并广播 `workflow.updated` WS 事件（如 hub 已配置）。
func patchWorkflowHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		var req patchWorkflowRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
			return
		} else if err := json.Unmarshal(b, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		workflowID := c.Param("id")
		wf, err := deps.Store.UpdateWorkflow(c.Request.Context(), workflowID, store.UpdateWorkflowParams{
			Title:         req.Title,
			WorkspacePath: req.WorkspacePath,
			Mode:          req.Mode,
		})
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		broadcastWorkflowUpdated(deps.Hub, wf)
		c.JSON(http.StatusOK, wf)
	}
}

func broadcastWorkflowUpdated(hub *ws.Hub, wf store.Workflow) {
	if hub == nil {
		return
	}
	env := ws.Envelope{
		Type:       "workflow.updated",
		Ts:         time.Now().UnixMilli(),
		WorkflowID: wf.ID,
		Payload:    wf,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	hub.Broadcast(b)
}
