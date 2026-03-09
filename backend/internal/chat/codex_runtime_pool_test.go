package chat

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/store"
)

type fakeCodexClient struct {
	mu sync.Mutex

	threadID         string
	initializeCalls  int
	startThreadCalls int
	resumeCalls      int
	startTurnCalls   int
	closeCalls       int
	closed           bool
	notes            chan codexAppServerNotification
	done             chan struct{}
}

func newFakeCodexClient(threadID string) *fakeCodexClient {
	return &fakeCodexClient{threadID: threadID, notes: make(chan codexAppServerNotification, 16), done: make(chan struct{})}
}

func (f *fakeCodexClient) Initialize(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initializeCalls++
	return nil
}

func (f *fakeCodexClient) StartThread(context.Context, codexAppServerThreadRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startThreadCalls++
	return f.threadID, nil
}

func (f *fakeCodexClient) ResumeThread(_ context.Context, req codexAppServerThreadRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resumeCalls++
	if req.ThreadID != "" {
		f.threadID = req.ThreadID
	}
	return f.threadID, nil
}

func (f *fakeCodexClient) StartTurn(_ context.Context, threadID string, _ string, _ *string) (string, error) {
	f.mu.Lock()
	f.startTurnCalls++
	f.threadID = threadID
	f.mu.Unlock()
	go func() {
		f.notes <- codexAppServerNotification{Method: "item/agentMessage/delta", Params: []byte(`{"threadId":"` + threadID + `","delta":"ok"}`)}
		f.notes <- codexAppServerNotification{Method: "turn/completed", Params: []byte(`{"turn":{"status":"completed"}}`)}
	}()
	return "turn_1", nil
}

func (f *fakeCodexClient) Notifications() <-chan codexAppServerNotification {
	return f.notes
}

func (f *fakeCodexClient) Wait() error {
	return nil
}

func (f *fakeCodexClient) Done() <-chan struct{} {
	return f.done
}

func (f *fakeCodexClient) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	f.closeCalls++
	close(f.done)
	close(f.notes)
	return nil
}

func TestCodexRuntimePoolReusesWarmClient(t *testing.T) {
	t.Parallel()

	created := make([]*fakeCodexClient, 0, 2)
	pool := newCodexRuntimePool(time.Hour, 0)
	pool.newClient = func(context.Context, runner.RunSpec) (codexAppServerClient, error) {
		client := newFakeCodexClient("th_reuse")
		created = append(created, client)
		return client, nil
	}
	defer func() { _ = pool.Close() }()

	req := codexAppServerThreadRequest{Model: "gpt-5", Cwd: "/tmp/workspace", BaseInstructions: "sys"}
	lease1, err := pool.Acquire(context.Background(), "cs_1", runner.RunSpec{Command: "codex"}, req)
	if err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	lease1.SetThreadID("th_reuse")
	lease1.Release()

	lease2, err := pool.Acquire(context.Background(), "cs_1", runner.RunSpec{Command: "codex"}, req)
	if err != nil {
		t.Fatalf("acquire second: %v", err)
	}
	defer lease2.Release()

	if len(created) != 1 {
		t.Fatalf("expected one client, got %d", len(created))
	}
	if !lease2.Fresh() && lease2.ThreadID() != "th_reuse" {
		t.Fatalf("expected warm thread id, got %q", lease2.ThreadID())
	}
	if created[0].initializeCalls != 1 {
		t.Fatalf("initialize calls = %d", created[0].initializeCalls)
	}
}

func TestCodexRuntimePoolRebuildsOnSignatureChange(t *testing.T) {
	t.Parallel()

	created := make([]*fakeCodexClient, 0, 2)
	pool := newCodexRuntimePool(time.Hour, 0)
	pool.newClient = func(context.Context, runner.RunSpec) (codexAppServerClient, error) {
		client := newFakeCodexClient("th_sig")
		created = append(created, client)
		return client, nil
	}
	defer func() { _ = pool.Close() }()

	lease1, err := pool.Acquire(context.Background(), "cs_1", runner.RunSpec{Command: "codex"}, codexAppServerThreadRequest{Model: "gpt-5", Cwd: "/tmp/a"})
	if err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	lease1.Release()

	lease2, err := pool.Acquire(context.Background(), "cs_1", runner.RunSpec{Command: "codex"}, codexAppServerThreadRequest{Model: "gpt-5", Cwd: "/tmp/b"})
	if err != nil {
		t.Fatalf("acquire second: %v", err)
	}
	defer lease2.Release()

	if len(created) != 2 {
		t.Fatalf("expected two clients, got %d", len(created))
	}
	if created[0].closeCalls != 1 {
		t.Fatalf("expected first client to close once, got %d", created[0].closeCalls)
	}
}

func TestCodexRuntimePoolEvictsExpiredRuntime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	client := newFakeCodexClient("th_idle")
	pool := newCodexRuntimePool(time.Minute, 0)
	pool.now = func() time.Time { return now }
	pool.newClient = func(context.Context, runner.RunSpec) (codexAppServerClient, error) { return client, nil }
	defer func() { _ = pool.Close() }()

	lease, err := pool.Acquire(context.Background(), "cs_1", runner.RunSpec{Command: "codex"}, codexAppServerThreadRequest{Model: "gpt-5", Cwd: "/tmp/a"})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	lease.Release()

	now = now.Add(2 * time.Minute)
	pool.reapExpired()

	if client.closeCalls != 1 {
		t.Fatalf("expected idle client close once, got %d", client.closeCalls)
	}
	pool.mu.Lock()
	_, ok := pool.sessions["cs_1"]
	pool.mu.Unlock()
	if ok {
		t.Fatalf("expected expired session entry to be removed")
	}
}

func TestManagerRunTurnReusesWarmCodexClient(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "warm",
		ExpertID:      "codex",
		Provider:      "cli",
		Model:         "gpt-5",
		WorkspacePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	var mu sync.Mutex
	created := make([]*fakeCodexClient, 0, 2)
	oldFactory := newCodexAppServerClient
	newCodexAppServerClient = func(context.Context, runner.RunSpec) (codexAppServerClient, error) {
		mu.Lock()
		defer mu.Unlock()
		client := newFakeCodexClient("th_warm")
		created = append(created, client)
		return client, nil
	}
	defer func() { newCodexAppServerClient = oldFactory }()

	mgr := NewManager(st, nil, Options{})
	defer func() { _ = mgr.Close() }()

	runSpec := runner.RunSpec{Command: "codex", Cwd: sess.WorkspacePath, Env: map[string]string{"VIBE_TREE_CLI_FAMILY": "codex", "VIBE_TREE_MODEL": "gpt-5"}}
	if _, err := mgr.RunTurn(context.Background(), TurnParams{Session: sess, ExpertID: "codex", UserInput: "first", ModelInput: "first", Spec: runSpec, Provider: "cli", Model: "gpt-5"}); err != nil {
		t.Fatalf("run first turn: %v", err)
	}
	sess, err = st.GetChatSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if _, err := mgr.RunTurn(context.Background(), TurnParams{Session: sess, ExpertID: "codex", UserInput: "second", ModelInput: "second", Spec: runSpec, Provider: "cli", Model: "gpt-5"}); err != nil {
		t.Fatalf("run second turn: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(created) != 1 {
		t.Fatalf("expected one warm client, got %d", len(created))
	}
	if created[0].initializeCalls != 1 {
		t.Fatalf("initialize calls = %d", created[0].initializeCalls)
	}
	if created[0].startThreadCalls != 1 {
		t.Fatalf("startThread calls = %d", created[0].startThreadCalls)
	}
	if created[0].resumeCalls != 0 {
		t.Fatalf("resume calls = %d", created[0].resumeCalls)
	}
	if created[0].startTurnCalls != 2 {
		t.Fatalf("startTurn calls = %d", created[0].startTurnCalls)
	}
}

func TestManagerReleaseSessionRuntimeClosesWarmClient(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "release",
		ExpertID:      "codex",
		Provider:      "cli",
		Model:         "gpt-5",
		WorkspacePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	client := newFakeCodexClient("th_release")
	oldFactory := newCodexAppServerClient
	newCodexAppServerClient = func(context.Context, runner.RunSpec) (codexAppServerClient, error) {
		return client, nil
	}
	defer func() { newCodexAppServerClient = oldFactory }()

	mgr := NewManager(st, nil, Options{})
	defer func() { _ = mgr.Close() }()

	runSpec := runner.RunSpec{Command: "codex", Cwd: sess.WorkspacePath, Env: map[string]string{"VIBE_TREE_CLI_FAMILY": "codex", "VIBE_TREE_MODEL": "gpt-5"}}
	if _, err := mgr.RunTurn(context.Background(), TurnParams{Session: sess, ExpertID: "codex", UserInput: "first", ModelInput: "first", Spec: runSpec, Provider: "cli", Model: "gpt-5"}); err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if err := mgr.ReleaseSessionRuntime(sess.ID); err != nil {
		t.Fatalf("release runtime: %v", err)
	}
	if client.closeCalls != 1 {
		t.Fatalf("expected close once, got %d", client.closeCalls)
	}
}
