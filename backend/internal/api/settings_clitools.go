package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vibe-tree/backend/internal/config"
	iflowcli "vibe-tree/backend/internal/iflow"
)

type cliToolSettingsResponse struct {
	Tools  []cliToolPublic  `json:"tools"`
	Models []llmModelPublic `json:"models"`
}

type cliToolPublic struct {
	ID                        string   `json:"id"`
	Label                     string   `json:"label"`
	ProtocolFamily            string   `json:"protocol_family"`
	ProtocolFamilies          []string `json:"protocol_families,omitempty"`
	CLIFamily                 string   `json:"cli_family"`
	DefaultModelID            string   `json:"default_model_id,omitempty"`
	CommandPath               string   `json:"command_path,omitempty"`
	Enabled                   bool     `json:"enabled"`
	IFLOWAuthMode             string   `json:"iflow_auth_mode,omitempty"`
	IFLOWBaseURL              string   `json:"iflow_base_url,omitempty"`
	IFLOWModels               []string `json:"iflow_models,omitempty"`
	IFLOWDefaultModel         string   `json:"iflow_default_model,omitempty"`
	IFLOWHasKey               bool     `json:"iflow_has_key,omitempty"`
	IFLOWMaskedKey            string   `json:"iflow_masked_key,omitempty"`
	IFLOWBrowserAuthenticated bool     `json:"iflow_browser_authenticated,omitempty"`
	IFLOWBrowserModel         string   `json:"iflow_browser_model,omitempty"`
}

type putCLIToolSettingsRequest struct {
	Tools []putCLIToolPublic `json:"tools"`
}

type putCLIToolPublic struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	ProtocolFamily    string   `json:"protocol_family"`
	ProtocolFamilies  []string `json:"protocol_families,omitempty"`
	CLIFamily         string   `json:"cli_family"`
	DefaultModelID    string   `json:"default_model_id,omitempty"`
	CommandPath       string   `json:"command_path,omitempty"`
	Enabled           bool     `json:"enabled"`
	IFLOWAuthMode     string   `json:"iflow_auth_mode,omitempty"`
	IFLOWBaseURL      string   `json:"iflow_base_url,omitempty"`
	IFLOWModels       []string `json:"iflow_models,omitempty"`
	IFLOWDefaultModel string   `json:"iflow_default_model,omitempty"`
	IFLOWAPIKey       *string  `json:"iflow_api_key,omitempty"`
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

		existingIFLOWKeys := map[string]string{}
		existingProtocolFamilies := map[string][]string{}
		for _, item := range cfg.CLITools {
			toolID := strings.TrimSpace(item.ID)
			if toolID == "" {
				continue
			}
			existingProtocolFamilies[toolID] = append([]string(nil), config.CLIToolProtocolFamilies(item)...)
			if normalizeCLIFamily(item.CLIFamily) != "iflow" && toolID != "iflow" {
				continue
			}
			existingIFLOWKeys[toolID] = strings.TrimSpace(item.IFlowAPIKey)
		}

		next := make([]config.CLIToolConfig, 0, len(req.Tools))
		for _, item := range req.Tools {
			toolID := strings.TrimSpace(item.ID)
			apiKey := existingIFLOWKeys[toolID]
			if item.IFLOWAPIKey != nil {
				apiKey = strings.TrimSpace(*item.IFLOWAPIKey)
			}
			protocolFamilies := normalizeStringList(item.ProtocolFamilies)
			if len(protocolFamilies) == 0 {
				protocolFamilies = append([]string(nil), existingProtocolFamilies[toolID]...)
			}
			next = append(next, config.CLIToolConfig{
				ID:                toolID,
				Label:             strings.TrimSpace(item.Label),
				ProtocolFamily:    strings.TrimSpace(item.ProtocolFamily),
				ProtocolFamilies:  protocolFamilies,
				CLIFamily:         strings.TrimSpace(item.CLIFamily),
				DefaultModelID:    strings.TrimSpace(item.DefaultModelID),
				CommandPath:       strings.TrimSpace(item.CommandPath),
				Enabled:           item.Enabled,
				IFlowAuthMode:     strings.TrimSpace(item.IFLOWAuthMode),
				IFlowAPIKey:       apiKey,
				IFlowBaseURL:      strings.TrimSpace(item.IFLOWBaseURL),
				IFlowModels:       append([]string(nil), item.IFLOWModels...),
				IFlowDefaultModel: strings.TrimSpace(item.IFLOWDefaultModel),
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
	browserStatus, _ := iflowcli.DetectBrowserAuthStatus()
	tools := make([]cliToolPublic, 0, len(cfg.CLITools))
	for _, item := range cfg.CLITools {
		tool := cliToolPublic{
			ID:               item.ID,
			Label:            item.Label,
			ProtocolFamily:   item.ProtocolFamily,
			ProtocolFamilies: append([]string(nil), config.CLIToolProtocolFamilies(item)...),
			CLIFamily:        item.CLIFamily,
			DefaultModelID:   item.DefaultModelID,
			CommandPath:      item.CommandPath,
			Enabled:          item.Enabled,
		}
		if normalizeCLIFamily(item.CLIFamily) == "iflow" || strings.TrimSpace(item.ID) == "iflow" {
			tool.IFLOWAuthMode = item.IFlowAuthMode
			tool.IFLOWBaseURL = item.IFlowBaseURL
			tool.IFLOWModels = append([]string(nil), item.IFlowModels...)
			if browserModel := strings.TrimSpace(browserStatus.ModelName); browserModel != "" && !containsString(tool.IFLOWModels, browserModel) {
				tool.IFLOWModels = append(tool.IFLOWModels, browserModel)
			}
			tool.IFLOWDefaultModel = item.IFlowDefaultModel
			if browserModel := strings.TrimSpace(browserStatus.ModelName); browserModel != "" && (strings.TrimSpace(tool.IFLOWDefaultModel) == "" || strings.TrimSpace(tool.IFLOWDefaultModel) == iflowcli.DefaultModel) {
				tool.IFLOWDefaultModel = browserModel
			}
			tool.IFLOWHasKey = strings.TrimSpace(item.IFlowAPIKey) != ""
			tool.IFLOWMaskedKey = maskKey(item.IFlowAPIKey)
			tool.IFLOWBrowserAuthenticated = browserStatus.Authenticated
			tool.IFLOWBrowserModel = browserStatus.ModelName
		}
		tools = append(tools, tool)
	}
	models := []llmModelPublic{}
	if cfg.LLM != nil {
		models = buildLLMSettingsResponse(*cfg.LLM).Models
	}
	return cliToolSettingsResponse{Tools: tools, Models: models}
}

func normalizeCLIFamily(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
