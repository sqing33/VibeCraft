package tmuxorch

import "path/filepath"

// ToolRootForRepo returns the artifact root directory for tmux orchestrator runs.
func ToolRootForRepo(repoRoot string) string {
	return filepath.Join(repoRoot, ".codex", "tools", "tmux-orch")
}

func StateFileForRepo(repoRoot, runID string) string {
	return filepath.Join(ToolRootForRepo(repoRoot), "state", runID+".json")
}
