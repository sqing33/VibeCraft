package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/store"
)

type approveWorkflowResponse struct {
	Workflow store.Workflow `json:"workflow"`
	Nodes    []store.Node   `json:"nodes"`
}

// approveWorkflowHandler 功能：在 manual 模式下批准所有 runnable nodes（pending_approval -> queued）。
// 参数/返回：依赖 store；返回 approveWorkflowResponse。
// 失败场景：workflow 不存在返回 404；非 manual 或参数非法返回 400；写库失败返回 500。
// 副作用：写入 SQLite nodes/workflows/events，并广播 workflow/node 更新事件。
func approveWorkflowHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		workflowID := c.Param("id")
		wf, nodes, err := deps.Store.ApproveRunnableNodes(c.Request.Context(), workflowID)
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
		for _, n := range nodes {
			broadcastNodeUpdated(deps.Hub, n)
		}

		c.JSON(http.StatusOK, approveWorkflowResponse{Workflow: wf, Nodes: nodes})
	}
}
