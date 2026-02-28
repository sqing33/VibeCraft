package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/scheduler"
	"vibe-tree/backend/internal/server"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

type mockSDKRunner struct{}

func (r mockSDKRunner) StartOneshot(ctx context.Context, spec runner.RunSpec) (runner.ProcessHandle, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithCancel(ctx)
	pr, pw := io.Pipe()

	h := &mockPipeHandle{
		ctx:       runCtx,
		cancel:    cancel,
		outR:      pr,
		outW:      pw,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}

	go h.run(func() error {
		// 输出一个稳定的 DAG，保证后续 worker 节点（bash）链路可测试。
		_, err := io.WriteString(pw, `{
  "workflow_title": "",
  "nodes": [
    {
      "id": "n1",
      "title": "Step 1",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock sdk runner",
      "prompt": "echo '[n1] hello'; sleep 0.02; echo '[n1] done'"
    },
    {
      "id": "n2",
      "title": "Step 2",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock sdk runner",
      "prompt": "echo '[n2] hello'; sleep 0.02; echo '[n2] done'"
    },
    {
      "id": "n3",
      "title": "Step 3",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock sdk runner",
      "prompt": "echo '[n3] hello'; sleep 0.02; echo '[n3] done'"
    }
  ],
  "edges": [
    { "from": "n1", "to": "n2", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n2", "to": "n3", "type": "success", "source_handle": null, "target_handle": null }
  ]
}
`)
		return err
	})

	return h, nil
}

type mockPipeHandle struct {
	ctx    context.Context
	cancel context.CancelFunc

	outR *io.PipeReader
	outW *io.PipeWriter

	startedAt time.Time

	done chan struct{}

	finishOnce sync.Once
	mu         sync.Mutex
	exitRes    runner.ExitResult
	waitErr    error
}

func (h *mockPipeHandle) PID() int { return 0 }

func (h *mockPipeHandle) Output() io.ReadCloser { return h.outR }

func (h *mockPipeHandle) Wait() (runner.ExitResult, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitRes, h.waitErr
}

func (h *mockPipeHandle) Cancel(_ time.Duration) error {
	if h.cancel != nil {
		h.cancel()
	}
	h.finish(0, context.Canceled)
	return nil
}

func (h *mockPipeHandle) WriteInput(_ []byte) (int, error) { return 0, io.ErrClosedPipe }

func (h *mockPipeHandle) Close() error {
	if h.cancel != nil {
		h.cancel()
	}
	h.finish(0, nil)
	if h.outR != nil {
		_ = h.outR.Close()
	}
	return nil
}

func (h *mockPipeHandle) run(fn func() error) {
	err := fn()
	exitCode := 0
	if err != nil {
		exitCode = 1
	}
	h.finish(exitCode, err)
}

func (h *mockPipeHandle) finish(exitCode int, err error) {
	h.finishOnce.Do(func() {
		if h.outW != nil {
			_ = h.outW.CloseWithError(err)
		}

		h.mu.Lock()
		h.exitRes = runner.ExitResult{
			ExitCode:  exitCode,
			Signal:    "",
			StartedAt: h.startedAt,
			EndedAt:   time.Now(),
		}
		h.waitErr = err
		h.mu.Unlock()

		close(h.done)
	})
}

func newTestExecMgr(grace time.Duration, hub *ws.Hub) *execution.Manager {
	execRunner := runner.MultiRunner{
		Process: runner.PTYRunner{DefaultGrace: grace},
		SDK:     mockSDKRunner{},
	}
	return execution.NewManager(execRunner, grace, hub)
}

func TestWorkflowCRUD(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)
	experts := expert.NewRegistry(config.Default())

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"manual"}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
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
	if created.ID == "" {
		t.Fatalf("missing workflow_id in create response")
	}

	listRes, err := http.Get(httpSrv.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %s", listRes.Status)
	}

	var listed []store.Workflow
	if err := json.NewDecoder(listRes.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) == 0 || listed[0].ID != created.ID {
		t.Fatalf("expected created workflow to be in list")
	}

	patchReq := []byte(`{"title":"hello2"}`)
	req, err := http.NewRequest(http.MethodPatch, httpSrv.URL+"/api/v1/workflows/"+created.ID, bytes.NewReader(patchReq))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	patchRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch workflow: %v", err)
	}
	defer patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch status: %s", patchRes.Status)
	}

	var patched store.Workflow
	if err := json.NewDecoder(patchRes.Body).Decode(&patched); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if patched.Title != "hello2" {
		t.Fatalf("expected patched title, got %q", patched.Title)
	}

	getRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get status: %s", getRes.Status)
	}

	var got store.Workflow
	if err := json.NewDecoder(getRes.Body).Decode(&got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.Title != "hello2" {
		t.Fatalf("expected title hello2, got %q", got.Title)
	}
}

func TestStartWorkflowCreatesMasterExecution(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)
	experts := expert.NewRegistry(config.Default())

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"manual"}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
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

	startRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	defer startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	var started struct {
		Workflow  store.Workflow      `json:"workflow"`
		Node      store.Node          `json:"master_node"`
		Execution execution.Execution `json:"execution"`
	}
	if err := json.NewDecoder(startRes.Body).Decode(&started); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if started.Workflow.ID != created.ID {
		t.Fatalf("expected workflow_id %q, got %q", created.ID, started.Workflow.ID)
	}
	if started.Node.ID == "" || started.Execution.ID == "" {
		t.Fatalf("missing node/execution id")
	}

	type envelope struct {
		Type        string `json:"type"`
		ExecutionID string `json:"execution_id"`
	}
	deadline := time.Now().Add(5 * time.Second)
	_ = conn.SetReadDeadline(deadline)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		var env envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			continue
		}
		if env.ExecutionID == started.Execution.ID && env.Type == "execution.exited" {
			break
		}
	}

	getWfRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	defer getWfRes.Body.Close()
	if getWfRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get workflow status: %s", getWfRes.Status)
	}
	var gotWf store.Workflow
	if err := json.NewDecoder(getWfRes.Body).Decode(&gotWf); err != nil {
		t.Fatalf("decode workflow: %v", err)
	}
	if gotWf.Status != "running" {
		t.Fatalf("expected workflow running after master exit, got %q", gotWf.Status)
	}

	deadline = time.Now().Add(3 * time.Second)
	var nodes []store.Node
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		if nodesRes.StatusCode != http.StatusOK {
			nodesRes.Body.Close()
			t.Fatalf("unexpected nodes status: %s", nodesRes.Status)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()

		if len(ns) >= 4 {
			nodes = ns
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(nodes) < 4 {
		t.Fatalf("expected dag nodes to be created, got %d", len(nodes))
	}

	var master *store.Node
	workers := make([]store.Node, 0)
	for i := range nodes {
		if nodes[i].NodeType == "master" {
			master = &nodes[i]
			continue
		}
		workers = append(workers, nodes[i])
	}
	if master == nil {
		t.Fatalf("expected master node to exist")
	}
	if master.LastExecution == nil || *master.LastExecution != started.Execution.ID {
		t.Fatalf("expected master last_execution_id to be %q", started.Execution.ID)
	}
	if len(workers) == 0 {
		t.Fatalf("expected worker nodes to exist")
	}
	for _, n := range workers {
		if n.Status != "pending_approval" {
			t.Fatalf("expected worker node pending_approval, got %q (node_id=%s)", n.Status, n.ID)
		}
		if n.LastExecution != nil {
			t.Fatalf("expected worker node without execution, got last_execution_id=%s", *n.LastExecution)
		}
	}
}

func TestStartWorkflow_UsesConfiguredExpertWhenProvided(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)

	experts := expert.NewRegistry(config.Config{
		Experts: []config.ExpertConfig{
			{ID: "bash", RunMode: "oneshot", Command: "bash", Args: []string{"-lc", "{{prompt}}"}, Env: map[string]string{}, TimeoutMs: 30 * 60 * 1000},
			{ID: "planner", RunMode: "oneshot", Command: "bash", Args: []string{"-lc", "{{prompt}}"}, Env: map[string]string{}, TimeoutMs: 30 * 60 * 1000},
		},
	})

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"manual"}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
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

	prompt := `
cat <<'JSON'
{
  "workflow_title": "",
  "nodes": [
    { "id": "a", "title": "Alpha", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[a]'" },
    { "id": "b", "title": "Beta", "type": "worker", "expert_id": "bash", "fallback_expert_id": "bash", "complexity": "low", "quality_tier": "fast", "model": null, "routing_reason": "", "prompt": "echo '[b]'" }
  ],
  "edges": [
    { "from": "a", "to": "b", "type": "success", "source_handle": null, "target_handle": null }
  ]
}
JSON
`
	startBody, _ := json.Marshal(map[string]string{
		"expert_id": "planner",
		"prompt":    prompt,
	})
	startRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/start", "application/json", bytes.NewReader(startBody))
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	defer startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	// Wait for DAG to be applied (master exits and creates worker nodes).
	deadline := time.Now().Add(5 * time.Second)
	var nodes []store.Node
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		if len(ns) >= 3 {
			nodes = ns
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (1 master + 2 workers), got %d", len(nodes))
	}

	workers := make([]store.Node, 0)
	for i := range nodes {
		if nodes[i].NodeType != "master" {
			workers = append(workers, nodes[i])
		}
	}
	if len(workers) != 2 {
		t.Fatalf("expected 2 worker nodes, got %d", len(workers))
	}
	for _, n := range workers {
		if n.Status != "pending_approval" {
			t.Fatalf("expected worker node pending_approval, got %q (node_id=%s)", n.Status, n.ID)
		}
		if n.Title != "Alpha" && n.Title != "Beta" {
			t.Fatalf("expected worker title Alpha/Beta, got %q (node_id=%s)", n.Title, n.ID)
		}
	}
}

func TestManualApproveRunsWorkerNodes(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)
	experts := expert.NewRegistry(config.Default())
	sched := scheduler.New(scheduler.Options{Store: nil, Executions: execMgr, Hub: hub, Experts: experts, MaxConcurrency: 4})

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	sched = scheduler.New(scheduler.Options{Store: st, Executions: execMgr, Hub: hub, Experts: experts, MaxConcurrency: 4})

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"manual"}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
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

	startRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	defer startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	// Wait for DAG to be applied (master exits and creates worker nodes).
	waitDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(waitDeadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		if len(ns) >= 4 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	for step := 0; step < 3; step++ {
		approveRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/approve", "application/json", nil)
		if err != nil {
			t.Fatalf("approve workflow: %v", err)
		}
		if approveRes.StatusCode != http.StatusOK {
			approveRes.Body.Close()
			t.Fatalf("unexpected approve status: %s", approveRes.Status)
		}
		var approved struct {
			Nodes []store.Node `json:"nodes"`
		}
		if err := json.NewDecoder(approveRes.Body).Decode(&approved); err != nil {
			approveRes.Body.Close()
			t.Fatalf("decode approve response: %v", err)
		}
		approveRes.Body.Close()

		if len(approved.Nodes) != 1 {
			t.Fatalf("expected 1 runnable node to be approved, got %d", len(approved.Nodes))
		}
		nodeID := approved.Nodes[0].ID

		if err := sched.Tick(context.Background()); err != nil {
			t.Fatalf("scheduler tick: %v", err)
		}

		// Wait for this node to finish.
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
			if err != nil {
				t.Fatalf("list nodes: %v", err)
			}
			var ns []store.Node
			if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
				nodesRes.Body.Close()
				t.Fatalf("decode nodes: %v", err)
			}
			nodesRes.Body.Close()
			for _, n := range ns {
				if n.ID == nodeID && n.Status == "succeeded" {
					goto nextStep
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for node %s to succeed", nodeID)

	nextStep:
	}

	// All worker nodes succeeded => workflow done.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		getWfRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID)
		if err != nil {
			t.Fatalf("get workflow: %v", err)
		}
		var wf store.Workflow
		if err := json.NewDecoder(getWfRes.Body).Decode(&wf); err != nil {
			getWfRes.Body.Close()
			t.Fatalf("decode workflow: %v", err)
		}
		getWfRes.Body.Close()
		if wf.Status == "done" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for workflow to be done")
}

func TestNodeRetry_UnskipsAndAllowsWorkflowToFinish(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)
	experts := expert.NewRegistry(config.Default())

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sched := scheduler.New(scheduler.Options{Store: st, Executions: execMgr, Hub: hub, Experts: experts, MaxConcurrency: 4})
	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"manual"}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
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

	startRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	// Wait for DAG to be applied (master exits and creates worker nodes).
	waitDeadline := time.Now().Add(5 * time.Second)
	var nodes []store.Node
	for time.Now().Before(waitDeadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		if len(ns) >= 4 {
			nodes = ns
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(nodes) < 4 {
		t.Fatalf("expected dag nodes to be created, got %d", len(nodes))
	}

	workers := make([]store.Node, 0)
	for _, n := range nodes {
		if n.NodeType != "master" {
			workers = append(workers, n)
		}
	}
	if len(workers) != 3 {
		t.Fatalf("expected 3 worker nodes, got %d", len(workers))
	}

	approveRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/approve", "application/json", nil)
	if err != nil {
		t.Fatalf("approve workflow: %v", err)
	}
	if approveRes.StatusCode != http.StatusOK {
		approveRes.Body.Close()
		t.Fatalf("unexpected approve status: %s", approveRes.Status)
	}
	var approved struct {
		Nodes []store.Node `json:"nodes"`
	}
	if err := json.NewDecoder(approveRes.Body).Decode(&approved); err != nil {
		approveRes.Body.Close()
		t.Fatalf("decode approve response: %v", err)
	}
	approveRes.Body.Close()
	if len(approved.Nodes) != 1 {
		t.Fatalf("expected 1 runnable node to be approved, got %d", len(approved.Nodes))
	}
	target := approved.Nodes[0]

	// Make the approved (runnable) worker fail.
	patchBody, _ := json.Marshal(map[string]string{"prompt": `echo "[n1] failing"; exit 1`})
	patchReq, err := http.NewRequest(http.MethodPatch, httpSrv.URL+"/api/v1/nodes/"+target.ID, bytes.NewReader(patchBody))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("patch node: %v", err)
	}
	patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch status: %s", patchRes.Status)
	}

	if err := sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	// Wait for failure and fail-fast skip.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()

		skipped := 0
		failed := false
		for _, n := range ns {
			if n.ID == target.ID && n.Status == "failed" {
				failed = true
			}
			if n.NodeType != "master" && n.Status == "skipped" {
				skipped++
			}
		}
		if failed && skipped == 2 {
			goto afterFailFast
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for fail-fast to skip nodes")

afterFailFast:

	// Fix prompt then retry.
	patchBody, _ = json.Marshal(map[string]string{"prompt": `echo "[n1] ok"; sleep 0.02; echo "[n1] done"`})
	patchReq, err = http.NewRequest(http.MethodPatch, httpSrv.URL+"/api/v1/nodes/"+target.ID, bytes.NewReader(patchBody))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes, err = http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("patch node: %v", err)
	}
	patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch status: %s", patchRes.Status)
	}

	retryRes, err := http.Post(httpSrv.URL+"/api/v1/nodes/"+target.ID+"/retry", "application/json", nil)
	if err != nil {
		t.Fatalf("retry node: %v", err)
	}
	if retryRes.StatusCode != http.StatusOK {
		retryRes.Body.Close()
		t.Fatalf("unexpected retry status: %s", retryRes.Status)
	}
	var retried struct {
		Workflow store.Workflow `json:"workflow"`
		Nodes    []store.Node   `json:"nodes"`
	}
	if err := json.NewDecoder(retryRes.Body).Decode(&retried); err != nil {
		retryRes.Body.Close()
		t.Fatalf("decode retry response: %v", err)
	}
	retryRes.Body.Close()

	foundQueued := false
	for _, n := range retried.Nodes {
		if n.ID == target.ID && n.Status == "queued" {
			foundQueued = true
		}
	}
	if !foundQueued {
		t.Fatalf("expected retried node to be queued")
	}

	if err := sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick after retry: %v", err)
	}

	// Wait for retried node to succeed.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		for _, n := range ns {
			if n.ID == target.ID && n.Status == "succeeded" {
				goto afterRetrySucceeded
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for retried node to succeed")

afterRetrySucceeded:

	// Approve + run remaining steps.
	for step := 0; step < 2; step++ {
		approveRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/approve", "application/json", nil)
		if err != nil {
			t.Fatalf("approve workflow: %v", err)
		}
		if approveRes.StatusCode != http.StatusOK {
			approveRes.Body.Close()
			t.Fatalf("unexpected approve status: %s", approveRes.Status)
		}
		var approved struct {
			Nodes []store.Node `json:"nodes"`
		}
		if err := json.NewDecoder(approveRes.Body).Decode(&approved); err != nil {
			approveRes.Body.Close()
			t.Fatalf("decode approve response: %v", err)
		}
		approveRes.Body.Close()

		if len(approved.Nodes) != 1 {
			t.Fatalf("expected 1 runnable node to be approved, got %d", len(approved.Nodes))
		}
		nodeID := approved.Nodes[0].ID

		if err := sched.Tick(context.Background()); err != nil {
			t.Fatalf("scheduler tick: %v", err)
		}

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
			if err != nil {
				t.Fatalf("list nodes: %v", err)
			}
			var ns []store.Node
			if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
				nodesRes.Body.Close()
				t.Fatalf("decode nodes: %v", err)
			}
			nodesRes.Body.Close()
			for _, n := range ns {
				if n.ID == nodeID && n.Status == "succeeded" {
					goto nextStep
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for node %s to succeed", nodeID)

	nextStep:
	}

	// All worker nodes succeeded => workflow done.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		getWfRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID)
		if err != nil {
			t.Fatalf("get workflow: %v", err)
		}
		var wf store.Workflow
		if err := json.NewDecoder(getWfRes.Body).Decode(&wf); err != nil {
			getWfRes.Body.Close()
			t.Fatalf("decode workflow: %v", err)
		}
		getWfRes.Body.Close()
		if wf.Status == "done" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for workflow to be done")
}

func TestNodeCancel_CancelsRunningNode(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)
	experts := expert.NewRegistry(config.Default())

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sched := scheduler.New(scheduler.Options{Store: st, Executions: execMgr, Hub: hub, Experts: experts, MaxConcurrency: 4})
	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st, Experts: experts})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	createReq := []byte(`{"title":"hello","workspace_path":".","mode":"manual"}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createReq))
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

	startRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %s", startRes.Status)
	}

	// Wait for DAG to be applied (master exits and creates worker nodes).
	waitDeadline := time.Now().Add(5 * time.Second)
	var nodes []store.Node
	for time.Now().Before(waitDeadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		if len(ns) >= 4 {
			nodes = ns
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(nodes) < 4 {
		t.Fatalf("expected dag nodes to be created, got %d", len(nodes))
	}

	var target store.Node
	for _, n := range nodes {
		if n.NodeType != "master" {
			target = n
			break
		}
	}
	if target.ID == "" {
		t.Fatalf("missing worker node")
	}

	approveRes, err := http.Post(httpSrv.URL+"/api/v1/workflows/"+created.ID+"/approve", "application/json", nil)
	if err != nil {
		t.Fatalf("approve workflow: %v", err)
	}
	if approveRes.StatusCode != http.StatusOK {
		approveRes.Body.Close()
		t.Fatalf("unexpected approve status: %s", approveRes.Status)
	}
	var approved struct {
		Nodes []store.Node `json:"nodes"`
	}
	if err := json.NewDecoder(approveRes.Body).Decode(&approved); err != nil {
		approveRes.Body.Close()
		t.Fatalf("decode approve response: %v", err)
	}
	approveRes.Body.Close()
	if len(approved.Nodes) != 1 {
		t.Fatalf("expected 1 runnable node to be approved, got %d", len(approved.Nodes))
	}
	target = approved.Nodes[0]

	// Make it run long enough to cancel.
	patchBody, _ := json.Marshal(map[string]string{"prompt": `echo "[run] start"; sleep 10; echo "[run] end"`})
	patchReq, err := http.NewRequest(http.MethodPatch, httpSrv.URL+"/api/v1/nodes/"+target.ID, bytes.NewReader(patchBody))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("patch node: %v", err)
	}
	patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected patch status: %s", patchRes.Status)
	}

	if err := sched.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	// Wait for node to start.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		for _, n := range ns {
			if n.ID == target.ID && n.Status == "running" && n.LastExecution != nil && *n.LastExecution != "" {
				goto started
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for node to start")

started:
	cancelRes, err := http.Post(httpSrv.URL+"/api/v1/nodes/"+target.ID+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("cancel node: %v", err)
	}
	cancelRes.Body.Close()
	if cancelRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected cancel status: %s", cancelRes.Status)
	}

	// Wait for node to be canceled.
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nodesRes, err := http.Get(httpSrv.URL + "/api/v1/workflows/" + created.ID + "/nodes")
		if err != nil {
			t.Fatalf("list nodes: %v", err)
		}
		var ns []store.Node
		if err := json.NewDecoder(nodesRes.Body).Decode(&ns); err != nil {
			nodesRes.Body.Close()
			t.Fatalf("decode nodes: %v", err)
		}
		nodesRes.Body.Close()
		for _, n := range ns {
			if n.ID == target.ID && n.Status == "canceled" {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for node to be canceled")
}
