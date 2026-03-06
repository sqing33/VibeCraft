package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vibe-tree/backend/internal/paths"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/config"
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

const maxChatTurnBodyBytes int64 = 24 << 20

type postChatTurnRequest struct {
	Input    string `json:"input"`
	ExpertID string `json:"expert_id"`
}

func parsePostChatTurnRequest(c *gin.Context) (postChatTurnRequest, []chat.UploadedAttachment, int, error) {
	var req postChatTurnRequest
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxChatTurnBodyBytes)
		if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
			status := http.StatusBadRequest
			if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
				status = http.StatusRequestEntityTooLarge
			}
			return req, nil, status, err
		}
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		req.Input = c.Request.FormValue("input")
		req.ExpertID = c.Request.FormValue("expert_id")
		uploads := make([]chat.UploadedAttachment, 0)
		if c.Request.MultipartForm != nil {
			for _, header := range c.Request.MultipartForm.File["files"] {
				file, err := header.Open()
				if err != nil {
					return req, nil, http.StatusBadRequest, err
				}
				data, err := io.ReadAll(file)
				_ = file.Close()
				if err != nil {
					return req, nil, http.StatusBadRequest, err
				}
				uploads = append(uploads, chat.UploadedAttachment{
					FileName: header.Filename,
					MIMEType: header.Header.Get("Content-Type"),
					Bytes:    data,
				})
			}
		}
		return req, uploads, 0, nil
	}
	if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
		if err := json.Unmarshal(b, &req); err != nil {
			return req, nil, http.StatusBadRequest, err
		}
	}
	return req, nil, 0, nil
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
		req, uploads, status, err := parsePostChatTurnRequest(c)
		if err != nil {
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		userText, providerInput := chat.PrepareTurnInputs(req.Input, len(uploads))
		if strings.TrimSpace(providerInput) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "input or files is required"})
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
		expertID := strings.TrimSpace(req.ExpertID)
		if expertID == "" {
			expertID = sess.ExpertID
		}
		resolved, err := deps.Experts.Resolve(expertID, providerInput, sess.WorkspacePath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if resolved.Spec.SDK == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session expert is not sdk provider"})
			return
		}
		sdk := *resolved.Spec.SDK
		result, err := deps.Chat.RunTurn(c.Request.Context(), chat.TurnParams{
			Session:             sess,
			ExpertID:            expertID,
			UserInput:           userText,
			ModelInput:          sdk.Prompt,
			Attachments:         uploads,
			SDK:                 sdk,
			Env:                 resolved.Spec.Env,
			Fallbacks:           resolved.Spec.SDKFallbacks,
			ThinkingTranslation: buildThinkingTranslationSpec(resolved.PrimaryModelID),
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

func buildThinkingTranslationSpec(primaryModelID string) *chat.ThinkingTranslationSpec {
	if strings.TrimSpace(primaryModelID) == "" {
		return nil
	}
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return nil
	}
	runtime, err := config.ResolveThinkingTranslation(cfg.Basic, cfg.LLM, primaryModelID)
	if err != nil || runtime == nil {
		return nil
	}
	env := map[string]string{}
	switch strings.ToLower(strings.TrimSpace(runtime.Provider)) {
	case "openai":
		if strings.TrimSpace(runtime.APIKey) != "" {
			env["OPENAI_API_KEY"] = strings.TrimSpace(runtime.APIKey)
		}
	case "anthropic":
		if strings.TrimSpace(runtime.APIKey) != "" {
			env["ANTHROPIC_API_KEY"] = strings.TrimSpace(runtime.APIKey)
		}
	default:
		return nil
	}
	return &chat.ThinkingTranslationSpec{
		Provider: strings.TrimSpace(runtime.Provider),
		Model:    strings.TrimSpace(runtime.Model),
		BaseURL:  strings.TrimSpace(runtime.BaseURL),
		Env:      env,
	}
}

func getChatAttachmentContentHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		sessionID := c.Param("id")
		attachmentID := c.Param("attachmentID")
		att, err := deps.Store.GetChatAttachment(c.Request.Context(), sessionID, attachmentID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "attachment not found"})
				return
			}
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		root, err := paths.ChatAttachmentsDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		fullPath := filepath.Join(root, filepath.FromSlash(att.StorageRelPath))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.JSON(http.StatusNotFound, gin.H{"error": "attachment content not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		contentType := strings.TrimSpace(att.MIMEType)
		if contentType == "" {
			contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(att.FileName)))
		}
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		c.Header("Content-Type", contentType)
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", att.FileName))
		c.Data(http.StatusOK, contentType, data)
	}
}

func compactChatSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}
		if deps.Chat == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "chat manager not configured"})
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
		resolved, err := deps.Experts.Resolve(sess.ExpertID, "", sess.WorkspacePath)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if resolved.Spec.SDK == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session expert is not sdk provider"})
			return
		}
		sdk := *resolved.Spec.SDK

		sess, rec, err := deps.Chat.CompactSession(c.Request.Context(), sessionID, sdk, resolved.Spec.Env)
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
