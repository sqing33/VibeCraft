package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/store"
)

type patchNodeRequest struct {
	Prompt   *string `json:"prompt"`
	ExpertID *string `json:"expert_id"`
}

type retryNodeResponse struct {
	Workflow store.Workflow `json:"workflow"`
	Nodes    []store.Node   `json:"nodes"`
}

// patchNodeHandler 功能：更新 node（MVP：支持 prompt/expert_id），用于 manual approval 编辑与修正。
// 参数/返回：依赖 store；返回更新后的 store.Node。
// 失败场景：node 不存在返回 404；参数非法返回 400；状态冲突返回 409；写库失败返回 500。
// 副作用：写入 SQLite nodes/workflows/events，并广播 workflow/node 更新事件。
func patchNodeHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}

		nodeID := c.Param("id")
		var req patchNodeRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		if req.ExpertID != nil {
			id := strings.TrimSpace(*req.ExpertID)
			if id == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "expert_id is required"})
				return
			}
			if _, ok := deps.Experts.KnownIDs()[id]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown expert_id"})
				return
			}
			*req.ExpertID = id
		}

		n, err := deps.Store.UpdateNode(c.Request.Context(), nodeID, store.UpdateNodeParams{
			Prompt:   req.Prompt,
			ExpertID: req.ExpertID,
		})
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		broadcastNodeUpdated(deps.Hub, n)
		if wf, err := deps.Store.GetWorkflow(c.Request.Context(), n.WorkflowID); err == nil {
			broadcastWorkflowUpdated(deps.Hub, wf)
		}

		c.JSON(http.StatusOK, n)
	}
}

// retryNodeHandler 功能：重试一个失败/取消/超时的 worker node（重置为 queued，并解开 fail-fast 产生的 skipped）。
// 参数/返回：依赖 store；返回 retryNodeResponse。
// 失败场景：node/workflow 不存在返回 404；状态不允许返回 409；参数非法返回 400；写库失败返回 500。
// 副作用：写入 SQLite nodes/workflows/events，并广播 workflow/node 更新事件；后续由 scheduler 启动新的 execution。
func retryNodeHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		nodeID := c.Param("id")
		wf, nodes, err := deps.Store.RetryNode(c.Request.Context(), nodeID)
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		broadcastWorkflowUpdated(deps.Hub, wf)
		for _, n := range nodes {
			broadcastNodeUpdated(deps.Hub, n)
		}

		c.JSON(http.StatusOK, retryNodeResponse{Workflow: wf, Nodes: nodes})
	}
}

type cancelNodeResponse struct {
	OK          bool   `json:"ok"`
	ExecutionID string `json:"execution_id,omitempty"`
}

// cancelNodeHandler 功能：取消一个 running worker node 的当前执行（best-effort）。
// 参数/返回：依赖 store 与 executions manager；返回 cancelNodeResponse。
// 失败场景：node 不存在返回 404；node 非 running 返回 409；取消失败返回 500。
// 副作用：向子进程发送取消信号；最终状态通过 execution.exited 收敛并落库。
func cancelNodeHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Executions == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "execution manager not configured"})
			return
		}

		nodeID := c.Param("id")
		n, err := deps.Store.GetNode(c.Request.Context(), nodeID)
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if n.NodeType == "master" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "master node cancel is not supported in MVP"})
			return
		}
		if n.Status != "running" {
			c.JSON(http.StatusConflict, gin.H{"error": "node is not running"})
			return
		}
		if n.LastExecution == nil || *n.LastExecution == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "node has no running execution"})
			return
		}

		execID := *n.LastExecution
		if err := deps.Executions.Cancel(execID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusConflict, gin.H{"error": "execution not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, cancelNodeResponse{OK: true, ExecutionID: execID})
	}
}
