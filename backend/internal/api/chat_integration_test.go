package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"vibe-tree/backend/internal/config"
	"vibe-tree/backend/internal/store"
)

func TestChatSessionLifecycle_DemoExpert(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-a",
		"expert_id":      "demo",
		"workspace_path": ".",
	})
	createRes, err := http.Post(baseURL+"/api/v1/chat/sessions", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer createRes.Body.Close()
	if createRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create status: %s", createRes.Status)
	}

	var sess store.ChatSession
	if err := json.NewDecoder(createRes.Body).Decode(&sess); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if sess.ID == "" {
		t.Fatalf("missing session id")
	}
	if sess.Provider != "demo" {
		t.Fatalf("expected demo provider, got %q", sess.Provider)
	}

	listRes, err := http.Get(baseURL + "/api/v1/chat/sessions")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	defer listRes.Body.Close()
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %s", listRes.Status)
	}
	var sessions []store.ChatSession
	if err := json.NewDecoder(listRes.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected non-empty sessions list")
	}

	turnBody, _ := json.Marshal(map[string]any{"input": "hello world"})
	turnRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", "application/json", bytes.NewReader(turnBody))
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	defer turnRes.Body.Close()
	if turnRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected turn status: %s", turnRes.Status)
	}
	if err := json.NewDecoder(turnRes.Body).Decode(&map[string]any{}); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}

	msgsRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/messages")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	defer msgsRes.Body.Close()
	if msgsRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected messages status: %s", msgsRes.Status)
	}
	var messages []store.ChatMessage
	if err := json.NewDecoder(msgsRes.Body).Decode(&messages); err != nil {
		t.Fatalf("decode messages response: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages after one turn, got %d", len(messages))
	}

	compactRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/compact", "application/json", nil)
	if err != nil {
		t.Fatalf("compact session: %v", err)
	}
	defer compactRes.Body.Close()
	if compactRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected compact status: %s", compactRes.Status)
	}

	forkRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/fork", "application/json", nil)
	if err != nil {
		t.Fatalf("fork session: %v", err)
	}
	defer forkRes.Body.Close()
	if forkRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected fork status: %s", forkRes.Status)
	}
	var forked store.ChatSession
	if err := json.NewDecoder(forkRes.Body).Decode(&forked); err != nil {
		t.Fatalf("decode fork response: %v", err)
	}
	if forked.ID == "" || forked.ID == sess.ID {
		t.Fatalf("expected new fork session id")
	}
}
