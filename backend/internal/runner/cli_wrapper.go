package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func NormalizeCLIFamily(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "claude", "claudecode":
		return "claude"
	case "codex", "openai":
		return "codex"
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

func CLIScriptPath(family string) (string, error) {
	family = NormalizeCLIFamily(family)
	if family == "" {
		return "", fmt.Errorf("cli family is required")
	}
	if envPath := strings.TrimSpace(os.Getenv("VIBECRAFT_AGENT_RUNTIME_" + strings.ToUpper(family) + "_SCRIPT")); envPath != "" {
		return envPath, nil
	}
	name := family + "_exec.sh"
	candidates := make([]string, 0, 8)
	if root := strings.TrimSpace(os.Getenv("VIBECRAFT_AGENT_RUNTIMES_DIR")); root != "" {
		candidates = append(candidates, filepath.Join(root, name))
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
			candidates = append(candidates, filepath.Join(dir, "scripts", "agent-runtimes", name))
			next := filepath.Dir(dir)
			if next == dir {
				break
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		for dir := filepath.Dir(exe); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
			candidates = append(candidates, filepath.Join(dir, "scripts", "agent-runtimes", name))
			next := filepath.Dir(dir)
			if next == dir {
				break
			}
		}
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cli runtime script not found for family %q", family)
}
