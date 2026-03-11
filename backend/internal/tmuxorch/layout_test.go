package tmuxorch

import (
	"path/filepath"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	repo := "/tmp/repo"
	root := ToolRootForRepo(repo)
	if got, want := root, filepath.Join(repo, ".codex", "tools", "tmux-orch"); got != want {
		t.Fatalf("ToolRootForRepo=%q want %q", got, want)
	}
	if got, want := StateFileForRepo(repo, "r1"), filepath.Join(root, "state", "r1.json"); got != want {
		t.Fatalf("StateFileForRepo=%q want %q", got, want)
	}
}
