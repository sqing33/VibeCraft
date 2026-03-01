package api_test

import (
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
