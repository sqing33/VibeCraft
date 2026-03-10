package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
)

type runtimeModelSettingsResponse struct {
	Runtimes []runtimeModelRuntimePublic `json:"runtimes"`
}

type runtimeModelRuntimePublic struct {
	ID             string                   `json:"id"`
	Label          string                   `json:"label"`
	Kind           string                   `json:"kind"`
	Provider       string                   `json:"provider,omitempty"`
	CLIToolID      string                   `json:"cli_tool_id,omitempty"`
	DefaultModelID string                   `json:"default_model_id,omitempty"`
	Models         []runtimeModelPublicItem `json:"models"`
}

type runtimeModelPublicItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	SourceID string `json:"source_id"`
}

type putRuntimeModelSettingsRequest struct {
	Runtimes []putRuntimeModelRuntime `json:"runtimes"`
}

type putRuntimeModelRuntime struct {
	ID             string                `json:"id"`
	DefaultModelID string                `json:"default_model_id,omitempty"`
	Models         []putRuntimeModelItem `json:"models"`
}

type putRuntimeModelItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	SourceID string `json:"source_id"`
}

func getRuntimeModelSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildRuntimeModelSettingsResponse(cfg.RuntimeModels))
	}
}

func putRuntimeModelSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putRuntimeModelSettingsRequest
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
		next := &config.RuntimeModelSettings{Runtimes: make([]config.RuntimeModelRuntimeConfig, 0, len(req.Runtimes))}
		for _, runtime := range req.Runtimes {
			item := config.RuntimeModelRuntimeConfig{
				ID:             strings.TrimSpace(runtime.ID),
				DefaultModelID: strings.TrimSpace(runtime.DefaultModelID),
				Models:         make([]config.RuntimeModelConfig, 0, len(runtime.Models)),
			}
			for _, model := range runtime.Models {
				item.Models = append(item.Models, config.RuntimeModelConfig{
					ID:       strings.TrimSpace(model.ID),
					Label:    strings.TrimSpace(model.Label),
					Provider: strings.TrimSpace(model.Provider),
					Model:    strings.TrimSpace(model.Model),
					SourceID: strings.TrimSpace(model.SourceID),
				})
			}
			next.Runtimes = append(next.Runtimes, item)
		}
		cfg.RuntimeModels = next
		if err := config.NormalizeRuntimeModelSettings(&cfg.RuntimeModels, cfg.APISources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.HydrateRuntimeSettings(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.ReconcileBasicSettingsWithRuntime(&cfg.Basic, cfg)
		if err := config.RebuildExperts(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if deps.Experts != nil {
			deps.Experts.Reload(cfg)
		}
		c.JSON(http.StatusOK, buildRuntimeModelSettingsResponse(cfg.RuntimeModels))
	}
}

func buildRuntimeModelSettingsResponse(settings *config.RuntimeModelSettings) runtimeModelSettingsResponse {
	if settings == nil {
		return runtimeModelSettingsResponse{Runtimes: []runtimeModelRuntimePublic{}}
	}
	resp := runtimeModelSettingsResponse{Runtimes: make([]runtimeModelRuntimePublic, 0, len(settings.Runtimes))}
	for _, runtime := range settings.Runtimes {
		publicRuntime := runtimeModelRuntimePublic{
			ID:             strings.TrimSpace(runtime.ID),
			Label:          strings.TrimSpace(runtime.Label),
			Kind:           strings.TrimSpace(runtime.Kind),
			Provider:       strings.TrimSpace(runtime.Provider),
			CLIToolID:      strings.TrimSpace(runtime.CLIToolID),
			DefaultModelID: strings.TrimSpace(runtime.DefaultModelID),
			Models:         make([]runtimeModelPublicItem, 0, len(runtime.Models)),
		}
		for _, model := range runtime.Models {
			publicRuntime.Models = append(publicRuntime.Models, runtimeModelPublicItem{
				ID:       strings.TrimSpace(model.ID),
				Label:    strings.TrimSpace(model.Label),
				Provider: strings.TrimSpace(model.Provider),
				Model:    strings.TrimSpace(model.Model),
				SourceID: strings.TrimSpace(model.SourceID),
			})
		}
		resp.Runtimes = append(resp.Runtimes, publicRuntime)
	}
	return resp
}
