package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/expert"
	"vibecraft/backend/internal/expertbuilder"
	"vibecraft/backend/internal/skillcatalog"
)

type expertSettingsResponse struct {
	Experts        []expertSettingsItem `json:"experts"`
	Skills         []skillCatalogItem   `json:"skills"`
	BuilderExperts []publicExpertRef    `json:"builder_experts"`
}

type publicExpertRef struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Description string `json:"description,omitempty"`
}

type skillCatalogItem struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
}

type expertSettingsItem struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Description       string   `json:"description,omitempty"`
	Category          string   `json:"category,omitempty"`
	Avatar            string   `json:"avatar,omitempty"`
	ManagedSource     string   `json:"managed_source,omitempty"`
	PrimaryModelID    string   `json:"primary_model_id,omitempty"`
	SecondaryModelID  string   `json:"secondary_model_id,omitempty"`
	FallbackOn        []string `json:"fallback_on,omitempty"`
	EnabledSkills     []string `json:"enabled_skills,omitempty"`
	Provider          string   `json:"provider,omitempty"`
	Model             string   `json:"model,omitempty"`
	SystemPrompt      string   `json:"system_prompt,omitempty"`
	PromptTemplate    string   `json:"prompt_template,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	MaxOutputTokens   int      `json:"max_output_tokens,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	TimeoutMs         int      `json:"timeout_ms,omitempty"`
	BuilderExpertID   string   `json:"builder_expert_id,omitempty"`
	BuilderSessionID  string   `json:"builder_session_id,omitempty"`
	BuilderSnapshotID string   `json:"builder_snapshot_id,omitempty"`
	GeneratedBy       string   `json:"generated_by,omitempty"`
	GeneratedAt       int64    `json:"generated_at,omitempty"`
	UpdatedAt         int64    `json:"updated_at,omitempty"`
	Enabled           bool     `json:"enabled"`
	Editable          bool     `json:"editable"`
}

type putExpertSettingsRequest struct {
	Experts []putExpertSettingsItem `json:"experts"`
}

type putExpertSettingsItem struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Description       string   `json:"description,omitempty"`
	Category          string   `json:"category,omitempty"`
	Avatar            string   `json:"avatar,omitempty"`
	PrimaryModelID    string   `json:"primary_model_id,omitempty"`
	SecondaryModelID  string   `json:"secondary_model_id,omitempty"`
	FallbackOn        []string `json:"fallback_on,omitempty"`
	EnabledSkills     []string `json:"enabled_skills,omitempty"`
	SystemPrompt      string   `json:"system_prompt,omitempty"`
	PromptTemplate    string   `json:"prompt_template,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	MaxOutputTokens   int      `json:"max_output_tokens,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	TimeoutMs         int      `json:"timeout_ms,omitempty"`
	BuilderExpertID   string   `json:"builder_expert_id,omitempty"`
	BuilderSessionID  string   `json:"builder_session_id,omitempty"`
	BuilderSnapshotID string   `json:"builder_snapshot_id,omitempty"`
	GeneratedBy       string   `json:"generated_by,omitempty"`
	GeneratedAt       int64    `json:"generated_at,omitempty"`
	UpdatedAt         int64    `json:"updated_at,omitempty"`
	Enabled           bool     `json:"enabled"`
}

type generateExpertSettingsRequest struct {
	BuilderExpertID string                  `json:"builder_expert_id"`
	BuilderModelID  string                  `json:"builder_model_id"`
	Messages        []expertbuilder.Message `json:"messages"`
}

type generateExpertSettingsResponse struct {
	AssistantMessage string             `json:"assistant_message"`
	Draft            expertSettingsItem `json:"draft"`
	Warnings         []string           `json:"warnings,omitempty"`
	RawJSON          string             `json:"raw_json,omitempty"`
}

func getExpertSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp, err := buildExpertSettingsResponse(cfg, deps)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func putExpertSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}
		var req putExpertSettingsRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		cfg, cfgPath, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		cfg, err = saveExpertProfiles(cfg, req.Experts)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		deps.Experts.Reload(cfg)
		resp, err := buildExpertSettingsResponse(cfg, deps)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func generateExpertSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Experts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "expert registry not configured"})
			return
		}
		var req generateExpertSettingsRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		builderID := strings.TrimSpace(req.BuilderExpertID)
		builderModelID := strings.TrimSpace(req.BuilderModelID)
		if len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "messages are required"})
			return
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
		localRegistry := expert.NewRegistry(cfg)
		if builderID == "" && builderModelID == "" {
			builderModelID = firstBuilderModelID(cfg)
		}
		resolveID := builderID
		if resolveID == "" {
			resolveID = builderModelID
		}
		resolved, err := localRegistry.Resolve(resolveID, "", ".")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		service := expertbuilder.NewService()
		result, err := service.Generate(context.Background(), resolved.Spec, cfg.LLM, skillcatalog.Discover(), req.Messages)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, generateExpertSettingsResponse{
			AssistantMessage: result.AssistantMessage,
			Draft:            expertSettingsFromDraft(result.Draft, resolveID),
			Warnings:         result.Warnings,
			RawJSON:          result.RawJSON,
		})
	}
}

func firstBuilderModelID(cfg config.Config) string {
	for _, expert := range cfg.Experts {
		if strings.EqualFold(strings.TrimSpace(expert.ManagedSource), config.ManagedSourceLLMModel) && strings.TrimSpace(expert.Provider) != "process" {
			return strings.TrimSpace(expert.ID)
		}
	}
	return "codex"
}

func buildExpertSettingsResponse(cfg config.Config, deps Deps) (expertSettingsResponse, error) {
	if err := config.RebuildExperts(&cfg); err != nil {
		return expertSettingsResponse{}, err
	}
	items := make([]expertSettingsItem, 0, len(cfg.Experts))
	for _, expert := range cfg.Experts {
		items = append(items, toExpertSettingsItem(expert))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	builderExperts := make([]publicExpertRef, 0)
	if deps.Experts != nil {
		for _, expert := range deps.Experts.ListPublic() {
			if strings.TrimSpace(expert.Provider) == "process" {
				continue
			}
			builderExperts = append(builderExperts, publicExpertRef{
				ID:          expert.ID,
				Label:       expert.Label,
				Provider:    expert.Provider,
				Model:       expert.Model,
				Description: expert.Description,
			})
		}
	}
	return expertSettingsResponse{
		Experts:        items,
		Skills:         toSkillCatalogItems(skillcatalog.Discover()),
		BuilderExperts: builderExperts,
	}, nil
}

func toExpertSettingsItem(expert config.ExpertConfig) expertSettingsItem {
	return expertSettingsItem{
		ID:                strings.TrimSpace(expert.ID),
		Label:             strings.TrimSpace(expert.Label),
		Description:       strings.TrimSpace(expert.Description),
		Category:          strings.TrimSpace(expert.Category),
		Avatar:            strings.TrimSpace(expert.Avatar),
		ManagedSource:     strings.TrimSpace(expert.ManagedSource),
		PrimaryModelID:    strings.TrimSpace(expert.PrimaryModelID),
		SecondaryModelID:  strings.TrimSpace(expert.SecondaryModelID),
		FallbackOn:        append([]string(nil), expert.FallbackOn...),
		EnabledSkills:     append([]string(nil), expert.EnabledSkills...),
		Provider:          strings.TrimSpace(expert.Provider),
		Model:             strings.TrimSpace(expert.Model),
		SystemPrompt:      strings.TrimSpace(expert.SystemPrompt),
		PromptTemplate:    strings.TrimSpace(expert.PromptTemplate),
		OutputFormat:      strings.TrimSpace(expert.OutputFormat),
		MaxOutputTokens:   expert.MaxOutputTokens,
		Temperature:       expert.Temperature,
		TimeoutMs:         expert.TimeoutMs,
		BuilderExpertID:   strings.TrimSpace(expert.BuilderExpertID),
		BuilderSessionID:  strings.TrimSpace(expert.BuilderSessionID),
		BuilderSnapshotID: strings.TrimSpace(expert.BuilderSnapshotID),
		GeneratedBy:       strings.TrimSpace(expert.GeneratedBy),
		GeneratedAt:       expert.GeneratedAt,
		UpdatedAt:         expert.UpdatedAt,
		Enabled:           !expert.Disabled,
		Editable:          strings.EqualFold(strings.TrimSpace(expert.ManagedSource), config.ManagedSourceExpertProfile),
	}
}

func toSkillCatalogItems(entries []skillcatalog.Entry) []skillCatalogItem {
	out := make([]skillCatalogItem, 0, len(entries))
	for _, entry := range entries {
		out = append(out, skillCatalogItem{ID: entry.ID, Description: entry.Description, Path: entry.Path})
	}
	return out
}

func expertSettingsFromDraft(draft expertbuilder.Draft, builderID string) expertSettingsItem {
	now := time.Now().UnixMilli()
	return expertSettingsItem{
		ID:                strings.TrimSpace(draft.ID),
		Label:             strings.TrimSpace(draft.Label),
		Description:       strings.TrimSpace(draft.Description),
		Category:          strings.TrimSpace(draft.Category),
		Avatar:            strings.TrimSpace(draft.Avatar),
		ManagedSource:     config.ManagedSourceExpertProfile,
		PrimaryModelID:    strings.TrimSpace(draft.PrimaryModelID),
		SecondaryModelID:  strings.TrimSpace(draft.SecondaryModelID),
		FallbackOn:        append([]string(nil), draft.FallbackOn...),
		EnabledSkills:     append([]string(nil), draft.EnabledSkills...),
		SystemPrompt:      strings.TrimSpace(draft.SystemPrompt),
		PromptTemplate:    strings.TrimSpace(draft.PromptTemplate),
		OutputFormat:      strings.TrimSpace(draft.OutputFormat),
		MaxOutputTokens:   draft.MaxOutputTokens,
		Temperature:       draft.Temperature,
		TimeoutMs:         draft.TimeoutMs,
		BuilderExpertID:   strings.TrimSpace(builderID),
		BuilderSessionID:  "",
		BuilderSnapshotID: "",
		GeneratedBy:       "expert-creator",
		GeneratedAt:       now,
		UpdatedAt:         now,
		Enabled:           true,
		Editable:          true,
	}
}
