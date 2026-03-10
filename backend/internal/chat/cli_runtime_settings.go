package chat

import (
	"fmt"

	"vibe-tree/backend/internal/cliruntime"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

func prepareCLIRuntimeRunSpec(sess store.ChatSession, spec runner.RunSpec, expertID string) (runner.RunSpec, error) {
	family := runner.NormalizeCLIFamily(spec.Env["VIBE_TREE_CLI_FAMILY"])
	switch family {
	case "":
		return spec, nil
	case "iflow":
		return prepareIFLOWRunSpec(sess, spec, expertID)
	case "codex":
		env := cloneEnvMap(spec.Env)
		baseURL := firstNonEmptyTrimmed(env["OPENAI_BASE_URL"], env["VIBE_TREE_BASE_URL"])
		toolID := firstNonEmptyTrimmed(env["VIBE_TREE_CLI_TOOL_ID"], "codex")
		homeDir, err := cliruntime.WriteCodexProviderConfig(toolID, baseURL)
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("prepare codex managed home: %w", err)
		}
		env["CODEX_HOME"] = homeDir
		spec.Env = env
		return spec, nil
	case "claude":
		env := cloneEnvMap(spec.Env)
		toolID := firstNonEmptyTrimmed(env["VIBE_TREE_CLI_TOOL_ID"], "claude")
		settingsPath, err := cliruntime.WriteClaudeSettingsFile(toolID, map[string]any{})
		if err != nil {
			return runner.RunSpec{}, fmt.Errorf("prepare claude managed settings: %w", err)
		}
		env["VIBE_TREE_CLAUDE_SETTINGS_PATH"] = settingsPath
		spec.Env = env
		return spec, nil
	default:
		return spec, nil
	}
}
