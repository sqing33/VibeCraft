package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/store"
)

func waitForOrchestrationStatus(t *testing.T, baseURL, orchestrationID, want string, timeout time.Duration) store.OrchestrationDetail {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := http.Get(baseURL + "/api/v1/orchestrations/" + orchestrationID)
		if err != nil {
			t.Fatalf("get orchestration detail: %v", err)
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			t.Fatalf("unexpected get orchestration detail status: %s", res.Status)
		}
		var detail store.OrchestrationDetail
		if err := json.NewDecoder(res.Body).Decode(&detail); err != nil {
			res.Body.Close()
			t.Fatalf("decode orchestration detail: %v", err)
		}
		res.Body.Close()
		if detail.Orchestration.Status == want {
			return detail
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for orchestration status=%s", want)
	return store.OrchestrationDetail{}
}

func TestOrchestrationLifecycle_CreateListDetailAndTick(t *testing.T) {
	env := newTestEnv(t, config.Default(), 4)
	baseURL := env.httpSrv.URL
	workspacePath := initGitRepo(t)

	body, _ := json.Marshal(map[string]any{
		"title":          "orchestrate-a",
		"goal":           "分析聊天页差异，并改进工作流页",
		"workspace_path": workspacePath,
	})
	res, err := http.Post(baseURL+"/api/v1/orchestrations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create orchestration: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create orchestration status: %s", res.Status)
	}

	var created store.OrchestrationDetail
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create orchestration response: %v", err)
	}
	if created.Orchestration.ID == "" {
		t.Fatalf("missing orchestration id")
	}
	if len(created.Rounds) != 1 {
		t.Fatalf("expected exactly one round, got %d", len(created.Rounds))
	}
	if len(created.AgentRuns) < 2 {
		t.Fatalf("expected multiple agent runs, got %d", len(created.AgentRuns))
	}

	listRes, err := http.Get(baseURL + "/api/v1/orchestrations")
	if err != nil {
		t.Fatalf("list orchestrations: %v", err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list orchestrations status: %s", listRes.Status)
	}
	var items []store.Orchestration
	if err := json.NewDecoder(listRes.Body).Decode(&items); err != nil {
		t.Fatalf("decode orchestrations list: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected non-empty orchestrations list")
	}

	for i := 0; i < 10; i++ {
		if err := env.orchMgr.Tick(context.Background()); err != nil {
			t.Fatalf("orchestration tick: %v", err)
		}
	}

	detail := waitForOrchestrationStatus(t, baseURL, created.Orchestration.ID, "waiting_continue", 5*time.Second)
	if len(detail.SynthesisSteps) != 1 {
		t.Fatalf("expected one synthesis step, got %d", len(detail.SynthesisSteps))
	}
	if detail.SynthesisSteps[0].Decision != string(store.SynthesisDecisionContinue) {
		t.Fatalf("unexpected synthesis decision: %s", detail.SynthesisSteps[0].Decision)
	}
	hasWorktree := false
	for _, run := range detail.AgentRuns {
		if run.LastExecution == nil || *run.LastExecution == "" {
			t.Fatalf("expected agent run execution id, got %+v", run)
		}
		if run.Intent == "modify" && run.WorktreePath != nil && *run.WorktreePath != "" {
			hasWorktree = true
		}
	}
	if !hasWorktree {
		t.Fatalf("expected at least one modify agent run to have worktree metadata")
	}

	continueRes, err := http.Post(baseURL+"/api/v1/orchestrations/"+created.Orchestration.ID+"/continue", "application/json", nil)
	if err != nil {
		t.Fatalf("continue orchestration: %v", err)
	}
	continueRes.Body.Close()
	if continueRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected continue orchestration status: %s", continueRes.Status)
	}

	for i := 0; i < 10; i++ {
		if err := env.orchMgr.Tick(context.Background()); err != nil {
			t.Fatalf("orchestration tick after continue: %v", err)
		}
	}

	detail = waitForOrchestrationStatus(t, baseURL, created.Orchestration.ID, "done", 5*time.Second)
	if len(detail.Rounds) < 2 {
		t.Fatalf("expected second round after continue, got %d rounds", len(detail.Rounds))
	}
}

func TestOrchestrationCancel_TransitionsToCanceled(t *testing.T) {
	env := newTestEnv(t, config.Default(), 4)
	baseURL := env.httpSrv.URL
	body, _ := json.Marshal(map[string]any{
		"title":          "orchestrate-cancel",
		"goal":           "分析一个页面并给出建议",
		"workspace_path": initGitRepo(t),
	})
	res, err := http.Post(baseURL+"/api/v1/orchestrations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create orchestration: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create orchestration status: %s", res.Status)
	}
	var created store.OrchestrationDetail
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create orchestration response: %v", err)
	}
	if err := env.orchMgr.Tick(context.Background()); err != nil {
		t.Fatalf("orchestration tick: %v", err)
	}
	cancelRes, err := http.Post(baseURL+"/api/v1/orchestrations/"+created.Orchestration.ID+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("cancel orchestration: %v", err)
	}
	defer cancelRes.Body.Close()
	if cancelRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected cancel orchestration status: %s", cancelRes.Status)
	}
	var canceled store.Orchestration
	if err := json.NewDecoder(cancelRes.Body).Decode(&canceled); err != nil {
		t.Fatalf("decode cancel orchestration response: %v", err)
	}
	if canceled.Status != string(store.OrchestrationStatusCanceled) {
		t.Fatalf("expected canceled status, got %q", canceled.Status)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := t.TempDir()
	cmds := [][]string{
		{"git", "init", repoDir},
		{"git", "-C", repoDir, "config", "user.email", "test@example.com"},
		{"git", "-C", repoDir, "config", "user.name", "Test User"},
	}
	for _, cmdArgs := range cmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("run %v: %v (%s)", cmdArgs, err, string(out))
		}
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	commit := exec.Command("git", "-C", repoDir, "add", ".")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, string(out))
	}
	commit = exec.Command("git", "-C", repoDir, "commit", "-m", "init")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (%s)", err, string(out))
	}
	return repoDir
}
