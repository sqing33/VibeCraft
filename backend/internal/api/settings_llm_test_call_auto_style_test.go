package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestLLMTestAPI_PersistsDetectedStyleForSavedModel(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found","type":"bad_response_status_code","param":"","code":"bad_response_status_code"}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":null}],\"usage\":null}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer proxy.Close()

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Label: "OpenAI", Provider: "openai", BaseURL: proxy.URL, APIKey: "sk-test"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-model", Label: "GPT", Provider: "openai", Model: "gpt", SourceID: "openai-default"}},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body, _ := json.Marshal(map[string]any{"provider": "openai", "model": "gpt", "source_id": "openai-default"})
	res, err := http.Post(env.httpSrv.URL+"/api/v1/settings/llm/test", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: %s body=%s", res.Status, strings.TrimSpace(string(payload)))
	}
	persisted, _, err := config.LoadPersisted()
	if err != nil {
		t.Fatalf("load persisted: %v", err)
	}
	if got := persisted.LLM.Models[0].OpenAIAPIStyle; got != config.OpenAIAPIStyleChatCompletions {
		t.Fatalf("expected persisted chat_completions style, got %q", got)
	}
}

func TestLLMTestAPI_DraftProbeDoesNotPersistStyle(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found","type":"bad_response_status_code","param":"","code":"bad_response_status_code"}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":null}],\"usage\":null}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer proxy.Close()

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Label: "OpenAI", Provider: "openai", BaseURL: proxy.URL, APIKey: "sk-test"}},
		Models:  []config.LLMModelConfig{{ID: "saved-model", Label: "Saved", Provider: "openai", Model: "other-model", SourceID: "openai-default"}},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body, _ := json.Marshal(map[string]any{"provider": "openai", "model": "gpt", "base_url": proxy.URL, "api_key": "sk-test"})
	res, err := http.Post(env.httpSrv.URL+"/api/v1/settings/llm/test", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: %s body=%s", res.Status, strings.TrimSpace(string(payload)))
	}
	persisted, _, err := config.LoadPersisted()
	if err != nil {
		t.Fatalf("load persisted: %v", err)
	}
	if got := persisted.LLM.Models[0].OpenAIAPIStyle; got != "" {
		t.Fatalf("expected no persisted style for draft probe, got %q", got)
	}
}
