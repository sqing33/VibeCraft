package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
)

type cliToolSettingsResponse struct {
	Tools  []cliToolPublic  `json:"tools"`
	Models []llmModelPublic `json:"models"`
}

type cliToolPublic struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	ProtocolFamily string `json:"protocol_family"`
	CLIFamily      string `json:"cli_family"`
	DefaultModelID string `json:"default_model_id,omitempty"`
	CommandPath    string `json:"command_path,omitempty"`
	Enabled        bool   `json:"enabled"`
}

type putCLIToolSettingsRequest struct {
	Tools []cliToolPublic `json:"tools"`
}

func getCLIToolSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildCLIToolSettingsResponse(cfg))
	}
}

func putCLIToolSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putCLIToolSettingsRequest
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
		next := make([]config.CLIToolConfig, 0, len(req.Tools))
		for _, item := range req.Tools {
			next = append(next, config.CLIToolConfig{
				ID:             strings.TrimSpace(item.ID),
				Label:          strings.TrimSpace(item.Label),
				ProtocolFamily: strings.TrimSpace(item.ProtocolFamily),
				CLIFamily:      strings.TrimSpace(item.CLIFamily),
				DefaultModelID: strings.TrimSpace(item.DefaultModelID),
				CommandPath:    strings.TrimSpace(item.CommandPath),
				Enabled:        item.Enabled,
			})
		}
		cfg.CLITools = next
		if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
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
		c.JSON(http.StatusOK, buildCLIToolSettingsResponse(cfg))
	}
}

func buildCLIToolSettingsResponse(cfg config.Config) cliToolSettingsResponse {
	tools := make([]cliToolPublic, 0, len(cfg.CLITools))
	for _, item := range cfg.CLITools {
		tools = append(tools, cliToolPublic{
			ID:             item.ID,
			Label:          item.Label,
			ProtocolFamily: item.ProtocolFamily,
			CLIFamily:      item.CLIFamily,
			DefaultModelID: item.DefaultModelID,
			CommandPath:    item.CommandPath,
			Enabled:        item.Enabled,
		})
	}
	models := []llmModelPublic{}
	if cfg.LLM != nil {
		models = buildLLMSettingsResponse(*cfg.LLM).Models
	}
	return cliToolSettingsResponse{Tools: tools, Models: models}
}
