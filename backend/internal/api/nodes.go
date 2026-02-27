package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/store"
)

type patchNodeRequest struct {
	Prompt   *string `json:"prompt"`
	ExpertID *string `json:"expert_id"`
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
