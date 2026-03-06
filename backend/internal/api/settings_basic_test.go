package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestBasicSettings_PutAndGetThinkingTranslation(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai", APIKey: "sk_test_123456"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"}},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body := []byte(`{"thinking_translation":{"source_id":"openai-default","model":"translator-fast","target_model_ids":["gpt-5-codex"]}}`)
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/basic", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: %s body=%s", res.Status, strings.TrimSpace(string(payload)))
	}

	getRes, err := http.Get(env.httpSrv.URL + "/api/v1/settings/basic")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get status: %s", getRes.Status)
	}
	var out struct {
		ThinkingTranslation *struct {
			SourceID       string   `json:"source_id"`
			Model          string   `json:"model"`
			TargetModelIDs []string `json:"target_model_ids"`
		} `json:"thinking_translation"`
	}
	if err := json.NewDecoder(getRes.Body).Decode(&out); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if out.ThinkingTranslation == nil {
		t.Fatalf("expected thinking translation config")
	}
	if out.ThinkingTranslation.Model != "translator-fast" {
		t.Fatalf("unexpected model: %q", out.ThinkingTranslation.Model)
	}
}

func TestBasicSettings_PutRejectsUnknownTargetModel(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"}},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body := []byte(`{"thinking_translation":{"source_id":"openai-default","model":"translator-fast","target_model_ids":["missing-model"]}}`)
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/basic", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: %s body=%s", res.Status, strings.TrimSpace(string(payload)))
	}
}

func TestLLMSettings_PutClearsStaleThinkingTranslation(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{ID: "openai-default", Provider: "openai"}},
		Models:  []config.LLMModelConfig{{ID: "gpt-5-codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"}},
	}
	cfg.Basic = &config.BasicSettings{
		ThinkingTranslation: &config.ThinkingTranslationSettings{
			SourceID:       "openai-default",
			Model:          "translator-fast",
			TargetModelIDs: []string{"gpt-5-codex"},
		},
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	env := newTestEnv(t, cfg, 2)
	body := []byte(`{"sources":[{"id":"anthropic-default","label":"Anthropic","provider":"anthropic"}],"models":[{"id":"claude-3-7-sonnet","label":"Claude 3.7","provider":"anthropic","model":"claude-3-7-sonnet","source_id":"anthropic-default"}]}`)
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/llm", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put llm settings: %v", err)
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
	if persisted.Basic != nil {
		t.Fatalf("expected stale thinking translation config to be cleared, got %+v", persisted.Basic)
	}
}
