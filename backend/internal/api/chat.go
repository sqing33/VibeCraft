package api

import (
	"context"
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
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

type createChatSessionRequest struct {
	Title         string   `json:"title"`
	ExpertID      string   `json:"expert_id"`
	CLIToolID     string   `json:"cli_tool_id"`
	ModelID       string   `json:"model_id"`
	MCPServerIDs  []string `json:"mcp_server_ids,omitempty"`
	WorkspacePath string   `json:"workspace_path"`
}

type llmModelRuntime struct {
	ModelID  string
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
}

func isChatCapableResolved(resolved expert.Resolved) bool {
	if resolved.Provider == "process" {
		return false
	}
	if resolved.Spec.SDK != nil {
		return true
	}
	return !resolved.HelperOnly
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
		if strings.TrimSpace(req.CLIToolID) != "" {
			expertID = strings.TrimSpace(req.CLIToolID)
		}
		if expertID == "" {
			expertID = "codex"
		}
		workspace := strings.TrimSpace(req.WorkspacePath)
		if workspace == "" {
			workspace = "."
		}

		resolved, err := deps.Experts.ResolveWithOptions(expertID, "", workspace, expert.ResolveOptions{CLIToolID: strings.TrimSpace(req.CLIToolID), ModelID: strings.TrimSpace(req.ModelID)})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resolved, status, err := applyLLMModelRuntime(resolved, strings.TrimSpace(req.ModelID))
		if err != nil {
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		if !isChatCapableResolved(resolved) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expert is not chat-capable"})
			return
		}

		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		selectedMCPs := normalizeSelectedMCPServerIDs(req.MCPServerIDs)
		if selectedMCPs == nil {
			selectedMCPs = config.DefaultEnabledMCPServerIDs(cfg, firstNonEmptyTrimmed(strings.TrimSpace(req.CLIToolID), resolved.ToolID, expertID))
		}

		sess, err := deps.Store.CreateChatSession(c.Request.Context(), store.CreateChatSessionParams{
			Title:         req.Title,
			ExpertID:      firstNonEmptyTrimmed(resolved.ExpertID, expertID),
			CLIToolID:     pointerOrNilString(strings.TrimSpace(req.CLIToolID)),
			ModelID:       pointerOrNilString(strings.TrimSpace(req.ModelID)),
			MCPServerIDs:  selectedMCPs,
			Provider:      firstNonEmptyTrimmed(resolved.ProtocolFamily, resolved.Provider),
			Model:         resolved.Model,
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

// listChatTurnsHandler 功能：返回一个 session 下最近若干轮已持久化的 chat timeline 快照。
// 参数/返回：通过 `:id` 指定 session，`limit` 控制返回轮数；返回 `[]store.ChatTurn`。
// 失败场景：session 不存在、limit 非法或查询失败时返回 4xx/5xx。
// 副作用：读取 SQLite 中的 turn 与 item 快照。
func listChatTurnsHandler(deps Deps) gin.HandlerFunc {
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
		turns, err := deps.Store.ListChatTurns(c.Request.Context(), sessionID, limit)
		if err != nil {
			if errors.Is(err, store.ErrValidation) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, turns)
	}
}

type patchChatSessionRequest struct {
	Title        *string   `json:"title"`
	Status       *string   `json:"status"`
	MCPServerIDs *[]string `json:"mcp_server_ids,omitempty"`
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
			Title:        req.Title,
			Status:       req.Status,
			MCPServerIDs: normalizeSelectedMCPServerIDPointer(req.MCPServerIDs),
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
	Input        string   `json:"input"`
	ExpertID     string   `json:"expert_id"`
	CLIToolID    string   `json:"cli_tool_id"`
	ModelID      string   `json:"model_id"`
	MCPServerIDs []string `json:"mcp_server_ids,omitempty"`
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
		req.CLIToolID = c.Request.FormValue("cli_tool_id")
		req.ModelID = c.Request.FormValue("model_id")
		if raw := c.Request.FormValue("mcp_server_ids"); strings.TrimSpace(raw) != "" {
			_ = json.Unmarshal([]byte(raw), &req.MCPServerIDs)
		}
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
		if selectedMCPs := normalizeSelectedMCPServerIDs(req.MCPServerIDs); selectedMCPs != nil {
			sess, err = deps.Store.PatchChatSession(c.Request.Context(), sessionID, store.PatchChatSessionParams{MCPServerIDs: &selectedMCPs})
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
		}
		expertID := strings.TrimSpace(req.ExpertID)
		if strings.TrimSpace(req.CLIToolID) != "" {
			expertID = strings.TrimSpace(req.CLIToolID)
		}
		if expertID == "" {
			expertID = sess.ExpertID
		}
		resolved, err := deps.Experts.ResolveWithOptions(expertID, providerInput, sess.WorkspacePath, expert.ResolveOptions{CLIToolID: strings.TrimSpace(req.CLIToolID), ModelID: strings.TrimSpace(req.ModelID)})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resolved, status, err = applyLLMModelRuntime(resolved, strings.TrimSpace(req.ModelID))
		if err != nil {
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		if !isChatCapableResolved(resolved) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session expert is not chat-capable"})
			return
		}
		modelInput := providerInput
		if resolved.Spec.SDK != nil {
			modelInput = resolved.Spec.SDK.Prompt
		}
		turnCtx := context.WithoutCancel(c.Request.Context())
		result, err := deps.Chat.RunTurn(turnCtx, chat.TurnParams{
			Session:             sess,
			ExpertID:            firstNonEmptyTrimmed(resolved.ExpertID, expertID),
			CLIToolID:           pointerOrNilString(firstNonEmptyTrimmed(strings.TrimSpace(req.CLIToolID), resolved.ToolID)),
			ModelID:             pointerOrNilString(strings.TrimSpace(req.ModelID)),
			UserInput:           userText,
			ModelInput:          modelInput,
			Attachments:         uploads,
			Spec:                resolved.Spec,
			Provider:            firstNonEmptyTrimmed(resolved.ProtocolFamily, resolved.Provider),
			Model:               resolved.Model,
			ThinkingTranslation: buildThinkingTranslationSpec(firstNonEmptyTrimmed(strings.TrimSpace(req.ModelID), resolved.PrimaryModelID, resolved.Model)),
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

func applyLLMModelRuntime(resolved expert.Resolved, requestedModelID string) (expert.Resolved, int, error) {
	if resolved.Spec.SDK == nil {
		return resolved, 0, nil
	}
	modelID := firstNonEmptyTrimmed(strings.TrimSpace(requestedModelID), strings.TrimSpace(resolved.PrimaryModelID))
	if modelID == "" {
		return resolved, 0, nil
	}
	runtime, found, err := resolveLLMModelRuntime(modelID)
	if err != nil {
		return resolved, http.StatusInternalServerError, err
	}
	if !found {
		if strings.TrimSpace(requestedModelID) == "" {
			return resolved, 0, nil
		}
		return resolved, http.StatusBadRequest, fmt.Errorf("model_id %q does not exist", modelID)
	}
	provider := strings.ToLower(strings.TrimSpace(resolved.Provider))
	if provider != "" && provider != strings.ToLower(strings.TrimSpace(runtime.Provider)) {
		return resolved, http.StatusBadRequest, fmt.Errorf("model_id %q is incompatible with sdk provider %q", modelID, resolved.Provider)
	}
	sdkCopy := *resolved.Spec.SDK
	sdkCopy.Provider = strings.TrimSpace(runtime.Provider)
	sdkCopy.Model = strings.TrimSpace(runtime.Model)
	sdkCopy.LLMModelID = strings.TrimSpace(runtime.ModelID)
	sdkCopy.BaseURL = strings.TrimSpace(runtime.BaseURL)
	env := cloneStringMap(resolved.Spec.Env)
	delete(env, "OPENAI_API_KEY")
	delete(env, "ANTHROPIC_API_KEY")
	delete(env, "OPENAI_BASE_URL")
	delete(env, "ANTHROPIC_BASE_URL")
	switch strings.ToLower(strings.TrimSpace(runtime.Provider)) {
	case "openai":
		if strings.TrimSpace(runtime.APIKey) != "" {
			env["OPENAI_API_KEY"] = strings.TrimSpace(runtime.APIKey)
		}
		if strings.TrimSpace(runtime.BaseURL) != "" {
			env["OPENAI_BASE_URL"] = strings.TrimSpace(runtime.BaseURL)
		}
	case "anthropic":
		if strings.TrimSpace(runtime.APIKey) != "" {
			env["ANTHROPIC_API_KEY"] = strings.TrimSpace(runtime.APIKey)
		}
		if strings.TrimSpace(runtime.BaseURL) != "" {
			env["ANTHROPIC_BASE_URL"] = strings.TrimSpace(runtime.BaseURL)
		}
	}
	specCopy := resolved.Spec
	specCopy.SDK = &sdkCopy
	specCopy.Env = env
	resolved.Spec = specCopy
	resolved.Provider = strings.TrimSpace(runtime.Provider)
	resolved.ProtocolFamily = strings.TrimSpace(runtime.Provider)
	resolved.Model = strings.TrimSpace(runtime.Model)
	resolved.PrimaryModelID = strings.TrimSpace(runtime.ModelID)
	return resolved, 0, nil
}

func resolveLLMModelRuntime(modelID string) (*llmModelRuntime, bool, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil, false, nil
	}
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return nil, false, fmt.Errorf("load persisted llm settings: %w", err)
	}
	modelCfg, sourceCfg, _, ok := config.FindLLMModelByID(cfg.LLM, modelID)
	if !ok {
		return nil, false, nil
	}
	return &llmModelRuntime{
		ModelID:  strings.TrimSpace(modelCfg.ID),
		Provider: strings.ToLower(strings.TrimSpace(modelCfg.Provider)),
		Model:    strings.TrimSpace(modelCfg.Model),
		BaseURL:  strings.TrimSpace(sourceCfg.BaseURL),
		APIKey:   strings.TrimSpace(sourceCfg.APIKey),
	}, true, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
		var sdk runner.SDKSpec
		if resolved.Spec.SDK != nil {
			sdk = *resolved.Spec.SDK
		}

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

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func pointerOrNilString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func normalizeSelectedMCPServerIDs(values []string) []string {
	if values == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeSelectedMCPServerIDPointer(values *[]string) *[]string {
	if values == nil {
		return nil
	}
	normalized := normalizeSelectedMCPServerIDs(*values)
	return &normalized
}
