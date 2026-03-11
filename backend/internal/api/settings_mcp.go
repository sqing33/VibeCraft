package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/mcpgateway"
)

type mcpSettingsResponse struct {
	Servers []mcpServerPublic       `json:"servers"`
	Tools   []cliToolPublic         `json:"tools"`
	Gateway mcpGatewaySettingsPublic `json:"gateway"`
}

type mcpServerPublic struct {
	ID                       string         `json:"id"`
	RawJSON                  string         `json:"raw_json"`
	DefaultEnabledCLIToolIDs []string       `json:"default_enabled_cli_tool_ids,omitempty"`
	Config                   map[string]any `json:"config,omitempty"`
}

type mcpGatewaySettingsPublic struct {
	Enabled        bool   `json:"enabled"`
	IdleTTLSeconds int    `json:"idle_ttl_seconds"`
	Reachable      bool   `json:"reachable"`
	Sessions       int    `json:"sessions,omitempty"`
	StatusPath     string `json:"status_path,omitempty"`
}

type putMCPSettingsRequest struct {
	Servers []mcpServerPublic        `json:"servers"`
	Gateway *mcpGatewaySettingsInput `json:"gateway,omitempty"`
}

type mcpGatewaySettingsInput struct {
	Enabled        bool `json:"enabled"`
	IdleTTLSeconds int  `json:"idle_ttl_seconds"`
}

// ValidateMCPServerConfig validates a single MCP server config before persisting.
// It is a var to allow tests to stub validation.
var ValidateMCPServerConfig = mcpgateway.ValidateDownstreamConfig

func getMCPSettingsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, _, err := config.LoadPersisted()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buildMCPSettingsResponse(cfg, deps))
	}
}

func putMCPSettingsHandler(deps Deps) gin.HandlerFunc {
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
		if req.Gateway != nil {
			cfg.MCPGateway = &config.MCPGatewaySettings{
				Enabled:        req.Gateway.Enabled,
				IdleTTLSeconds: req.Gateway.IdleTTLSeconds,
			}
		}
		if err := config.NormalizeMCPServers(&cfg.MCPServers, cfg.CLITools); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.NormalizeMCPGatewaySettings(&cfg.MCPGateway)

		validateCtx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
		defer cancel()
		for _, server := range cfg.MCPServers {
			if err := ValidateMCPServerConfig(validateCtx, server.Config); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "mcp server " + strconv.Quote(strings.TrimSpace(server.ID)) + " is not runnable: " + err.Error()})
				return
			}
		}
		if err := config.SaveTo(cfgPath, cfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if deps.MCPGateway != nil {
			deps.MCPGateway.ReloadConfig(cfg)
		}
		c.JSON(http.StatusOK, buildMCPSettingsResponse(cfg, deps))
	}
}

func getMCPGatewayStatusHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.MCPGateway == nil {
			c.JSON(http.StatusOK, gin.H{"enabled": false, "reachable": false})
			return
		}
		c.JSON(http.StatusOK, deps.MCPGateway.Status())
	}
}

func buildMCPSettingsResponse(cfg config.Config, deps Deps) mcpSettingsResponse {
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
	config.NormalizeMCPGatewaySettings(&cfg.MCPGateway)
	gateway := mcpGatewaySettingsPublic{
		StatusPath: "/api/v1/mcp-gateway/status",
	}
	if cfg.MCPGateway != nil {
		gateway.Enabled = cfg.MCPGateway.Enabled
		gateway.IdleTTLSeconds = cfg.MCPGateway.IdleTTLSeconds
	}
	if deps.MCPGateway != nil {
		status := deps.MCPGateway.Status()
		gateway.Reachable = status.Reachable
		gateway.Sessions = status.Sessions
	}
	return mcpSettingsResponse{Servers: servers, Tools: buildCLIToolSettingsResponse(cfg).Tools, Gateway: gateway}
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
