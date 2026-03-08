package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/skillcatalog"
)

type skillSettingsResponse struct {
	Skills []skillBindingPublic `json:"skills"`
	Tools  []cliToolPublic      `json:"tools"`
}

type skillBindingPublic struct {
	ID                string   `json:"id"`
	Description       string   `json:"description,omitempty"`
	Path              string   `json:"path,omitempty"`
	Source            string   `json:"source,omitempty"`
	Enabled           bool     `json:"enabled"`
	EnabledCLIToolIDs []string `json:"enabled_cli_tool_ids,omitempty"`
}

type putSkillSettingsRequest struct {
	Skills []skillBindingPublic `json:"skills"`
}

func getSkillSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildSkillSettingsResponse(cfg))
	}
}

func putSkillSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putSkillSettingsRequest
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
		next := make([]config.SkillBindingConfig, 0, len(req.Skills))
		for _, item := range req.Skills {
			next = append(next, config.SkillBindingConfig{
				ID:                strings.TrimSpace(item.ID),
				Description:       strings.TrimSpace(item.Description),
				Path:              strings.TrimSpace(item.Path),
				Source:            strings.TrimSpace(item.Source),
				Enabled:           item.Enabled,
				EnabledCLIToolIDs: append([]string(nil), item.EnabledCLIToolIDs...),
			})
		}
		cfg.SkillBindings = next
		if err := config.NormalizeSkillBindings(&cfg.SkillBindings, cfg.CLITools); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildSkillSettingsResponse(cfg))
	}
}

func buildSkillSettingsResponse(cfg config.Config) skillSettingsResponse {
	merged := config.MergeDiscoveredSkillBindings(cfg, skillcatalog.Discover())
	items := make([]skillBindingPublic, 0, len(merged))
	for _, item := range merged {
		items = append(items, skillBindingPublic{
			ID:                strings.TrimSpace(item.ID),
			Description:       strings.TrimSpace(item.Description),
			Path:              strings.TrimSpace(item.Path),
			Source:            strings.TrimSpace(item.Source),
			Enabled:           item.Enabled,
			EnabledCLIToolIDs: append([]string(nil), item.EnabledCLIToolIDs...),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return skillSettingsResponse{Skills: items, Tools: buildCLIToolSettingsResponse(cfg).Tools}
}
