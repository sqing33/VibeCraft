package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vibe-tree/backend/internal/skillcatalog"
)

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
		item.Label = strings.TrimSpace(item.Label)
		item.EnabledCLIToolIDs = normalizeCLIToolIDList(item.EnabledCLIToolIDs, allowedTools)
		item.DefaultEnabledCLIToolIDs = normalizeCLIToolIDList(item.DefaultEnabledCLIToolIDs, allowedTools)
		if item.ID == "" {
			return fmt.Errorf("mcp_servers[%d].id is required", i)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("mcp_servers[%d].id %q is duplicated", i, item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Label == "" {
			item.Label = item.ID
		}
		if item.Config == nil {
			item.Config = map[string]any{}
		}
		if len(item.Config) == 0 {
			return fmt.Errorf("mcp_servers[%d].config is required", i)
		}
		defaultAllowed := make(map[string]struct{}, len(item.EnabledCLIToolIDs))
		for _, toolID := range item.EnabledCLIToolIDs {
			defaultAllowed[toolID] = struct{}{}
		}
		for _, toolID := range item.DefaultEnabledCLIToolIDs {
			if _, ok := defaultAllowed[toolID]; !ok {
				return fmt.Errorf("mcp_servers[%d].default_enabled_cli_tool_ids contains %q which is not enabled", i, toolID)
			}
		}
		out = append(out, item)
	}
	*servers = out
	return nil
}

func NormalizeSkillBindings(bindings *[]SkillBindingConfig, cliTools []CLIToolConfig) error {
	if bindings == nil {
		return nil
	}
	if len(*bindings) == 0 {
		*bindings = nil
		return nil
	}
	allowedTools := cliToolIDSet(cliTools)
	seen := make(map[string]struct{}, len(*bindings))
	out := make([]SkillBindingConfig, 0, len(*bindings))
	for i := range *bindings {
		item := (*bindings)[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Description = strings.TrimSpace(item.Description)
		item.Path = strings.TrimSpace(item.Path)
		item.Source = strings.TrimSpace(item.Source)
		item.EnabledCLIToolIDs = normalizeCLIToolIDList(item.EnabledCLIToolIDs, allowedTools)
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
		out = append(out, item)
	}
	*bindings = out
	return nil
}

func DefaultEnabledMCPServerIDs(cfg Config, cliToolID string) []string {
	cliToolID = strings.TrimSpace(cliToolID)
	if cliToolID == "" {
		return nil
	}
	out := make([]string, 0)
	for _, server := range cfg.MCPServers {
		if !server.Enabled {
			continue
		}
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
		if !server.Enabled {
			continue
		}
		if !containsString(server.EnabledCLIToolIDs, cliToolID) {
			continue
		}
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

func EffectiveSkillBindings(bindings []SkillBindingConfig, cliToolID string) []SkillBindingConfig {
	cliToolID = strings.TrimSpace(cliToolID)
	if cliToolID == "" {
		return nil
	}
	out := make([]SkillBindingConfig, 0, len(bindings))
	for _, item := range bindings {
		if !item.Enabled {
			continue
		}
		if !containsString(item.EnabledCLIToolIDs, cliToolID) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func MergeDiscoveredSkillBindings(cfg Config, discovered []skillcatalog.Entry) []SkillBindingConfig {
	toolIDs := availableCLIToolIDs(cfg.CLITools)
	byID := make(map[string]SkillBindingConfig, len(cfg.SkillBindings))
	for _, item := range cfg.SkillBindings {
		byID[strings.TrimSpace(item.ID)] = item
	}
	orderedIDs := make([]string, 0, len(discovered)+len(cfg.SkillBindings))
	seen := make(map[string]struct{}, len(discovered)+len(cfg.SkillBindings))
	pushID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		orderedIDs = append(orderedIDs, id)
	}
	for _, entry := range discovered {
		pushID(entry.ID)
	}
	for _, item := range cfg.SkillBindings {
		pushID(item.ID)
	}

	discoveredByID := make(map[string]skillcatalog.Entry, len(discovered))
	for _, entry := range discovered {
		discoveredByID[strings.TrimSpace(entry.ID)] = entry
	}

	out := make([]SkillBindingConfig, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		binding, ok := byID[id]
		if !ok {
			entry := discoveredByID[id]
			binding = SkillBindingConfig{
				ID:                id,
				Description:       strings.TrimSpace(entry.Description),
				Path:              strings.TrimSpace(entry.Path),
				Source:            inferSkillSource(entry.Path),
				Enabled:           true,
				EnabledCLIToolIDs: append([]string(nil), toolIDs...),
			}
		} else {
			entry := discoveredByID[id]
			if strings.TrimSpace(binding.Description) == "" {
				binding.Description = strings.TrimSpace(entry.Description)
			}
			if strings.TrimSpace(binding.Path) == "" {
				binding.Path = strings.TrimSpace(entry.Path)
			}
			if strings.TrimSpace(binding.Source) == "" {
				binding.Source = inferSkillSource(binding.Path)
			}
		}
		out = append(out, binding)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func EffectiveSkillCatalogEntries(cfg Config, cliToolID string, expertSkillIDs []string, discovered []skillcatalog.Entry) []skillcatalog.Entry {
	bindings := EffectiveSkillBindings(MergeDiscoveredSkillBindings(cfg, discovered), cliToolID)
	if len(bindings) == 0 {
		return nil
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
	out := make([]skillcatalog.Entry, 0, len(bindings))
	for _, binding := range bindings {
		if strictExpert {
			if _, ok := allowedExpertSkills[binding.ID]; !ok {
				continue
			}
		}
		path := strings.TrimSpace(binding.Path)
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		out = append(out, skillcatalog.Entry{
			ID:          strings.TrimSpace(binding.ID),
			Description: strings.TrimSpace(binding.Description),
			Path:        path,
		})
	}
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

func availableCLIToolIDs(cliTools []CLIToolConfig) []string {
	out := make([]string, 0, len(cliTools))
	for _, item := range cliTools {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
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
