package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/store"
)

func TestPrepare_ReusesAndRecoversGitWorktree(t *testing.T) {
	repoRoot := initGitRepoForWorkspaceTest(t)
	run := store.AgentRun{
		OrchestrationID: "or_test_prepare",
		ID:              "ar_test_prepare",
		Intent:          "modify",
		WorkspaceMode:   "git_worktree",
		WorkspacePath:   repoRoot,
	}

	prepared, err := Prepare(context.Background(), run)
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	if prepared.Mode != "git_worktree" || prepared.WorktreePath == nil || prepared.BranchName == nil {
		t.Fatalf("expected git_worktree allocation, got %+v", prepared)
	}
	cleanupPreparedWorkspace(t, repoRoot, prepared)

	reused, err := Prepare(context.Background(), run)
	if err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if reused.Mode != "git_worktree" {
		t.Fatalf("expected reused git_worktree, got %+v", reused)
	}
	if reused.WorktreePath == nil || *reused.WorktreePath != *prepared.WorktreePath {
		t.Fatalf("expected same worktree path on reuse, first=%+v second=%+v", prepared, reused)
	}

	if err := os.RemoveAll(*reused.WorktreePath); err != nil {
		t.Fatalf("remove worktree path: %v", err)
	}
	if _, err := Prepare(context.Background(), run); err != nil {
		t.Fatalf("prepare after stale worktree removal: %v", err)
	}
	if afterStale, err := Prepare(context.Background(), run); err != nil {
		t.Fatalf("prepare after stale recovery: %v", err)
	} else if afterStale.Mode != "git_worktree" || afterStale.WorktreePath == nil || *afterStale.WorktreePath != *prepared.WorktreePath {
		t.Fatalf("expected stale worktree recovery, got %+v", afterStale)
	}
}

func initGitRepoForWorkspaceTest(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init")
	runGit(t, repoRoot, "config", "user.email", "test@example.com")
	runGit(t, repoRoot, "config", "user.name", "Workspace Test")
	filePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-m", "init")
	return repoRoot
}

func cleanupPreparedWorkspace(t *testing.T, repoRoot string, prepared PreparedWorkspace) {
	t.Helper()
	if prepared.WorktreePath == nil || prepared.BranchName == nil {
		return
	}
	t.Cleanup(func() {
		runGitAllowFailure(repoRoot, "worktree", "remove", "--force", *prepared.WorktreePath)
		runGitAllowFailure(repoRoot, "worktree", "prune")
		runGitAllowFailure(repoRoot, "branch", "-D", *prepared.BranchName)
	})
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, string(out))
	}
}

func runGitAllowFailure(cwd string, args ...string) {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	_, _ = cmd.CombinedOutput()
}
