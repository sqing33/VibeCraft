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

func TestLLMSettings_MasksAPIKey(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{
			{ID: "openai-default", Provider: "openai", APIKey: "sk_test_123456"},
		},
		Models: []config.LLMModelConfig{
			{ID: "codex", Provider: "openai", Model: "gpt-5-codex", SourceID: "openai-default"},
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
	res, err := http.Get(env.httpSrv.URL + "/api/v1/settings/llm")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(b), "sk_test_123456") {
		t.Fatalf("plaintext key leaked in response body")
	}

	var out struct {
		Sources []struct {
			ID        string `json:"id"`
			HasKey    bool   `json:"has_key"`
			MaskedKey string `json:"masked_key"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Sources) != 1 {
		t.Fatalf("unexpected sources len: %d", len(out.Sources))
	}
	if !out.Sources[0].HasKey {
		t.Fatalf("expected has_key=true")
	}
	if out.Sources[0].MaskedKey != "****3456" {
		t.Fatalf("unexpected masked_key: %q", out.Sources[0].MaskedKey)
	}
}

func TestLLMSettings_PutNormalizesMixedCaseModelIdentifiers(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	env := newTestEnv(t, config.Default(), 2)
	body := []byte(`{
		"sources": [
			{"id":"openai-default","label":"OpenAI","provider":"openai","base_url":"https://api.example.com"}
		],
		"models": [
			{"id":"GPT-5-CODEX","label":"GPT-5-CODEX","provider":"openai","model":"GPT-5-CODEX","source_id":"openai-default"}
		]
	}`)

	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/llm", bytes.NewReader(body))
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

	var out struct {
		Models []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Models) != 1 {
		t.Fatalf("unexpected models len: %d", len(out.Models))
	}
	if out.Models[0].ID != "gpt-5-codex" {
		t.Fatalf("unexpected model id: %q", out.Models[0].ID)
	}
	if out.Models[0].Model != "gpt-5-codex" {
		t.Fatalf("unexpected model name: %q", out.Models[0].Model)
	}
	if out.Models[0].Label != "GPT-5-CODEX" {
		t.Fatalf("unexpected model label: %q", out.Models[0].Label)
	}

	expertsRes, err := http.Get(env.httpSrv.URL + "/api/v1/experts")
	if err != nil {
		t.Fatalf("get experts: %v", err)
	}
	defer expertsRes.Body.Close()
	if expertsRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected experts status: %s", expertsRes.Status)
	}

	var experts []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(expertsRes.Body).Decode(&experts); err != nil {
		t.Fatalf("decode experts: %v", err)
	}

	var found bool
	for _, expert := range experts {
		if expert.ID != "gpt-5-codex" {
			continue
		}
		found = true
		if expert.Model != "gpt-5-codex" {
			t.Fatalf("unexpected expert model: %q", expert.Model)
		}
		if expert.Label != "GPT-5-CODEX" {
			t.Fatalf("unexpected expert label: %q", expert.Label)
		}
	}
	if !found {
		t.Fatalf("normalized expert id not found in experts list")
	}
}
