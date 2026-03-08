package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
)

type mcpSettingsResponse struct {
	Servers []mcpServerPublic `json:"servers"`
	Tools   []cliToolPublic   `json:"tools"`
}

type mcpServerPublic struct {
	ID                       string         `json:"id"`
	Label                    string         `json:"label,omitempty"`
	Enabled                  bool           `json:"enabled"`
	EnabledCLIToolIDs        []string       `json:"enabled_cli_tool_ids,omitempty"`
	DefaultEnabledCLIToolIDs []string       `json:"default_enabled_cli_tool_ids,omitempty"`
	Config                   map[string]any `json:"config,omitempty"`
}

type putMCPSettingsRequest struct {
	Servers []mcpServerPublic `json:"servers"`
}

func getMCPSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildMCPSettingsResponse(cfg))
	}
}

func putMCPSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req putMCPSettingsRequest
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
		next := make([]config.MCPServerConfig, 0, len(req.Servers))
		for _, item := range req.Servers {
			next = append(next, config.MCPServerConfig{
				ID:                       strings.TrimSpace(item.ID),
				Label:                    strings.TrimSpace(item.Label),
				Enabled:                  item.Enabled,
				EnabledCLIToolIDs:        append([]string(nil), item.EnabledCLIToolIDs...),
				DefaultEnabledCLIToolIDs: append([]string(nil), item.DefaultEnabledCLIToolIDs...),
				Config:                   cloneJSONMap(item.Config),
			})
		}
		cfg.MCPServers = next
		if err := config.NormalizeMCPServers(&cfg.MCPServers, cfg.CLITools); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildMCPSettingsResponse(cfg))
	}
}

func buildMCPSettingsResponse(cfg config.Config) mcpSettingsResponse {
	servers := make([]mcpServerPublic, 0, len(cfg.MCPServers))
	for _, item := range cfg.MCPServers {
		servers = append(servers, mcpServerPublic{
			ID:                       strings.TrimSpace(item.ID),
			Label:                    strings.TrimSpace(item.Label),
			Enabled:                  item.Enabled,
			EnabledCLIToolIDs:        append([]string(nil), item.EnabledCLIToolIDs...),
			DefaultEnabledCLIToolIDs: append([]string(nil), item.DefaultEnabledCLIToolIDs...),
			Config:                   cloneJSONMap(item.Config),
		})
	}
	return mcpSettingsResponse{Servers: servers, Tools: buildCLIToolSettingsResponse(cfg).Tools}
}
