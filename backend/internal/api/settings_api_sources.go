package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibecraft/backend/internal/config"
)

type apiSourceSettingsResponse struct {
	Sources []apiSourcePublic `json:"sources"`
}

type apiSourcePublic struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	BaseURL   string `json:"base_url,omitempty"`
	AuthMode  string `json:"auth_mode,omitempty"`
	HasKey    bool   `json:"has_key"`
	MaskedKey string `json:"masked_key,omitempty"`
}

type putAPISourceSettingsRequest struct {
	Sources []putAPISourcePublic `json:"sources"`
}

type putAPISourcePublic struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	BaseURL  string  `json:"base_url,omitempty"`
	AuthMode string  `json:"auth_mode,omitempty"`
	APIKey   *string `json:"api_key,omitempty"`
}

func getAPISourceSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildAPISourceSettingsResponse(cfg.APISources))
	}
}

func putAPISourceSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putAPISourceSettingsRequest
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
		existingKeys := make(map[string]string, len(cfg.APISources))
		for _, source := range cfg.APISources {
			existingKeys[strings.TrimSpace(source.ID)] = strings.TrimSpace(source.APIKey)
		}
		next := make([]config.APISourceConfig, 0, len(req.Sources))
		for _, source := range req.Sources {
			id := strings.TrimSpace(source.ID)
			apiKey := existingKeys[id]
			if source.APIKey != nil {
				apiKey = strings.TrimSpace(*source.APIKey)
			}
			next = append(next, config.APISourceConfig{
				ID:       id,
				Label:    strings.TrimSpace(source.Label),
				BaseURL:  strings.TrimSpace(source.BaseURL),
				AuthMode: strings.TrimSpace(source.AuthMode),
				APIKey:   apiKey,
			})
		}
		cfg.APISources = next
		if err := config.NormalizeAPISources(&cfg.APISources); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		reconcileRuntimeModelSourceBindings(cfg.RuntimeModels, cfg.APISources)
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
		c.JSON(http.StatusOK, buildAPISourceSettingsResponse(cfg.APISources))
	}
}

func buildAPISourceSettingsResponse(sources []config.APISourceConfig) apiSourceSettingsResponse {
	resp := apiSourceSettingsResponse{Sources: make([]apiSourcePublic, 0, len(sources))}
	for _, source := range sources {
		key := strings.TrimSpace(source.APIKey)
		resp.Sources = append(resp.Sources, apiSourcePublic{
			ID:        strings.TrimSpace(source.ID),
			Label:     strings.TrimSpace(source.Label),
			BaseURL:   strings.TrimSpace(source.BaseURL),
			AuthMode:  strings.TrimSpace(source.AuthMode),
			HasKey:    key != "",
			MaskedKey: maskKey(key),
		})
	}
	return resp
}

func reconcileRuntimeModelSourceBindings(settings *config.RuntimeModelSettings, sources []config.APISourceConfig) {
	if settings == nil {
		return
	}
	sourceByID := make(map[string]struct{}, len(sources))
	fallbackGeneral := ""
	fallbackIFLOW := ""
	for _, source := range sources {
		id := strings.TrimSpace(source.ID)
		if id == "" {
			continue
		}
		sourceByID[id] = struct{}{}
		isIFLOW := strings.TrimSpace(source.AuthMode) != "" || strings.EqualFold(strings.TrimSpace(source.Provider), config.ProviderIFLOW)
		if isIFLOW {
			if fallbackIFLOW == "" {
				fallbackIFLOW = id
			}
			continue
		}
		if fallbackGeneral == "" {
			fallbackGeneral = id
		}
	}
	for i := range settings.Runtimes {
		runtimeID := strings.TrimSpace(settings.Runtimes[i].ID)
		fallbackID := fallbackGeneral
		if runtimeID == config.RuntimeIDIFLOW {
			fallbackID = fallbackIFLOW
		}
		if fallbackID == "" {
			continue
		}
		for j := range settings.Runtimes[i].Models {
			currentID := strings.TrimSpace(settings.Runtimes[i].Models[j].SourceID)
			if _, ok := sourceByID[currentID]; ok {
				continue
			}
			settings.Runtimes[i].Models[j].SourceID = fallbackID
		}
	}
}
