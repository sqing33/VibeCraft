package tmuxorch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDraftStatusClose_Smoke(t *testing.T) {
	tmp := t.TempDir()

	// Initialize a minimal git repo with one commit so HEAD is not unborn.
	mustRun(t, tmp, "git", "init")
	mustRun(t, tmp, "git", "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "init")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	orch := New()
	draft, err := orch.Draft(DraftRequest{Goal: "只分析: 检查项目结构", ExecutionKind: "analyze", Mode: "auto"})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if draft.RunID == "" {
		t.Fatalf("missing run_id")
	}

	st, err := orch.Status(StatusRequest{RunID: draft.RunID})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.RunID != draft.RunID {
		t.Fatalf("status run_id=%q want %q", st.RunID, draft.RunID)
	}

	closeRes, err := orch.Close(CloseRequest{RunID: draft.RunID})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closeRes.RunID != draft.RunID {
		t.Fatalf("close run_id=%q want %q", closeRes.RunID, draft.RunID)
	}

	statePath := StateFileForRepo(tmp, draft.RunID)
	b, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var parsed RunState
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if parsed.RunID != draft.RunID {
		t.Fatalf("state run_id=%q want %q", parsed.RunID, draft.RunID)
	}

	planPath := filepath.Join(ToolRootForRepo(tmp), "plans", "ORCH_PLAN.md")
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("expected plan file: %v", err)
	}
}

func mustRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := execCommand(dir, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd=%v err=%v out=%s", args, err, string(out))
	}
}

// Wrapped for tests (avoid importing os/exec in every file).
func execCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	return cmd
}
