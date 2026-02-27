package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"vibe-tree/backend/internal/api"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/server"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

func TestWorkflowCRUD(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 50 * time.Millisecond
	execRunner := runner.PTYRunner{DefaultGrace: grace}
	execMgr := execution.NewManager(execRunner, grace, hub)

	stateDBPath := filepath.Join(t.TempDir(), "state.db")
	st, err := store.Open(context.Background(), stateDBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub, Store: st})
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
