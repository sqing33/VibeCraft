package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vibecraft/backend/internal/config"
	iflowcli "vibecraft/backend/internal/iflow"
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/skillcatalog"
	"vibecraft/backend/internal/store"
)

func prepareIFLOWRunSpec(sess store.ChatSession, spec runner.RunSpec, expertID string) (runner.RunSpec, error) {
	return (*Manager)(nil).prepareIFLOWRunSpec(sess, spec, expertID)
}

func (m *Manager) prepareIFLOWRunSpec(sess store.ChatSession, spec runner.RunSpec, expertID string) (runner.RunSpec, error) {
	if runner.NormalizeCLIFamily(spec.Env["VIBECRAFT_CLI_FAMILY"]) != "iflow" {
		return spec, nil
	}
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return runner.RunSpec{}, fmt.Errorf("load persisted iflow runtime config: %w", err)
	}
	toolID := firstNonEmptyTrimmed(spec.Env["VIBECRAFT_CLI_TOOL_ID"], pointerStringValue(sess.CLIToolID), "iflow")
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
	env["VIBECRAFT_IFLOW_HOME"] = homeDir
	env["VIBECRAFT_IFLOW_AUTH_MODE"] = firstNonEmptyTrimmed(env["VIBECRAFT_IFLOW_AUTH_MODE"], tool.IFlowAuthMode, config.IFLOWAuthModeBrowser)
	env["VIBECRAFT_IFLOW_BASE_URL"] = firstNonEmptyTrimmed(env["VIBECRAFT_IFLOW_BASE_URL"], tool.IFlowBaseURL, iflowcli.DefaultBaseURL)
	if env["VIBECRAFT_IFLOW_AUTH_MODE"] == config.IFLOWAuthModeAPIKey {
		env["VIBECRAFT_IFLOW_API_KEY"] = firstNonEmptyTrimmed(env["VIBECRAFT_IFLOW_API_KEY"], tool.IFlowAPIKey)
		if strings.TrimSpace(env["VIBECRAFT_IFLOW_API_KEY"]) == "" {
			return runner.RunSpec{}, fmt.Errorf("iFlow API Key 未配置，请到 Settings → API 来源 中填写 iFlow API Key，或切换到网页登录")
		}
	} else {
		delete(env, "VIBECRAFT_IFLOW_API_KEY")
		status, err := iflowcli.DetectBrowserAuthStatus()
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("detect iflow browser auth status: %w", err)
		}
		if !status.Authenticated {
			return runner.RunSpec{}, fmt.Errorf("iFlow 官方网页登录未完成，请到 Settings → CLI 工具 → iFlow CLI 启动网页登录，或切换到 API Key 登录")
		}
		if strings.TrimSpace(env["VIBECRAFT_MODEL"]) == "" || strings.TrimSpace(env["VIBECRAFT_MODEL"]) == iflowcli.DefaultModel {
			if strings.TrimSpace(status.ModelName) != "" {
				env["VIBECRAFT_MODEL"] = strings.TrimSpace(status.ModelName)
				env["VIBECRAFT_MODEL_ID"] = strings.ToLower(strings.TrimSpace(status.ModelName))
			}
		}
	}
	if base := strings.TrimSpace(env["VIBECRAFT_SYSTEM_PROMPT"]); base != "" || len(effectiveSkills) > 0 {
		env["VIBECRAFT_SYSTEM_PROMPT"] = appendCodexSkillInstructions(base, effectiveSkills)
	}
	if m != nil && m.mcpGateway != nil {
		info, err := m.mcpGateway.EnsureSessionAccess(context.Background(), sess.ID, sess.WorkspacePath, sortedKeys(effectiveMCPs))
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("prepare iflow gateway access: %w", err)
		}
		if info != nil {
			payload, err := json.Marshal(map[string]any{
				info.ServerID: map[string]any{
					"type":    "http",
					"httpUrl": info.URL,
					"headers": info.Headers,
				},
			})
			if err != nil {
				return runner.RunSpec{}, fmt.Errorf("marshal iflow gateway settings: %w", err)
			}
			env["VIBECRAFT_IFLOW_MCP_SERVERS_JSON"] = string(payload)
			env["VIBECRAFT_IFLOW_ALLOWED_MCP_SERVERS"] = info.ServerID
			spec.Env = env
			return spec, nil
		}
	}
	if len(effectiveMCPs) > 0 {
		payload, err := json.Marshal(effectiveMCPs)
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("marshal iflow mcp settings: %w", err)
		}
		env["VIBECRAFT_IFLOW_MCP_SERVERS_JSON"] = string(payload)
		env["VIBECRAFT_IFLOW_ALLOWED_MCP_SERVERS"] = strings.Join(sortedKeys(effectiveMCPs), ",")
	} else {
		delete(env, "VIBECRAFT_IFLOW_MCP_SERVERS_JSON")
		delete(env, "VIBECRAFT_IFLOW_ALLOWED_MCP_SERVERS")
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
