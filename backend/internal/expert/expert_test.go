package expert

import (
	"testing"
	"time"

	"vibecraft/backend/internal/config"
)

func TestRegistryResolve_ReplacesPromptAndEnvTemplates(t *testing.T) {
	cfg := config.Config{
		Experts: []config.ExpertConfig{
			{
				ID:             "codex",
				Provider:       "openai",
				Model:          "gpt-5-codex",
				PromptTemplate: "[{{workspace}}] {{prompt}}",
				Env:            map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}"},
				TimeoutMs:      1234,
			},
		},
	}

	t.Setenv("OPENAI_API_KEY", "sk_test_123")

	r := NewRegistry(cfg)
	res, err := r.Resolve("codex", "echo hi", "/tmp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Spec.SDK == nil {
		t.Fatalf("expected sdk spec to be set")
	}
	if res.Spec.SDK.Provider != "openai" {
		t.Fatalf("unexpected provider: %q", res.Spec.SDK.Provider)
	}
	if res.Spec.Cwd != "/tmp" {
		t.Fatalf("unexpected cwd: %q", res.Spec.Cwd)
	}
	if res.Spec.SDK.Prompt != "[/tmp] echo hi" {
		t.Fatalf("unexpected prompt: %q", res.Spec.SDK.Prompt)
	}
	if res.Spec.SDK.Model != "gpt-5-codex" {
		t.Fatalf("unexpected model: %q", res.Spec.SDK.Model)
	}
	if got := res.Spec.Env["OPENAI_API_KEY"]; got != "sk_test_123" {
		t.Fatalf("unexpected env OPENAI_API_KEY: %q", got)
	}
	if res.Timeout != 1234*time.Millisecond {
		t.Fatalf("unexpected timeout: %s", res.Timeout)
	}
}

func TestRegistryResolve_MissingEnvTemplateErrors(t *testing.T) {
	cfg := config.Config{
		Experts: []config.ExpertConfig{
			{
				ID:       "codex",
				Provider: "openai",
				Model:    "gpt-5-codex",
				Env:      map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}"},
			},
		},
	}

	r := NewRegistry(cfg)
	if _, err := r.Resolve("codex", "hi", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRegistryResolve_UnsupportedRunModeErrors(t *testing.T) {
	cfg := config.Config{
		Experts: []config.ExpertConfig{
			{
				ID:       "mcp",
				Provider: "mcp",
				Model:    "foo",
			},
		},
	}

	r := NewRegistry(cfg)
	if _, err := r.Resolve("mcp", "hi", ""); err == nil {
		t.Fatalf("expected error")
	}
}
