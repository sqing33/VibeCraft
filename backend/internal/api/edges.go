package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// listWorkflowEdgesHandler 功能：列出 workflow 下的 edges（用于 DAG 可视化/调试）。
// 参数/返回：依赖 store；返回 []store.Edge。
// 失败场景：workflow 不存在返回 404；查询失败返回 500。
// 副作用：读取 SQLite。
func listWorkflowEdgesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}

		workflowID := c.Param("id")
		if _, err := deps.Store.GetWorkflow(c.Request.Context(), workflowID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		edges, err := deps.Store.ListEdges(c.Request.Context(), workflowID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, edges)
	}
}
