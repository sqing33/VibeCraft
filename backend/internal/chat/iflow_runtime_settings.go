package chat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vibe-tree/backend/internal/config"
	iflowcli "vibe-tree/backend/internal/iflow"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/skillcatalog"
	"vibe-tree/backend/internal/store"
)

func prepareIFLOWRunSpec(sess store.ChatSession, spec runner.RunSpec, expertID string) (runner.RunSpec, error) {
	if runner.NormalizeCLIFamily(spec.Env["VIBE_TREE_CLI_FAMILY"]) != "iflow" {
		return spec, nil
	}
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return runner.RunSpec{}, fmt.Errorf("load persisted iflow runtime config: %w", err)
	}
	toolID := firstNonEmptyTrimmed(spec.Env["VIBE_TREE_CLI_TOOL_ID"], pointerStringValue(sess.CLIToolID), "iflow")
	tool, ok := config.CLIToolByID(cfg)[toolID]
	if !ok {
		tool = config.CLIToolConfig{ID: "iflow", CLIFamily: "iflow", IFlowAuthMode: config.IFLOWAuthModeBrowser, IFlowBaseURL: iflowcli.DefaultBaseURL, IFlowModels: []string{iflowcli.DefaultModel}, IFlowDefaultModel: iflowcli.DefaultModel}
	}
	homeDir, err := iflowcli.EnsureHome()
	if err != nil {
		return runner.RunSpec{}, fmt.Errorf("prepare managed iflow home: %w", err)
	}
	effectiveSkills := config.EffectiveSkillCatalogEntries(cfg, toolID, expertEnabledSkillIDs(cfg, expertID), skillcatalog.Discover())
	effectiveMCPs := config.EffectiveMCPServers(cfg, toolID, sess.MCPServerIDs)
	env := cloneEnvMap(spec.Env)
	env["VIBE_TREE_IFLOW_HOME"] = homeDir
	env["VIBE_TREE_IFLOW_AUTH_MODE"] = firstNonEmptyTrimmed(tool.IFlowAuthMode, config.IFLOWAuthModeBrowser)
	env["VIBE_TREE_IFLOW_BASE_URL"] = firstNonEmptyTrimmed(tool.IFlowBaseURL, iflowcli.DefaultBaseURL)
	if env["VIBE_TREE_IFLOW_AUTH_MODE"] == config.IFLOWAuthModeAPIKey {
		if strings.TrimSpace(tool.IFlowAPIKey) == "" {
			return runner.RunSpec{}, fmt.Errorf("iFlow API Key 未配置，请到 Settings → CLI 工具 → iFlow CLI 填写官方 API Key，或切换到网页登录")
		}
		env["VIBE_TREE_IFLOW_API_KEY"] = strings.TrimSpace(tool.IFlowAPIKey)
	} else {
		delete(env, "VIBE_TREE_IFLOW_API_KEY")
		status, err := iflowcli.DetectBrowserAuthStatus()
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("detect iflow browser auth status: %w", err)
		}
		if !status.Authenticated {
			return runner.RunSpec{}, fmt.Errorf("iFlow 官方网页登录未完成，请到 Settings → CLI 工具 → iFlow CLI 启动网页登录，或切换到 API Key 登录")
		}
		if strings.TrimSpace(env["VIBE_TREE_MODEL"]) == "" || strings.TrimSpace(env["VIBE_TREE_MODEL"]) == iflowcli.DefaultModel {
			if strings.TrimSpace(status.ModelName) != "" {
				env["VIBE_TREE_MODEL"] = strings.TrimSpace(status.ModelName)
				env["VIBE_TREE_MODEL_ID"] = strings.TrimSpace(status.ModelName)
			}
		}
	}
	if base := strings.TrimSpace(env["VIBE_TREE_SYSTEM_PROMPT"]); base != "" || len(effectiveSkills) > 0 {
		env["VIBE_TREE_SYSTEM_PROMPT"] = appendCodexSkillInstructions(base, effectiveSkills)
	}
	if len(effectiveMCPs) > 0 {
		payload, err := json.Marshal(effectiveMCPs)
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("marshal iflow mcp settings: %w", err)
		}
		env["VIBE_TREE_IFLOW_MCP_SERVERS_JSON"] = string(payload)
		env["VIBE_TREE_IFLOW_ALLOWED_MCP_SERVERS"] = strings.Join(sortedKeys(effectiveMCPs), ",")
	} else {
		delete(env, "VIBE_TREE_IFLOW_MCP_SERVERS_JSON")
		delete(env, "VIBE_TREE_IFLOW_ALLOWED_MCP_SERVERS")
	}
	spec.Env = env
	return spec, nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, strings.TrimSpace(key))
	}
	sort.Strings(keys)
	return keys
}
