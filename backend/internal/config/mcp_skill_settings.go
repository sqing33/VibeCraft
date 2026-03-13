package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vibecraft/backend/internal/skillcatalog"
)

const defaultMCPGatewayIdleTTLSeconds = 600

func NormalizeMCPGatewaySettings(settings **MCPGatewaySettings) {
	if settings == nil {
		return
	}
	if *settings == nil {
		*settings = &MCPGatewaySettings{}
	}
	if (*settings).IdleTTLSeconds <= 0 {
		(*settings).IdleTTLSeconds = defaultMCPGatewayIdleTTLSeconds
	}
}

func NormalizeMCPServers(servers *[]MCPServerConfig, cliTools []CLIToolConfig) error {
	if servers == nil {
		return nil
	}
	if len(*servers) == 0 {
		*servers = nil
		return nil
	}
	allowedTools := cliToolIDSet(cliTools)
	seen := make(map[string]struct{}, len(*servers))
	out := make([]MCPServerConfig, 0, len(*servers))
	for i := range *servers {
		item := (*servers)[i]
		item.ID = strings.TrimSpace(item.ID)
		item.RawJSON = strings.TrimSpace(item.RawJSON)
		item.DefaultEnabledCLIToolIDs = normalizeCLIToolIDList(item.DefaultEnabledCLIToolIDs, allowedTools)
		if item.RawJSON != "" {
			parsed, err := ParseMCPServersJSON(item.RawJSON)
			if err != nil {
				return fmt.Errorf("mcp_servers[%d].raw_json: %w", i, err)
			}
			if len(parsed) != 1 {
				return fmt.Errorf("mcp_servers[%d].raw_json must contain exactly one server", i)
			}
			parsedItem := parsed[0]
			if item.ID != "" && item.ID != parsedItem.ID {
				return fmt.Errorf("mcp_servers[%d].id %q does not match raw_json id %q", i, item.ID, parsedItem.ID)
			}
			item.ID = parsedItem.ID
			if len(item.Config) == 0 {
				item.Config = parsedItem.Config
			}
			if item.RawJSON == "" {
				item.RawJSON = parsedItem.RawJSON
			}
		}
		if item.ID == "" {
			return fmt.Errorf("mcp_servers[%d].id is required", i)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("mcp_servers[%d].id %q is duplicated", i, item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Config == nil {
			item.Config = map[string]any{}
		}
		if len(item.Config) == 0 {
			return fmt.Errorf("mcp_servers[%d].config is required", i)
		}
		item.RawJSON = canonicalMCPServerRawJSON(item.ID, item.Config)
		out = append(out, MCPServerConfig{
			ID:                       item.ID,
			DefaultEnabledCLIToolIDs: append([]string(nil), item.DefaultEnabledCLIToolIDs...),
			Config:                   cloneJSONMap(item.Config),
			RawJSON:                  item.RawJSON,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	*servers = out
	return nil
}

func ParseMCPServersJSON(raw string) ([]MCPServerConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("JSON is required")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	registry := payload
	if nested, ok := payload["mcpServers"]; ok {
		nestedMap, ok := nested.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("mcpServers must be an object")
		}
		registry = nestedMap
	}
	if len(registry) == 0 {
		return nil, fmt.Errorf("at least one MCP server is required")
	}
	keys := make([]string, 0, len(registry))
	for key := range registry {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]MCPServerConfig, 0, len(keys))
	for _, key := range keys {
		id := strings.TrimSpace(key)
		if id == "" {
			return nil, fmt.Errorf("MCP server key cannot be empty")
		}
		cfgMap, ok := registry[key].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("MCP server %q must be an object", id)
		}
		configMap := cloneJSONMap(cfgMap)
		out = append(out, MCPServerConfig{
			ID:      id,
			Config:  configMap,
			RawJSON: canonicalMCPServerRawJSON(id, configMap),
		})
	}
	return out, nil
}

func DefaultEnabledMCPServerIDs(cfg Config, cliToolID string) []string {
	cliToolID = strings.TrimSpace(cliToolID)
	if cliToolID == "" {
		return nil
	}
	out := make([]string, 0)
	for _, server := range cfg.MCPServers {
		if !containsString(server.DefaultEnabledCLIToolIDs, cliToolID) {
			continue
		}
		out = append(out, server.ID)
	}
	sort.Strings(out)
	return out
}

func EffectiveMCPServers(cfg Config, cliToolID string, selectedIDs []string) map[string]map[string]any {
	cliToolID = strings.TrimSpace(cliToolID)
	if cliToolID == "" {
		return nil
	}
	if selectedIDs == nil {
		selectedIDs = DefaultEnabledMCPServerIDs(cfg, cliToolID)
	}
	selected := make(map[string]struct{}, len(selectedIDs))
	for _, item := range selectedIDs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		selected[item] = struct{}{}
	}
	if len(selected) == 0 {
		return nil
	}
	out := make(map[string]map[string]any)
	for _, server := range cfg.MCPServers {
		if _, ok := selected[server.ID]; !ok {
			continue
		}
		if len(server.Config) == 0 {
			continue
		}
		out[server.ID] = cloneJSONMap(server.Config)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func NormalizeSkillBindings(bindings *[]SkillBindingConfig, _ []CLIToolConfig) error {
	if bindings == nil {
		return nil
	}
	if len(*bindings) == 0 {
		*bindings = nil
		return nil
	}
	seen := make(map[string]struct{}, len(*bindings))
	out := make([]SkillBindingConfig, 0, len(*bindings))
	for i := range *bindings {
		item := (*bindings)[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Description = strings.TrimSpace(item.Description)
		item.Path = strings.TrimSpace(item.Path)
		item.Source = strings.TrimSpace(item.Source)
		if item.ID == "" {
			return fmt.Errorf("skill_bindings[%d].id is required", i)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("skill_bindings[%d].id %q is duplicated", i, item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Path != "" {
			item.Path = filepath.Clean(item.Path)
		}
		out = append(out, SkillBindingConfig{
			ID:          item.ID,
			Description: item.Description,
			Path:        item.Path,
			Source:      item.Source,
			Enabled:     item.Enabled,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	*bindings = out
	return nil
}

func EffectiveSkillCatalogEntries(cfg Config, _ string, expertSkillIDs []string, discovered []skillcatalog.Entry) []skillcatalog.Entry {
	if len(discovered) == 0 {
		discovered = skillcatalog.Discover()
	}
	if len(discovered) == 0 {
		return nil
	}
	enabledBindings := make(map[string]bool, len(cfg.SkillBindings))
	for _, item := range cfg.SkillBindings {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		enabledBindings[id] = item.Enabled
	}
	allowedExpertSkills := make(map[string]struct{}, len(expertSkillIDs))
	for _, item := range expertSkillIDs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		allowedExpertSkills[item] = struct{}{}
	}
	strictExpert := len(allowedExpertSkills) > 0
	out := make([]skillcatalog.Entry, 0, len(discovered))
	seen := make(map[string]struct{}, len(discovered))
	for _, entry := range discovered {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if enabled, ok := enabledBindings[id]; ok && !enabled {
			continue
		}
		if strictExpert {
			if _, ok := allowedExpertSkills[id]; !ok {
				continue
			}
		}
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, skillcatalog.Entry{
			ID:          id,
			Description: strings.TrimSpace(entry.Description),
			Path:        path,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func normalizeCLIToolIDList(values []string, allowed map[string]struct{}) []string {
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
		if len(allowed) > 0 {
			if _, ok := allowed[value]; !ok {
				continue
			}
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

func cliToolIDSet(cliTools []CLIToolConfig) map[string]struct{} {
	out := make(map[string]struct{}, len(cliTools))
	for _, item := range cliTools {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func inferSkillSource(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.Contains(path, string(filepath.Separator)+".codex"+string(filepath.Separator)+"skills") {
		return "codex"
	}
	if strings.Contains(path, string(filepath.Separator)+".claude"+string(filepath.Separator)+"skills") {
		return "claude"
	}
	return "filesystem"
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

func cloneJSONMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = cloneJSONValue(value)
	}
	return out
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneJSONMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneJSONValue(item))
		}
		return out
	default:
		return typed
	}
}

func canonicalMCPServerRawJSON(id string, cfg map[string]any) string {
	payload := map[string]any{
		"mcpServers": map[string]any{
			strings.TrimSpace(id): cloneJSONMap(cfg),
		},
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
