package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/store"
)

type cancelWorkflowResponse struct {
	Workflow      store.Workflow `json:"workflow"`
	CanceledNodes []store.Node   `json:"canceled_nodes"`
	CancelQueued  int            `json:"cancel_queued"`
}

// cancelWorkflowHandler 功能：取消一个 workflow（标记为 canceled，并尝试取消所有 running execution；同时将未开始节点标记为 canceled，避免被调度器继续推进）。
// 参数/返回：依赖 store 与 executions manager；返回 cancelWorkflowResponse。
// 失败场景：workflow 不存在返回 404；workflow 已终态返回 409；取消或写库失败返回 5xx。
// 副作用：写入 SQLite workflows/nodes/events，并向子进程发送取消信号（异步收敛到 execution.exited）。
func cancelWorkflowHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Executions == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "execution manager not configured"})
			return
		}

		workflowID := c.Param("id")
		res, err := deps.Store.CancelWorkflow(c.Request.Context(), workflowID)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
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

		for _, execID := range res.RunningExecutionIDs {
			if err := deps.Executions.Cancel(execID); err != nil && !errors.Is(err, os.ErrNotExist) {
				// best-effort; execution 可能已退出或不在内存态
				continue
			}
		}

		broadcastWorkflowUpdated(deps.Hub, res.Workflow)
		for _, n := range res.CanceledNodes {
			broadcastNodeUpdated(deps.Hub, n)
		}

		c.JSON(http.StatusOK, cancelWorkflowResponse{
			Workflow:      res.Workflow,
			CanceledNodes: res.CanceledNodes,
			CancelQueued:  len(res.RunningExecutionIDs),
		})
	}
}
