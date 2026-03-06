package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/chat"
	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/orchestration"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/scheduler"
	"vibe-tree/backend/internal/server"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

type testEnv struct {
	httpSrv *httptest.Server
	store   *store.Store
	sched   *scheduler.Scheduler
	orchMgr *orchestration.Manager
	execMgr *execution.Manager
}

func newTestEnv(t *testing.T, cfg config.Config, maxConcurrency int) *testEnv {
	t.Helper()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)
	experts := expert.NewRegistry(cfg)

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	chatMgr := chat.NewManager(st, hub, chat.Options{})
	orchMgr := orchestration.NewManager(orchestration.Options{Store: st, Executions: execMgr, Experts: experts, Hub: hub, MaxConcurrency: maxConcurrency})

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts, Chat: chatMgr, Orchestration: orchMgr})
	httpSrv := httptest.NewServer(engine)
	t.Cleanup(httpSrv.Close)

	sched := scheduler.New(scheduler.Options{Store: st, Executions: execMgr, Hub: hub, Experts: experts, MaxConcurrency: maxConcurrency})

	return &testEnv{
		httpSrv: httpSrv,
		store:   st,
		sched:   sched,
		orchMgr: orchMgr,
		execMgr: execMgr,
	}
}

func mustCreateWorkflow(t *testing.T, baseURL string, mode string) store.Workflow {
	t.Helper()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"` + mode + `"}`)
	res, err := http.Post(baseURL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create status: %s", res.Status)
	}
	var created store.Workflow
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return created
}

func waitForNodes(t *testing.T, baseURL, workflowID string, wantAtLeast int, timeout time.Duration) []store.Node {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(baseURL + "/api/v1/workflows/" + workflowID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		if len(ns) >= wantAtLeast {
			return ns
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for nodes >= %d", wantAtLeast)
	return nil
}

func getWorkflow(t *testing.T, baseURL, workflowID string) store.Workflow {
	t.Helper()

	res, err := http.Get(baseURL + "/api/v1/workflows/" + workflowID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get workflow status: %s", res.Status)
	}
	var wf store.Workflow
	if err := json.NewDecoder(res.Body).Decode(&wf); err != nil {
		t.Fatalf("decode workflow: %v", err)
	}
	return wf
}

func patchWorkflowMode(t *testing.T, baseURL, workflowID, mode string) {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"mode": mode})
	req, err := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/workflows/"+workflowID, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new patch workflow request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch workflow: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch workflow status: %s", res.Status)
	}
}

func patchNodePrompt(t *testing.T, baseURL, nodeID, prompt string) {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"prompt": prompt})
	req, err := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/nodes/"+nodeID, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new patch node request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch node: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch node status: %s", res.Status)
	}
}

func approveWorkflow(t *testing.T, baseURL, workflowID string) []store.Node {
	t.Helper()

	res, err := http.Post(baseURL+"/api/v1/workflows/"+workflowID+"/approve", "application/json", nil)
	if err != nil {
		t.Fatalf("approve workflow: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected approve status: %s", res.Status)
	}
	var out struct {
		Nodes []store.Node `json:"nodes"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode approve response: %v", err)
	}
	return out.Nodes
}

func TestManualApprove_UsesPatchedPrompt(t *testing.T) {
	env := newTestEnv(t, config.Default(), 4)
	baseURL := env.httpSrv.URL

	created := mustCreateWorkflow(t, baseURL, "manual")

	startRes, err := http.Post(baseURL+"/api/v1/workflows/"+created.ID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	nodes := waitForNodes(t, baseURL, created.ID, 4, 5*time.Second)

	var step1 store.Node
	found := false
	for _, n := range nodes {
		if n.NodeType == "master" {
			continue
		}
		if n.Title == "Step 1" {
			step1 = n
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Step 1 node to exist")
	}
	if step1.Status != "pending_approval" {
		t.Fatalf("expected Step 1 pending_approval, got %q", step1.Status)
	}

	marker := "[patched] hello"
	patchNodePrompt(t, baseURL, step1.ID, `echo "`+marker+`"; sleep 0.02; echo "[patched] done"`)

	approved := approveWorkflow(t, baseURL, created.ID)
	if len(approved) != 1 {
		t.Fatalf("expected 1 runnable node approved, got %d", len(approved))
	}
	if approved[0].ID != step1.ID {
		t.Fatalf("expected Step 1 to be approved, got node_id=%s", approved[0].ID)
	}

	if err := env.sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	// Wait for Step 1 to finish.
	deadline := time.Now().Add(5 * time.Second)
	var execID string
	for time.Now().Before(deadline) {
		ns := waitForNodes(t, baseURL, created.ID, 4, 2*time.Second)
		for _, n := range ns {
			if n.ID == step1.ID && n.Status == "succeeded" && n.LastExecution != nil {
				execID = *n.LastExecution
				goto finished
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for Step 1 to succeed")

finished:

	logRes, err := http.Get(baseURL + "/api/v1/executions/" + execID + "/log?tail=20000")
	if err != nil {
		t.Fatalf("get log tail: %v", err)
	}
	defer logRes.Body.Close()
	if logRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected log tail status: %s", logRes.Status)
	}
	b, _ := io.ReadAll(logRes.Body)
	if !strings.Contains(string(b), marker) {
		t.Fatalf("expected log tail to contain marker %q, got %q", marker, string(b))
	}
}

func TestRecoverAfterRestart_MarksRunningExecutionsFailed(t *testing.T) {
	env := newTestEnv(t, config.Default(), 1)

	ctx := context.Background()
	wf, err := env.store.CreateWorkflow(ctx, store.CreateWorkflowParams{
		Title:         "hello",
		WorkspacePath: ".",
		Mode:          "manual",
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	wf, node, err := env.store.StartWorkflowMaster(ctx, wf.ID, store.StartWorkflowMasterParams{
		ExpertID: "bash",
		Prompt:   "echo master",
	})
	if err != nil {
		t.Fatalf("start workflow master: %v", err)
	}
	if wf.Status != "running" || node.Status != "running" {
		t.Fatalf("expected workflow/node running, got wf=%q node=%q", wf.Status, node.Status)
	}

	execID := "ex_recover_test"
	logPath, err := paths.ExecutionLogPath(execID)
	if err != nil {
		t.Fatalf("execution log path: %v", err)
	}
	if err := paths.EnsureDir(filepath.Dir(logPath)); err != nil {
		t.Fatalf("ensure log dir: %v", err)
	}
	if err := os.WriteFile(logPath, []byte("start\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	_, err = env.store.StartExecution(ctx, store.StartExecutionParams{
		ExecutionID: execID,
		WorkflowID:  wf.ID,
		NodeID:      node.ID,
		Attempt:     1,
		PID:         1,
		LogPath:     logPath,
		StartedAt:   time.Now().UnixMilli(),
		Command:     "bash",
		Args:        []string{"-lc", "echo hi"},
		Cwd:         ".",
	})
	if err != nil {
		t.Fatalf("start execution: %v", err)
	}

	fixed, err := env.store.RecoverAfterRestart(ctx)
	if err != nil {
		t.Fatalf("recover after restart: %v", err)
	}
	if fixed != 1 {
		t.Fatalf("expected fixed=1, got %d", fixed)
	}

	gotWf, err := env.store.GetWorkflow(ctx, wf.ID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	if gotWf.Status != "failed" {
		t.Fatalf("expected workflow failed, got %q", gotWf.Status)
	}

	gotNode, err := env.store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if gotNode.Status != "failed" {
		t.Fatalf("expected node failed, got %q", gotNode.Status)
	}
	if gotNode.ErrorMessage == nil || *gotNode.ErrorMessage != "daemon_restarted" {
		t.Fatalf("expected node error_message daemon_restarted, got %v", gotNode.ErrorMessage)
	}

	var execStatus string
	var execErr sql.NullString
	if err := env.store.DB().QueryRowContext(ctx, `SELECT status, error_message FROM executions WHERE id = ?;`, execID).Scan(&execStatus, &execErr); err != nil {
		t.Fatalf("query execution: %v", err)
	}
	if execStatus != "failed" {
		t.Fatalf("expected execution failed, got %q", execStatus)
	}
	if !execErr.Valid || execErr.String != "daemon_restarted" {
		t.Fatalf("expected execution error_message daemon_restarted, got %v", execErr)
	}

	logRes, err := http.Get(env.httpSrv.URL + "/api/v1/executions/" + execID + "/log?tail=20000")
	if err != nil {
		t.Fatalf("get log tail: %v", err)
	}
	defer logRes.Body.Close()
	if logRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected log tail status: %s", logRes.Status)
	}
}

func TestAutoMode_DependencyOrderAndMaxConcurrency(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	created := mustCreateWorkflow(t, baseURL, "auto")

	prompt := `
cat <<'JSON'
{
  "workflow_title": "",
  "nodes": [
    { "id": "a", "title": "Alpha", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[a] start'; sleep 0.4; echo '[a] end'" },
    { "id": "b", "title": "Beta", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[b] start'; sleep 0.2; echo '[b] end'" }
  ],
  "edges": [
    { "from": "a", "to": "b", "type": "success", "source_handle": null, "target_handle": null }
  ]
}
JSON
`
	startBody, _ := json.Marshal(map[string]string{
		"expert_id": "bash",
		"prompt":    prompt,
	})
	startRes, err := http.Post(baseURL+"/api/v1/workflows/"+created.ID+"/start", "application/json", bytes.NewReader(startBody))
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	nodes := waitForNodes(t, baseURL, created.ID, 3, 5*time.Second)
	var alphaID, betaID string
	for _, n := range nodes {
		if n.NodeType == "master" {
			continue
		}
		switch n.Title {
		case "Alpha":
			alphaID = n.ID
		case "Beta":
			betaID = n.ID
		}
	}
	if alphaID == "" || betaID == "" {
		t.Fatalf("expected Alpha/Beta nodes to exist")
	}

	if err := env.sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	// While Alpha is running, Beta must remain queued (dependency not satisfied).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ns := waitForNodes(t, baseURL, created.ID, 3, 2*time.Second)
		var alphaStatus, betaStatus string
		for _, n := range ns {
			if n.ID == alphaID {
				alphaStatus = n.Status
			}
			if n.ID == betaID {
				betaStatus = n.Status
			}
		}
		if alphaStatus == "running" {
			if betaStatus != "queued" {
				t.Fatalf("expected Beta queued while Alpha running, got %q", betaStatus)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for Alpha succeeded.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ns := waitForNodes(t, baseURL, created.ID, 3, 2*time.Second)
		for _, n := range ns {
			if n.ID == alphaID && n.Status == "succeeded" {
				goto alphaDone
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for Alpha succeeded")

alphaDone:

	if err := env.sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick after Alpha: %v", err)
	}

	// Beta should run and finish.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ns := waitForNodes(t, baseURL, created.ID, 3, 2*time.Second)
		for _, n := range ns {
			if n.ID == betaID && n.Status == "succeeded" {
				goto betaDone
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for Beta succeeded")

betaDone:

	wf := getWorkflow(t, baseURL, created.ID)
	if wf.Status != "done" {
		t.Fatalf("expected workflow done, got %q", wf.Status)
	}
}

func TestModeSwitch_InterceptsQueuedNodes(t *testing.T) {
	env := newTestEnv(t, config.Default(), 1)
	baseURL := env.httpSrv.URL

	created := mustCreateWorkflow(t, baseURL, "auto")

	prompt := `
cat <<'JSON'
{
  "workflow_title": "",
  "nodes": [
    { "id": "a", "title": "Alpha", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[a] start'; sleep 0.6; echo '[a] end'" },
    { "id": "b", "title": "Beta", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[b] start'; sleep 0.6; echo '[b] end'" }
  ],
  "edges": []
}
JSON
`
	startBody, _ := json.Marshal(map[string]string{
		"expert_id": "bash",
		"prompt":    prompt,
	})
	startRes, err := http.Post(baseURL+"/api/v1/workflows/"+created.ID+"/start", "application/json", bytes.NewReader(startBody))
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	nodes := waitForNodes(t, baseURL, created.ID, 3, 5*time.Second)
	var alphaID, betaID string
	for _, n := range nodes {
		if n.NodeType == "master" {
			continue
		}
		switch n.Title {
		case "Alpha":
			alphaID = n.ID
		case "Beta":
			betaID = n.ID
		}
	}
	if alphaID == "" || betaID == "" {
		t.Fatalf("expected Alpha/Beta nodes to exist")
	}

	if err := env.sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	// Switch to manual while one node is running.
	patchWorkflowMode(t, baseURL, created.ID, "manual")

	// Expect: one running, one pending_approval.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ns := waitForNodes(t, baseURL, created.ID, 3, 2*time.Second)
		var running, pending int
		for _, n := range ns {
			if n.NodeType == "master" {
				continue
			}
			if n.Status == "running" {
				running++
			}
			if n.Status == "pending_approval" {
				pending++
			}
		}
		if running == 1 && pending == 1 {
			goto intercepted
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for queued node to become pending_approval after mode switch")

intercepted:

	// Wait for running node to finish.
	deadline = time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		ns := waitForNodes(t, baseURL, created.ID, 3, 2*time.Second)
		succeeded := 0
		pending := 0
		for _, n := range ns {
			if n.NodeType == "master" {
				continue
			}
			if n.Status == "succeeded" {
				succeeded++
			}
			if n.Status == "pending_approval" {
				pending++
			}
		}
		if succeeded == 1 && pending == 1 {
			goto oneDone
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for first node to finish")

oneDone:

	// Scheduler tick should NOT start pending_approval node in manual mode.
	if err := env.sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick in manual: %v", err)
	}
	running, err := env.store.CountRunningWorkerNodes(context.Background())
	if err != nil {
		t.Fatalf("count running nodes: %v", err)
	}
	if running != 0 {
		t.Fatalf("expected running=0 in manual mode, got %d", running)
	}

	// Switch back to auto and run the remaining node.
	patchWorkflowMode(t, baseURL, created.ID, "auto")
	if err := env.sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick after switch back to auto: %v", err)
	}

	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		wf := getWorkflow(t, baseURL, created.ID)
		if wf.Status == "done" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for workflow to be done")
}

func TestAutoWorkflow_TenNodeDAG_Completes(t *testing.T) {
	env := newTestEnv(t, config.Default(), 3)
	baseURL := env.httpSrv.URL

	created := mustCreateWorkflow(t, baseURL, "auto")

	prompt := `
cat <<'JSON'
{
  "workflow_title": "",
  "nodes": [
    { "id": "n1", "title": "n1", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n1]'; sleep 0.02; echo '[n1] done'" },
    { "id": "n2", "title": "n2", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n2]'; sleep 0.02; echo '[n2] done'" },
    { "id": "n3", "title": "n3", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n3]'; sleep 0.02; echo '[n3] done'" },
    { "id": "n4", "title": "n4", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n4]'; sleep 0.02; echo '[n4] done'" },
    { "id": "n5", "title": "n5", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n5]'; sleep 0.02; echo '[n5] done'" },
    { "id": "n6", "title": "n6", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n6]'; sleep 0.02; echo '[n6] done'" },
    { "id": "n7", "title": "n7", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n7]'; sleep 0.02; echo '[n7] done'" },
    { "id": "n8", "title": "n8", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n8]'; sleep 0.02; echo '[n8] done'" },
    { "id": "n9", "title": "n9", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n9]'; sleep 0.02; echo '[n9] done'" },
    { "id": "n10", "title": "n10", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[n10]'; sleep 0.02; echo '[n10] done'" }
  ],
  "edges": [
    { "from": "n1", "to": "n2", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n1", "to": "n3", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n2", "to": "n4", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n3", "to": "n4", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n4", "to": "n5", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n4", "to": "n6", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n5", "to": "n7", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n6", "to": "n7", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n7", "to": "n8", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n8", "to": "n9", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n9", "to": "n10", "type": "success", "source_handle": null, "target_handle": null }
  ]
}
JSON
`
	startBody, _ := json.Marshal(map[string]string{
		"expert_id": "bash",
		"prompt":    prompt,
	})
	startRes, err := http.Post(baseURL+"/api/v1/workflows/"+created.ID+"/start", "application/json", bytes.NewReader(startBody))
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	_ = waitForNodes(t, baseURL, created.ID, 11, 5*time.Second)

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		_ = env.sched.Tick(context.Background())
		wf := getWorkflow(t, baseURL, created.ID)
		if wf.Status == "done" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for 10-node workflow to be done")
}

func TestWebSocketReconnect_CanContinueReceivingLogsAndCatchUpByTail(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}

	startReq, _ := json.Marshal(map[string]any{
		"command": "bash",
		"args": []string{
			"-lc",
			`echo "start"; for i in {1..200}; do echo "line:$i"; sleep 0.01; done; echo "end"`,
		},
	})
	res, err := http.Post(baseURL+"/api/v1/executions", "application/json", bytes.NewReader(startReq))
	if err != nil {
		t.Fatalf("start execution: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start execution status: %s", res.Status)
	}
	var exec execution.Execution
	if err := json.NewDecoder(res.Body).Decode(&exec); err != nil {
		t.Fatalf("decode start execution: %v", err)
	}

	// Read some logs, then disconnect.
	seenBefore := false
	readDeadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(readDeadline)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if strings.Contains(string(msg), exec.ID) && strings.Contains(string(msg), `"type":"node.log"`) {
			seenBefore = true
			break
		}
	}
	_ = conn.Close()
	if !seenBefore {
		t.Fatalf("expected to see node.log before disconnect")
	}

	// Reconnect and ensure we can still receive logs.
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws 2: %v", err)
	}
	defer conn2.Close()

	seenAfter := false
	readDeadline = time.Now().Add(3 * time.Second)
	_ = conn2.SetReadDeadline(readDeadline)
	for {
		_, msg, err := conn2.ReadMessage()
		if err != nil {
			break
		}
		if strings.Contains(string(msg), exec.ID) && strings.Contains(string(msg), `"type":"node.log"`) {
			seenAfter = true
			break
		}
	}
	if !seenAfter {
		t.Fatalf("expected to see node.log after reconnect")
	}

	// Catch-up: wait until "end" appears in log tail.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		logRes, err := http.Get(baseURL + "/api/v1/executions/" + exec.ID + "/log?tail=200000")
		if err != nil {
			t.Fatalf("get log tail: %v", err)
		}
		b, _ := io.ReadAll(logRes.Body)
		logRes.Body.Close()
		text := string(b)
		if strings.Contains(text, "end") {
			if !strings.Contains(text, "line:1") || !strings.Contains(text, "line:200") {
				t.Fatalf("expected log tail to contain first/last lines, got tail=%q", text)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for execution log tail to contain end")
}
