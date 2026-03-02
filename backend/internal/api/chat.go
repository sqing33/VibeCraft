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

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/store"
)

type createChatSessionRequest struct {
	Title         string `json:"title"`
	ExpertID      string `json:"expert_id"`
	WorkspacePath string `json:"workspace_path"`
}

func createChatSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}

		var req createChatSessionRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		expertID := strings.TrimSpace(req.ExpertID)
		if expertID == "" {
			expertID = "codex"
		}
		workspace := strings.TrimSpace(req.WorkspacePath)
		if workspace == "" {
			workspace = "."
		}

		resolved, err := deps.Experts.Resolve(expertID, "", workspace)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if resolved.Spec.SDK == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expert must be sdk provider"})
			return
		}

		sess, err := deps.Store.CreateChatSession(c.Request.Context(), store.CreateChatSessionParams{
			Title:         req.Title,
			ExpertID:      expertID,
			Provider:      resolved.Spec.SDK.Provider,
			Model:         resolved.Spec.SDK.Model,
			WorkspacePath: workspace,
		})
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sess)
	}
}

func listChatSessionsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		limit := 100
		if raw := c.Query("limit"); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 {
				limit = v
			}
		}
		sessions, err := deps.Store.ListChatSessions(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sessions)
	}
}

func listChatMessagesHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		limit := 200
		if raw := c.Query("limit"); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil && v > 0 {
				limit = v
			}
		}
		sessionID := c.Param("id")
		if _, err := deps.Store.GetChatSession(c.Request.Context(), sessionID); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		msgs, err := deps.Store.ListChatMessages(c.Request.Context(), sessionID, limit)
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, msgs)
	}
}

type patchChatSessionRequest struct {
	Title  *string `json:"title"`
	Status *string `json:"status"`
}

func patchChatSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		var req patchChatSessionRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
			return
		} else if err := json.Unmarshal(b, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sess, err := deps.Store.PatchChatSession(c.Request.Context(), c.Param("id"), store.PatchChatSessionParams{
			Title:  req.Title,
			Status: req.Status,
		})
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sess)
	}
}

type postChatTurnRequest struct {
	Input string `json:"input"`
}

func postChatTurnHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Chat == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "chat manager not configured"})
			return
		}
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}
		var req postChatTurnRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		if strings.TrimSpace(req.Input) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "input is required"})
			return
		}
		sessionID := c.Param("id")
		sess, err := deps.Store.GetChatSession(c.Request.Context(), sessionID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resolved, err := deps.Experts.Resolve(sess.ExpertID, req.Input, sess.WorkspacePath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if resolved.Spec.SDK == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session expert is not sdk provider"})
			return
		}
		sdk := *resolved.Spec.SDK
		sdk.Provider = sess.Provider
		sdk.Model = sess.Model
		result, err := deps.Chat.RunTurn(c.Request.Context(), chat.TurnParams{
			Session:    sess,
			UserInput:  req.Input,
			ModelInput: sdk.Prompt,
			SDK:        sdk,
			Env:        resolved.Spec.Env,
		})
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func compactChatSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Chat == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "chat manager not configured"})
			return
		}
		sess, rec, err := deps.Chat.CompactSession(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"session": sess, "compaction": rec})
	}
}

type forkChatSessionRequest struct {
	Title string `json:"title"`
}

func forkChatSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		var req forkChatSessionRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		sess, err := deps.Store.ForkChatSession(c.Request.Context(), c.Param("id"), req.Title)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
				return
			}
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sess)
	}
}
