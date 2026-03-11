package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type importCodexHistoryRequest struct {
	ThreadIDs []string `json:"thread_ids"`
}

func listCodexHistoryThreadsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.CodexHistory == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "codex history service not configured"})
			return
		}
		limit := 500
		if raw := c.Query("limit"); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil && value > 0 {
				limit = value
			}
		}
		threads, err := deps.CodexHistory.ListThreads(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, threads)
	}
}

func importCodexHistoryHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.CodexHistory == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "codex history service not configured"})
			return
		}
		var req importCodexHistoryRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		result, err := deps.CodexHistory.ImportThreads(c.Request.Context(), req.ThreadIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
