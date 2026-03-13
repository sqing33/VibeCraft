package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"vibecraft/backend/internal/api"
	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/execution"
	"vibecraft/backend/internal/server"
	"vibecraft/backend/internal/ws"
)

func TestWebSocketBroadcastExecutionLifecycle(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 200 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	reqBody := []byte(`{"command":"bash","args":["-lc","echo hi; sleep 0.05; echo bye"]}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/executions", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("start execution: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}

	var started execution.Execution
	if err := json.NewDecoder(res.Body).Decode(&started); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if started.ID == "" {
		t.Fatalf("missing execution_id in start response")
	}

	type wsEnvelope struct {
		Type            string          `json:"type"`
		ExecutionID     string          `json:"execution_id"`
		OrchestrationID string          `json:"orchestration_id"`
		RoundID         string          `json:"round_id"`
		AgentRunID      string          `json:"agent_run_id"`
		Payload         json.RawMessage `json:"payload"`
	}

	decodeEnvelopes := func(msg []byte) ([]wsEnvelope, error) {
		var batch []wsEnvelope
		if err := json.Unmarshal(msg, &batch); err == nil {
			return batch, nil
		}
		var single wsEnvelope
		if err := json.Unmarshal(msg, &single); err != nil {
			return nil, err
		}
		return []wsEnvelope{single}, nil
	}

	deadline := time.Now().Add(3 * time.Second)
	_ = conn.SetReadDeadline(deadline)

	var gotStarted, gotLog, gotExited bool
	for !gotExited {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		envs, err := decodeEnvelopes(msg)
		if err != nil {
			t.Fatalf("parse ws json: %v (msg=%q)", err, string(msg))
		}
		for _, env := range envs {
			if env.ExecutionID != started.ID {
				continue
			}
			switch env.Type {
			case "execution.started":
				gotStarted = true
			case "node.log":
				gotLog = true
			case "execution.exited":
				gotExited = true
			}
		}
	}

	if !gotStarted {
		t.Fatalf("missing execution.started event")
	}
	if !gotLog {
		t.Fatalf("missing node.log event")
	}
	if !gotExited {
		t.Fatalf("missing execution.exited event")
	}
}

func TestCancelExecutionBroadcastsCanceledExit(t *testing.T) {
	t.Parallel()

	hub := ws.NewHub()
	grace := 100 * time.Millisecond
	execMgr := newTestExecMgr(grace, hub)

	engine := server.New(server.Options{DevCORS: false}, api.Deps{Executions: execMgr, Hub: hub})
	httpSrv := httptest.NewServer(engine)
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	reqBody := []byte(`{"command":"bash","args":["-lc","echo start; sleep 10; echo end"]}`)
	res, err := http.Post(httpSrv.URL+"/api/v1/executions", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("start execution: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}

	var started execution.Execution
	if err := json.NewDecoder(res.Body).Decode(&started); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if started.ID == "" {
		t.Fatalf("missing execution_id in start response")
	}

	deadline := time.Now().Add(3 * time.Second)
	_ = conn.SetReadDeadline(deadline)

	type envelope struct {
		Type        string          `json:"type"`
		ExecutionID string          `json:"execution_id"`
		Payload     json.RawMessage `json:"payload"`
	}

	decodeEnvelopes := func(msg []byte) ([]envelope, error) {
		var batch []envelope
		if err := json.Unmarshal(msg, &batch); err == nil {
			return batch, nil
		}
		var single envelope
		if err := json.Unmarshal(msg, &single); err != nil {
			return nil, err
		}
		return []envelope{single}, nil
	}

	// Wait until we see the process produce any log (ensures it's running).
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		envs, err := decodeEnvelopes(msg)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if env.ExecutionID != started.ID {
				continue
			}
			if env.Type == "node.log" {
				goto gotLog
			}
		}
	}
gotLog:

	cancelRes, err := http.Post(httpSrv.URL+"/api/v1/executions/"+started.ID+"/cancel", "application/json", nil)
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	cancelRes.Body.Close()
	if cancelRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected cancel status: %s", cancelRes.Status)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		envs, err := decodeEnvelopes(msg)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if env.ExecutionID != started.ID || env.Type != "execution.exited" {
				continue
			}

			var payload struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				t.Fatalf("parse execution.exited payload: %v", err)
			}
			if payload.Status != "canceled" {
				t.Fatalf("expected canceled status, got %q", payload.Status)
			}
			return
		}
	}
}

func TestOrchestrationExecutionEventsIncludeCorrelation(t *testing.T) {
	env := newTestEnv(t, config.Default(), 4)
	wsURL := "ws" + strings.TrimPrefix(env.httpSrv.URL, "http") + "/api/v1/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	type envelope struct {
		Type            string `json:"type"`
		ExecutionID     string `json:"execution_id"`
		OrchestrationID string `json:"orchestration_id"`
		RoundID         string `json:"round_id"`
		AgentRunID      string `json:"agent_run_id"`
	}

	decodeEnvelopes := func(msg []byte) ([]envelope, error) {
		var batch []envelope
		if err := json.Unmarshal(msg, &batch); err == nil {
			return batch, nil
		}
		var single envelope
		if err := json.Unmarshal(msg, &single); err != nil {
			return nil, err
		}
		return []envelope{single}, nil
	}

	body, _ := json.Marshal(map[string]any{
		"title":          "ws-orch",
		"goal":           "分析聊天页差异，并改进工作流页",
		"workspace_path": initGitRepo(t),
	})
	res, err := http.Post(env.httpSrv.URL+"/api/v1/orchestrations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create orchestration: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create orchestration status: %s", res.Status)
	}
	var created struct {
		Orchestration struct {
			ID string `json:"orchestration_id"`
		} `json:"orchestration"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create orchestration response: %v", err)
	}
	if created.Orchestration.ID == "" {
		t.Fatalf("missing orchestration id")
	}
	if err := env.orchMgr.Tick(context.Background()); err != nil {
		t.Fatalf("orchestration tick: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		envs, err := decodeEnvelopes(msg)
		if err != nil {
			continue
		}
		for _, envMsg := range envs {
			if envMsg.Type != "execution.started" {
				continue
			}
			if envMsg.OrchestrationID != created.Orchestration.ID {
				continue
			}
			if envMsg.RoundID == "" || envMsg.AgentRunID == "" || envMsg.ExecutionID == "" {
				t.Fatalf("expected orchestration correlation ids in execution.started, got %+v", envMsg)
			}
			return
		}
	}
}
