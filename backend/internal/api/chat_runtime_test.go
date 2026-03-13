package api

import (
	"testing"

	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/expert"
	"vibecraft/backend/internal/runner"
)

func TestApplyLLMModelRuntime_UsesPersistedSourceConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	cfg := config.Default()
	cfg.LLM = &config.LLMSettings{
		Sources: []config.LLMSourceConfig{{
			ID:       "openai-proxy",
			Label:    "OpenAI Proxy",
			Provider: "openai",
			BaseURL:  "https://proxy.example.com",
			APIKey:   "sk-proxy-123",
		}},
		Models: []config.LLMModelConfig{{
			ID:       "gpt-proxy",
			Label:    "GPT Proxy",
			Provider: "openai",
			Model:    "gpt-4.1-mini",
			SourceID: "openai-proxy",
		}},
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	if err := config.SaveTo(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	resolved := expert.Resolved{
		Spec: runner.RunSpec{
			Env: map[string]string{},
			SDK: &runner.SDKSpec{
				Provider:   "openai",
				Model:      "gpt-4.1-mini",
				LLMModelID: "gpt-proxy",
			},
		},
		ExpertID:       "gpt-proxy",
		Provider:       "openai",
		ProtocolFamily: "openai",
		Model:          "gpt-4.1-mini",
		PrimaryModelID: "gpt-proxy",
	}

	patched, status, err := applyLLMModelRuntime(resolved, "gpt-proxy")
	if err != nil {
		t.Fatalf("apply llm model runtime: status=%d err=%v", status, err)
	}
	if got := patched.Spec.SDK.BaseURL; got != "https://proxy.example.com" {
		t.Fatalf("sdk base_url = %q, want https://proxy.example.com", got)
	}
	if got := patched.Spec.Env["OPENAI_API_KEY"]; got != "sk-proxy-123" {
		t.Fatalf("OPENAI_API_KEY = %q, want sk-proxy-123", got)
	}
	if got := patched.Spec.Env["OPENAI_BASE_URL"]; got != "https://proxy.example.com" {
		t.Fatalf("OPENAI_BASE_URL = %q, want https://proxy.example.com", got)
	}
}
