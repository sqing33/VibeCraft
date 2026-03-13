package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/store"
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

func TestChatMessagesPagination_BeforeTurn(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-pagination",
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

	for i := 1; i <= 3; i++ {
		turnBody, _ := json.Marshal(map[string]any{"input": fmt.Sprintf("turn %d", i)})
		turnRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", "application/json", bytes.NewReader(turnBody))
		if err != nil {
			t.Fatalf("post turn %d: %v", i, err)
		}
		turnRes.Body.Close()
		if turnRes.StatusCode != http.StatusOK {
			t.Fatalf("unexpected turn status for %d: %s", i, turnRes.Status)
		}
	}

	msgsRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/messages?limit=4")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	defer msgsRes.Body.Close()
	if msgsRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected messages status: %s", msgsRes.Status)
	}
	var page []store.ChatMessage
	if err := json.NewDecoder(msgsRes.Body).Decode(&page); err != nil {
		t.Fatalf("decode messages response: %v", err)
	}
	if len(page) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(page))
	}
	if page[0].Turn != 3 || page[3].Turn != 6 {
		t.Fatalf("expected turns 3..6 page, got %d..%d", page[0].Turn, page[3].Turn)
	}

	olderRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/messages?limit=4&before_turn=3")
	if err != nil {
		t.Fatalf("list older messages: %v", err)
	}
	defer olderRes.Body.Close()
	if olderRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected older messages status: %s", olderRes.Status)
	}
	var older []store.ChatMessage
	if err := json.NewDecoder(olderRes.Body).Decode(&older); err != nil {
		t.Fatalf("decode older messages response: %v", err)
	}
	if len(older) != 2 {
		t.Fatalf("expected 2 older messages, got %d", len(older))
	}
	for _, msg := range older {
		if msg.Turn >= 3 {
			t.Fatalf("expected msg turn < 3, got %d", msg.Turn)
		}
	}

	invalidRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/messages?before_turn=0")
	if err != nil {
		t.Fatalf("list invalid before_turn: %v", err)
	}
	defer invalidRes.Body.Close()
	if invalidRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid before_turn, got %s", invalidRes.Status)
	}
}

func TestChatSessionLifecycle_HelperSDKExpert(t *testing.T) {
	cfg := config.Default()
	cfg.Experts = append(cfg.Experts, config.ExpertConfig{
		ID:         "demo_helper",
		Label:      "Demo Helper",
		Provider:   "demo",
		Model:      "demo-helper",
		HelperOnly: true,
		Env:        map[string]string{},
		TimeoutMs:  30 * 1000,
	})

	env := newTestEnv(t, cfg, 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-helper-sdk",
		"expert_id":      "demo_helper",
		"workspace_path": ".",
	})
	createRes, err := http.Post(baseURL+"/api/v1/chat/sessions", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer createRes.Body.Close()
	if createRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(createRes.Body)
		t.Fatalf("unexpected create status: %s body=%s", createRes.Status, strings.TrimSpace(string(body)))
	}

	var sess store.ChatSession
	if err := json.NewDecoder(createRes.Body).Decode(&sess); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if sess.ExpertID != "demo_helper" {
		t.Fatalf("expected demo_helper expert, got %q", sess.ExpertID)
	}
	if sess.Provider != "demo" || sess.Model != "demo-helper" {
		t.Fatalf("unexpected session model: provider=%q model=%q", sess.Provider, sess.Model)
	}

	turnBody, _ := json.Marshal(map[string]any{"input": "hello helper sdk"})
	turnRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", "application/json", bytes.NewReader(turnBody))
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	defer turnRes.Body.Close()
	if turnRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(turnRes.Body)
		t.Fatalf("unexpected turn status: %s body=%s", turnRes.Status, strings.TrimSpace(string(body)))
	}

	var turnResult map[string]any
	if err := json.NewDecoder(turnRes.Body).Decode(&turnResult); err != nil {
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
		t.Fatalf("expected 2 messages after one helper-sdk turn, got %d", len(messages))
	}
	if got := strings.TrimSpace(pointerValue(messages[0].ExpertID)); got != "demo_helper" {
		t.Fatalf("expected user message expert_id demo_helper, got %q", got)
	}
}

func pointerValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func TestChatSession_PerTurnExpertOverride_UpdatesSessionDefaults(t *testing.T) {
	cfg := config.Default()
	for i := range cfg.Experts {
		if cfg.Experts[i].ID == "demo" {
			cfg.Experts[i].Model = "demo-a"
		}
	}
	cfg.Experts = append(cfg.Experts, config.ExpertConfig{
		ID:        "demo_b",
		Label:     "Demo B",
		Provider:  "demo",
		Model:     "demo-b",
		Env:       map[string]string{},
		TimeoutMs: 30 * 1000,
	})

	env := newTestEnv(t, cfg, 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-override",
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
	if sess.ExpertID != "demo" {
		t.Fatalf("expected session expert demo, got %q", sess.ExpertID)
	}
	if sess.Provider != "demo" || sess.Model != "demo-a" {
		t.Fatalf("unexpected session model: provider=%q model=%q", sess.Provider, sess.Model)
	}

	turnBody, _ := json.Marshal(map[string]any{"input": "hello", "expert_id": "demo_b"})
	turnRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", "application/json", bytes.NewReader(turnBody))
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	defer turnRes.Body.Close()
	if turnRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected turn status: %s", turnRes.Status)
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
	var updated *store.ChatSession
	for i := range sessions {
		if sessions[i].ID == sess.ID {
			updated = &sessions[i]
			break
		}
	}
	if updated == nil {
		t.Fatalf("session not found in list")
	}
	if updated.ExpertID != "demo_b" || updated.Provider != "demo" || updated.Model != "demo-b" {
		t.Fatalf("unexpected session defaults after override: %+v", *updated)
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
	for _, msg := range messages {
		if msg.ExpertID == nil || *msg.ExpertID != "demo_b" {
			t.Fatalf("unexpected message expert_id: %+v", msg)
		}
		if msg.Provider == nil || *msg.Provider != "demo" {
			t.Fatalf("unexpected message provider: %+v", msg)
		}
		if msg.Model == nil || *msg.Model != "demo-b" {
			t.Fatalf("unexpected message model: %+v", msg)
		}
	}
}

func TestChatSession_MultipartTurnPersistsAttachments(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-attachments",
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

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("input", ""); err != nil {
		t.Fatalf("write input field: %v", err)
	}
	part, err := writer.CreateFormFile("files", "hello.txt")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write([]byte("hello attachment")); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", &body)
	if err != nil {
		t.Fatalf("new multipart turn request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	turnRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post multipart turn: %v", err)
	}
	defer turnRes.Body.Close()
	if turnRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected multipart turn status: %s", turnRes.Status)
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
		t.Fatalf("expected 2 messages after multipart turn, got %d", len(messages))
	}
	if len(messages[0].Attachments) != 1 {
		t.Fatalf("expected attachment metadata on user message, got %+v", messages[0])
	}
	if messages[0].ContentText != "（仅附件）" {
		t.Fatalf("expected attachment-only display text, got %q", messages[0].ContentText)
	}
}

func TestChatSession_AttachmentContentEndpoint(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-preview",
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

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "hello.txt")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write([]byte("preview me")); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", &body)
	if err != nil {
		t.Fatalf("new multipart turn request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	turnRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post multipart turn: %v", err)
	}
	defer turnRes.Body.Close()
	if turnRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected multipart turn status: %s", turnRes.Status)
	}

	msgsRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/messages")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	defer msgsRes.Body.Close()
	var messages []store.ChatMessage
	if err := json.NewDecoder(msgsRes.Body).Decode(&messages); err != nil {
		t.Fatalf("decode messages response: %v", err)
	}
	if len(messages) == 0 || len(messages[0].Attachments) == 0 {
		t.Fatalf("expected persisted attachment metadata, got %+v", messages)
	}
	att := messages[0].Attachments[0]
	contentRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/attachments/" + att.ID + "/content")
	if err != nil {
		t.Fatalf("get attachment content: %v", err)
	}
	defer contentRes.Body.Close()
	if contentRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected attachment content status: %s", contentRes.Status)
	}
	if got := contentRes.Header.Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("unexpected content-type: %q", got)
	}
	payload, err := io.ReadAll(contentRes.Body)
	if err != nil {
		t.Fatalf("read attachment content: %v", err)
	}
	if string(payload) != "preview me" {
		t.Fatalf("unexpected attachment content: %q", string(payload))
	}
}

func TestChatTurnsAPI_ReturnsPersistedTimeline(t *testing.T) {
	env := newTestEnv(t, config.Default(), 2)
	baseURL := env.httpSrv.URL

	createBody, _ := json.Marshal(map[string]any{
		"title":          "chat-turns",
		"expert_id":      "demo",
		"workspace_path": ".",
	})
	createRes, err := http.Post(baseURL+"/api/v1/chat/sessions", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer createRes.Body.Close()
	var sess store.ChatSession
	if err := json.NewDecoder(createRes.Body).Decode(&sess); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	turnBody, _ := json.Marshal(map[string]any{"input": "hello turns"})
	turnRes, err := http.Post(baseURL+"/api/v1/chat/sessions/"+sess.ID+"/turns", "application/json", bytes.NewReader(turnBody))
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	defer turnRes.Body.Close()
	if turnRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(turnRes.Body)
		t.Fatalf("unexpected turn status: %s body=%s", turnRes.Status, strings.TrimSpace(string(body)))
	}

	turnsRes, err := http.Get(baseURL + "/api/v1/chat/sessions/" + sess.ID + "/turns")
	if err != nil {
		t.Fatalf("list turns: %v", err)
	}
	defer turnsRes.Body.Close()
	if turnsRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(turnsRes.Body)
		t.Fatalf("unexpected turns status: %s body=%s", turnsRes.Status, strings.TrimSpace(string(body)))
	}
	var turns []store.ChatTurn
	if err := json.NewDecoder(turnsRes.Body).Decode(&turns); err != nil {
		t.Fatalf("decode turns response: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Status != "completed" {
		t.Fatalf("expected completed turn, got %+v", turns[0])
	}
	if turns[0].AssistantMessageID == nil || *turns[0].AssistantMessageID == "" {
		t.Fatalf("expected assistant linkage, got %+v", turns[0])
	}
	if len(turns[0].Items) == 0 {
		t.Fatalf("expected persisted items, got %+v", turns[0])
	}
	if turns[0].Items[0].Kind != "answer" {
		t.Fatalf("expected answer item first, got %+v", turns[0].Items)
	}
}
