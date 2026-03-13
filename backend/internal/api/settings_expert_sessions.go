package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/expert"
	"vibecraft/backend/internal/expertbuilder"
	"vibecraft/backend/internal/skillcatalog"
	"vibecraft/backend/internal/store"
)

type expertBuilderSessionItem struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	TargetExpertID   *string `json:"target_expert_id,omitempty"`
	BuilderModelID   string  `json:"builder_model_id"`
	Status           string  `json:"status"`
	LatestSnapshotID *string `json:"latest_snapshot_id,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

type expertBuilderMessageItem struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	Role        string `json:"role"`
	ContentText string `json:"content_text"`
	CreatedAt   int64  `json:"created_at"`
}

type expertBuilderSnapshotItem struct {
	ID               string             `json:"id"`
	SessionID        string             `json:"session_id"`
	Version          int64              `json:"version"`
	AssistantMessage string             `json:"assistant_message"`
	Draft            expertSettingsItem `json:"draft"`
	RawJSON          *string            `json:"raw_json,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
	CreatedAt        int64              `json:"created_at"`
}

type expertBuilderSessionDetailResponse struct {
	Session   expertBuilderSessionItem    `json:"session"`
	Messages  []expertBuilderMessageItem  `json:"messages"`
	Snapshots []expertBuilderSnapshotItem `json:"snapshots"`
}

type createExpertBuilderSessionRequest struct {
	Title          string `json:"title"`
	TargetExpertID string `json:"target_expert_id"`
	BuilderModelID string `json:"builder_model_id"`
}

type postExpertBuilderMessageRequest struct {
	Content string `json:"content"`
}

type publishExpertBuilderSnapshotRequest struct {
	SnapshotID string `json:"snapshot_id"`
	ExpertID   string `json:"expert_id"`
}

func listExpertBuilderSessionsHandler(deps Deps) gin.HandlerFunc {
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
		targetExpertID := strings.TrimSpace(c.Query("target_expert_id"))
		sessions, err := deps.Store.ListExpertBuilderSessions(c.Request.Context(), limit, targetExpertID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]expertBuilderSessionItem, 0, len(sessions))
		for _, sess := range sessions {
			out = append(out, toExpertBuilderSessionItem(sess))
		}
		c.JSON(http.StatusOK, gin.H{"sessions": out})
	}
}

func createExpertBuilderSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		var req createExpertBuilderSessionRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		sess, err := deps.Store.CreateExpertBuilderSession(c.Request.Context(), store.CreateExpertBuilderSessionParams{Title: req.Title, TargetExpertID: req.TargetExpertID, BuilderModelID: req.BuilderModelID})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"session": toExpertBuilderSessionItem(sess)})
	}
}

func getExpertBuilderSessionHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		detail, err := loadExpertBuilderSessionDetail(c.Request.Context(), deps, c.Param("id"))
		if err != nil {
			status := http.StatusInternalServerError
			if err == store.ErrValidation || err == context.Canceled {
				status = http.StatusBadRequest
			}
			if err == store.ErrConflict || strings.Contains(strings.ToLower(err.Error()), "not exist") || strings.Contains(strings.ToLower(err.Error()), "not found") {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

func postExpertBuilderMessageHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "store not configured"})
			return
		}
		var req postExpertBuilderMessageRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		content := strings.TrimSpace(req.Content)
		if content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
			return
		}
		sess, err := deps.Store.GetExpertBuilderSession(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if _, err := deps.Store.AppendExpertBuilderMessage(c.Request.Context(), sess.ID, "user", content); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		messages, err := deps.Store.ListExpertBuilderMessages(c.Request.Context(), sess.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		builderMessages := make([]expertbuilder.Message, 0, len(messages))
		for _, msg := range messages {
			builderMessages = append(builderMessages, expertbuilder.Message{Role: msg.Role, Content: msg.ContentText})
		}
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := config.RebuildExperts(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		resolved, err := expert.NewRegistry(cfg).Resolve(sess.BuilderModelID, "", ".")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		result, err := expertbuilder.NewService().Generate(c.Request.Context(), resolved.Spec, cfg.LLM, skillcatalog.Discover(), builderMessages)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		assistantMsg, err := deps.Store.AppendExpertBuilderMessage(c.Request.Context(), sess.ID, "assistant", result.AssistantMessage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		draftJSON, err := json.Marshal(result.Draft)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		snapshot, err := deps.Store.CreateExpertBuilderSnapshot(c.Request.Context(), store.CreateExpertBuilderSnapshotParams{SessionID: sess.ID, AssistantMessage: assistantMsg.ContentText, DraftJSON: string(draftJSON), RawJSON: pointerOrNil(result.RawJSON), Warnings: result.Warnings})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if sess.TargetExpertID == nil && strings.TrimSpace(result.Draft.ID) != "" {
			_, _ = deps.Store.PatchExpertBuilderSession(c.Request.Context(), sess.ID, store.PatchExpertBuilderSessionParams{TargetExpertID: pointerOrNil(result.Draft.ID), LatestSnapshotID: &snapshot.ID})
		}
		detail, err := loadExpertBuilderSessionDetail(c.Request.Context(), deps, sess.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, detail)
	}
}

func publishExpertBuilderSnapshotHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Store == nil || deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dependencies not configured"})
			return
		}
		var req publishExpertBuilderSnapshotRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		sess, err := deps.Store.GetExpertBuilderSession(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		snapshotID := strings.TrimSpace(req.SnapshotID)
		if snapshotID == "" && sess.LatestSnapshotID != nil {
			snapshotID = strings.TrimSpace(*sess.LatestSnapshotID)
		}
		if snapshotID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "snapshot_id is required"})
			return
		}
		snapshot, err := deps.Store.GetExpertBuilderSnapshot(c.Request.Context(), snapshotID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		var draft expertbuilder.Draft
		if err := json.Unmarshal([]byte(snapshot.DraftJSON), &draft); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		expertID := strings.TrimSpace(req.ExpertID)
		if expertID == "" && sess.TargetExpertID != nil {
			expertID = strings.TrimSpace(*sess.TargetExpertID)
		}
		item := expertSettingsFromDraft(draft, sess.BuilderModelID)
		if expertID != "" {
			item.ID = expertID
		}
		item.BuilderSessionID = sess.ID
		item.BuilderSnapshotID = snapshot.ID
		item.BuilderExpertID = sess.BuilderModelID
		cfg, cfgPath, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		cfg, err = saveExpertProfiles(cfg, []putExpertSettingsItem{toPutExpertSettingsItem(item)})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		deps.Experts.Reload(cfg)
		_, _ = deps.Store.PatchExpertBuilderSession(c.Request.Context(), sess.ID, store.PatchExpertBuilderSessionParams{TargetExpertID: &item.ID, Status: pointerOrNil("published"), LatestSnapshotID: &snapshot.ID})
		detail, err := loadExpertBuilderSessionDetail(c.Request.Context(), deps, sess.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"session": detail.Session, "published_expert": item, "snapshots": detail.Snapshots, "messages": detail.Messages})
	}
}

func loadExpertBuilderSessionDetail(ctx context.Context, deps Deps, sessionID string) (expertBuilderSessionDetailResponse, error) {
	sess, err := deps.Store.GetExpertBuilderSession(ctx, sessionID)
	if err != nil {
		return expertBuilderSessionDetailResponse{}, err
	}
	messages, err := deps.Store.ListExpertBuilderMessages(ctx, sessionID)
	if err != nil {
		return expertBuilderSessionDetailResponse{}, err
	}
	snapshots, err := deps.Store.ListExpertBuilderSnapshots(ctx, sessionID)
	if err != nil {
		return expertBuilderSessionDetailResponse{}, err
	}
	outMsgs := make([]expertBuilderMessageItem, 0, len(messages))
	for _, msg := range messages {
		outMsgs = append(outMsgs, expertBuilderMessageItem{ID: msg.ID, SessionID: msg.SessionID, Role: msg.Role, ContentText: msg.ContentText, CreatedAt: msg.CreatedAt})
	}
	outSnaps := make([]expertBuilderSnapshotItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var draft expertbuilder.Draft
		if err := json.Unmarshal([]byte(snapshot.DraftJSON), &draft); err != nil {
			return expertBuilderSessionDetailResponse{}, err
		}
		item := expertSettingsFromDraft(draft, sess.BuilderModelID)
		item.BuilderSessionID = sess.ID
		item.BuilderSnapshotID = snapshot.ID
		outSnaps = append(outSnaps, expertBuilderSnapshotItem{ID: snapshot.ID, SessionID: snapshot.SessionID, Version: snapshot.Version, AssistantMessage: snapshot.AssistantMessage, Draft: item, RawJSON: snapshot.RawJSON, Warnings: snapshot.Warnings, CreatedAt: snapshot.CreatedAt})
	}
	return expertBuilderSessionDetailResponse{Session: toExpertBuilderSessionItem(sess), Messages: outMsgs, Snapshots: outSnaps}, nil
}

func toExpertBuilderSessionItem(sess store.ExpertBuilderSession) expertBuilderSessionItem {
	return expertBuilderSessionItem{ID: sess.ID, Title: sess.Title, TargetExpertID: sess.TargetExpertID, BuilderModelID: sess.BuilderModelID, Status: sess.Status, LatestSnapshotID: sess.LatestSnapshotID, CreatedAt: sess.CreatedAt, UpdatedAt: sess.UpdatedAt}
}

func saveExpertProfiles(cfg config.Config, items []putExpertSettingsItem) (config.Config, error) {
	preserved := make([]config.ExpertConfig, 0, len(cfg.Experts))
	for _, expert := range cfg.Experts {
		if strings.EqualFold(strings.TrimSpace(expert.ManagedSource), config.ManagedSourceExpertProfile) {
			continue
		}
		preserved = append(preserved, expert)
	}
	now := time.Now().UnixMilli()
	custom := make([]config.ExpertConfig, 0, len(items))
	for _, item := range items {
		updatedAt := item.UpdatedAt
		if updatedAt <= 0 {
			updatedAt = now
		}
		custom = append(custom, config.ExpertConfig{ID: strings.TrimSpace(item.ID), Label: strings.TrimSpace(item.Label), Description: strings.TrimSpace(item.Description), Category: strings.TrimSpace(item.Category), Avatar: strings.TrimSpace(item.Avatar), ManagedSource: config.ManagedSourceExpertProfile, PrimaryModelID: strings.TrimSpace(item.PrimaryModelID), SecondaryModelID: strings.TrimSpace(item.SecondaryModelID), FallbackOn: append([]string(nil), item.FallbackOn...), EnabledSkills: append([]string(nil), item.EnabledSkills...), SystemPrompt: strings.TrimSpace(item.SystemPrompt), PromptTemplate: strings.TrimSpace(item.PromptTemplate), OutputFormat: strings.TrimSpace(item.OutputFormat), MaxOutputTokens: item.MaxOutputTokens, Temperature: item.Temperature, TimeoutMs: item.TimeoutMs, BuilderExpertID: strings.TrimSpace(item.BuilderExpertID), BuilderSessionID: strings.TrimSpace(item.BuilderSessionID), BuilderSnapshotID: strings.TrimSpace(item.BuilderSnapshotID), GeneratedBy: strings.TrimSpace(item.GeneratedBy), GeneratedAt: item.GeneratedAt, UpdatedAt: updatedAt, Disabled: !item.Enabled})
	}
	cfg.Experts = append(preserved, custom...)
	if err := config.RebuildExperts(&cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func toPutExpertSettingsItem(item expertSettingsItem) putExpertSettingsItem {
	return putExpertSettingsItem{ID: item.ID, Label: item.Label, Description: item.Description, Category: item.Category, Avatar: item.Avatar, PrimaryModelID: item.PrimaryModelID, SecondaryModelID: item.SecondaryModelID, FallbackOn: append([]string(nil), item.FallbackOn...), EnabledSkills: append([]string(nil), item.EnabledSkills...), SystemPrompt: item.SystemPrompt, PromptTemplate: item.PromptTemplate, OutputFormat: item.OutputFormat, MaxOutputTokens: item.MaxOutputTokens, Temperature: item.Temperature, TimeoutMs: item.TimeoutMs, BuilderExpertID: item.BuilderExpertID, BuilderSessionID: item.BuilderSessionID, BuilderSnapshotID: item.BuilderSnapshotID, GeneratedBy: item.GeneratedBy, GeneratedAt: item.GeneratedAt, UpdatedAt: item.UpdatedAt, Enabled: item.Enabled}
}

func pointerOrNil(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}
