package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
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
	RawJSON                  string         `json:"raw_json"`
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
		for index, item := range req.Servers {
			parsed, err := config.ParseMCPServersJSON(item.RawJSON)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "servers[" + strconv.Itoa(index) + "]: " + err.Error()})
				return
			}
			defaults := normalizeStringList(item.DefaultEnabledCLIToolIDs)
			for _, server := range parsed {
				server.DefaultEnabledCLIToolIDs = append([]string(nil), defaults...)
				next = append(next, server)
			}
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
			RawJSON:                  strings.TrimSpace(item.RawJSON),
			DefaultEnabledCLIToolIDs: append([]string(nil), item.DefaultEnabledCLIToolIDs...),
			Config:                   cloneJSONMap(item.Config),
		})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
	return mcpSettingsResponse{Servers: servers, Tools: buildCLIToolSettingsResponse(cfg).Tools}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
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
	sort.Strings(out)
	return out
}
