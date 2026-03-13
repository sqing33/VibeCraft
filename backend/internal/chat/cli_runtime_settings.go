package chat

import (
	"context"
	"fmt"

	"vibecraft/backend/internal/cliruntime"
	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/store"
)

func (m *Manager) prepareCLIRuntimeRunSpec(sess store.ChatSession, spec runner.RunSpec, expertID string) (runner.RunSpec, error) {
	env := cloneEnvMap(spec.Env)
	env, err := m.injectGatewayEnv(context.Background(), sess, env)
	if err != nil {
		return runner.RunSpec{}, err
	}
	spec.Env = env
	family := runner.NormalizeCLIFamily(spec.Env["VIBECRAFT_CLI_FAMILY"])
	switch family {
	case "":
		return spec, nil
	case "iflow":
		return m.prepareIFLOWRunSpec(sess, spec, expertID)
	case "codex":
		env := cloneEnvMap(spec.Env)
		baseURL := firstNonEmptyTrimmed(env["OPENAI_BASE_URL"], env["VIBECRAFT_BASE_URL"])
		toolID := firstNonEmptyTrimmed(env["VIBECRAFT_CLI_TOOL_ID"], "codex")
		homeDir, err := cliruntime.WriteCodexProviderConfig(toolID, baseURL)
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("prepare codex managed home: %w", err)
		}
		env["CODEX_HOME"] = homeDir
		spec.Env = env
		return spec, nil
	case "claude":
		env := cloneEnvMap(spec.Env)
		toolID := firstNonEmptyTrimmed(env["VIBECRAFT_CLI_TOOL_ID"], "claude")
		if payload, ok := cliruntime.ClaudeGatewayPayloadFromEnv(env); ok {
			path, err := cliruntime.WriteClaudeMCPConfigFile(toolID, payload)
			if err != nil {
				return runner.RunSpec{}, fmt.Errorf("prepare claude managed mcp config: %w", err)
			}
			env["VIBECRAFT_CLAUDE_MCP_CONFIG_PATH"] = path
		}
		settingsPath, err := cliruntime.WriteClaudeSettingsFile(toolID, map[string]any{})
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("prepare claude managed settings: %w", err)
		}
		env["VIBECRAFT_CLAUDE_SETTINGS_PATH"] = settingsPath
		spec.Env = env
		return spec, nil
	case "opencode":
		env := cloneEnvMap(spec.Env)
		toolID := firstNonEmptyTrimmed(env["VIBECRAFT_CLI_TOOL_ID"], "opencode")
		if payload, ok := cliruntime.OpenCodeGatewayPayloadFromEnv(env); ok {
			path, err := cliruntime.WriteOpenCodeGatewayConfig(toolID, payload)
			if err != nil {
				return runner.RunSpec{}, fmt.Errorf("prepare opencode managed config: %w", err)
			}
			env["VIBECRAFT_OPENCODE_CONFIG_PATH"] = path
		}
		spec.Env = env
		return spec, nil
	default:
		return spec, nil
	}
}

func (m *Manager) injectGatewayEnv(ctx context.Context, sess store.ChatSession, env map[string]string) (map[string]string, error) {
	if m == nil || m.mcpGateway == nil || !m.mcpGateway.Enabled() {
		delete(env, "VIBECRAFT_MCP_GATEWAY_NAME")
		delete(env, "VIBECRAFT_MCP_GATEWAY_URL")
		delete(env, "VIBECRAFT_MCP_GATEWAY_TOKEN")
		return env, nil
	}
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return nil, fmt.Errorf("load persisted mcp config: %w", err)
	}
	toolID := firstNonEmptyTrimmed(env["VIBECRAFT_CLI_TOOL_ID"], pointerStringValue(sess.CLIToolID))
	effectiveMCPs := config.EffectiveMCPServers(cfg, toolID, sess.MCPServerIDs)
	info, err := m.mcpGateway.EnsureSessionAccess(ctx, sess.ID, sess.WorkspacePath, sortedKeys(effectiveMCPs))
	if err != nil {
		return nil, fmt.Errorf("prepare mcp gateway access: %w", err)
	}
	if info == nil {
		delete(env, "VIBECRAFT_MCP_GATEWAY_NAME")
		delete(env, "VIBECRAFT_MCP_GATEWAY_URL")
		delete(env, "VIBECRAFT_MCP_GATEWAY_TOKEN")
		return env, nil
	}
	env["VIBECRAFT_MCP_GATEWAY_NAME"] = info.ServerID
	env["VIBECRAFT_MCP_GATEWAY_URL"] = info.URL
	env["VIBECRAFT_MCP_GATEWAY_TOKEN"] = info.Token
	return env, nil
}
