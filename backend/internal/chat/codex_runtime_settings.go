package chat

import (
	"fmt"
	"sort"
	"strings"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/skillcatalog"
	"vibe-tree/backend/internal/store"
)

type codexRuntimeSettings struct {
	BaseInstructions string
	Config           map[string]any
}

func (m *Manager) buildCodexThreadRequest(sess store.ChatSession, spec runner.RunSpec, expertID string, cliToolID *string, resumeThreadID string) (codexAppServerThreadRequest, error) {
	runtime, err := resolveCodexRuntimeSettings(sess, spec, expertID, firstNonEmptyTrimmed(pointerStringValue(cliToolID), spec.Env["VIBE_TREE_CLI_TOOL_ID"], pointerStringValue(sess.CLIToolID)))
	if err != nil {
		return codexAppServerThreadRequest{}, err
	}
	return codexAppServerThreadRequest{
		ThreadID:         resumeThreadID,
		Model:            strings.TrimSpace(spec.Env["VIBE_TREE_MODEL"]),
		Cwd:              firstNonEmptyTrimmed(strings.TrimSpace(spec.Cwd), strings.TrimSpace(sess.WorkspacePath), "."),
		BaseInstructions: runtime.BaseInstructions,
		Config:           runtime.Config,
	}, nil
}

func resolveCodexRuntimeSettings(sess store.ChatSession, spec runner.RunSpec, expertID, cliToolID string) (codexRuntimeSettings, error) {
	baseInstructions := strings.TrimSpace(spec.Env["VIBE_TREE_SYSTEM_PROMPT"])
	cliToolID = strings.TrimSpace(cliToolID)
	if cliToolID == "" {
		return codexRuntimeSettings{BaseInstructions: baseInstructions}, nil
	}
	cfg, _, err := config.LoadPersisted()
	if err != nil {
		return codexRuntimeSettings{}, fmt.Errorf("load persisted runtime config: %w", err)
	}
	effectiveMCPs := config.EffectiveMCPServers(cfg, cliToolID, sess.MCPServerIDs)
	runtimeConfig := map[string]any{"mcp_servers": map[string]any{}}
	if effectiveMCPs != nil {
		runtimeConfig["mcp_servers"] = effectiveMCPs
	}
	if sess.ReasoningEffort != nil && strings.TrimSpace(*sess.ReasoningEffort) != "" {
		runtimeConfig["model_reasoning_effort"] = strings.TrimSpace(*sess.ReasoningEffort)
	}
	effectiveSkills := config.EffectiveSkillCatalogEntries(cfg, cliToolID, expertEnabledSkillIDs(cfg, expertID), skillcatalog.Discover())
	return codexRuntimeSettings{
		BaseInstructions: appendCodexSkillInstructions(baseInstructions, effectiveSkills),
		Config:           runtimeConfig,
	}, nil
}

func expertEnabledSkillIDs(cfg config.Config, expertID string) []string {
	expertID = strings.TrimSpace(expertID)
	if expertID == "" {
		return nil
	}
	for _, item := range cfg.Experts {
		if strings.TrimSpace(item.ID) != expertID {
			continue
		}
		return append([]string(nil), item.EnabledSkills...)
	}
	return nil
}

func appendCodexSkillInstructions(base string, skills []skillcatalog.Entry) string {
	if len(skills) == 0 {
		return base
	}
	ordered := append([]skillcatalog.Entry(nil), skills...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	lines := []string{
		"[Enabled Skills]",
		"Use only the skills listed below when they are relevant.",
		"Read the referenced SKILL.md on demand before following a skill.",
		"Do not assume the contents of skills that you have not read.",
	}
	for _, item := range ordered {
		line := "- " + strings.TrimSpace(item.ID)
		if strings.TrimSpace(item.Description) != "" {
			line += " | " + strings.TrimSpace(item.Description)
		}
		if strings.TrimSpace(item.Path) != "" {
			line += " | path=" + strings.TrimSpace(item.Path)
		}
		lines = append(lines, line)
	}
	block := strings.Join(lines, "\n")
	if strings.TrimSpace(base) == "" {
		return block
	}
	return strings.TrimSpace(base) + "\n\n" + block
}
