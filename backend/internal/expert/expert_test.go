package expert

import (
	"testing"
	"time"

	"vibe-tree/backend/internal/config"
)

func TestRegistryResolve_ReplacesPromptAndEnvTemplates(t *testing.T) {
	cfg := config.Config{
		Experts: []config.ExpertConfig{
			{
				ID:        "bash",
				RunMode:   "oneshot",
				Command:   "bash",
				Args:      []string{"-lc", "{{prompt}}"},
				Env:       map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}"},
				TimeoutMs: 1234,
			},
		},
	}

	t.Setenv("OPENAI_API_KEY", "sk_test_123")

	r := NewRegistry(cfg)
	res, err := r.Resolve("bash", "echo hi", "/tmp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Spec.Command != "bash" {
		t.Fatalf("unexpected command: %q", res.Spec.Command)
	}
	if len(res.Spec.Args) != 2 || res.Spec.Args[1] != "echo hi" {
		t.Fatalf("unexpected args: %#v", res.Spec.Args)
	}
	if res.Spec.Cwd != "/tmp" {
		t.Fatalf("unexpected cwd: %q", res.Spec.Cwd)
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
				ID:      "codex",
				RunMode: "sdk",
				SDK: &config.ExpertSDKConfig{
					Provider: "openai",
					Model:    "gpt-5-codex",
				},
				Env: map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}"},
			},
		},
	}

	r := NewRegistry(cfg)
	if _, err := r.Resolve("codex", "hi", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRegistryResolve_SDKSpec(t *testing.T) {
	temp := 0.25
	cfg := config.Config{
		Experts: []config.ExpertConfig{
			{
				ID:      "codex",
				RunMode: "sdk",
				SDK: &config.ExpertSDKConfig{
					Provider:        "openai",
					Model:           "gpt-5-codex",
					BaseURL:         "https://example.invalid/${OPENAI_TENANT}",
					Instructions:    "be brief",
					MaxOutputTokens: 321,
					Temperature:     &temp,
					OutputSchema:    "dag_v1",
				},
				Env:       map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}", "OPENAI_TENANT": "${OPENAI_TENANT}"},
				TimeoutMs: 1234,
			},
		},
	}

	t.Setenv("OPENAI_API_KEY", "sk_test_123")
	t.Setenv("OPENAI_TENANT", "t1")

	r := NewRegistry(cfg)
	res, err := r.Resolve("codex", "hello", "/tmp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Spec.Command != "sdk:openai" {
		t.Fatalf("unexpected command: %q", res.Spec.Command)
	}
	if len(res.Spec.Args) != 1 || res.Spec.Args[0] != "gpt-5-codex" {
		t.Fatalf("unexpected args: %#v", res.Spec.Args)
	}
	if res.Spec.Cwd != "/tmp" {
		t.Fatalf("unexpected cwd: %q", res.Spec.Cwd)
	}
	if res.Spec.SDK == nil {
		t.Fatalf("expected sdk spec")
	}
	if res.Spec.SDK.Provider != "openai" || res.Spec.SDK.Model != "gpt-5-codex" {
		t.Fatalf("unexpected sdk: %#v", res.Spec.SDK)
	}
	if res.Spec.SDK.Prompt != "hello" || res.Spec.SDK.Instructions != "be brief" {
		t.Fatalf("unexpected prompt/instructions: %#v", res.Spec.SDK)
	}
	if res.Spec.SDK.BaseURL != "https://example.invalid/t1" {
		t.Fatalf("unexpected base_url: %q", res.Spec.SDK.BaseURL)
	}
	if res.Spec.Env["OPENAI_API_KEY"] != "sk_test_123" {
		t.Fatalf("unexpected env OPENAI_API_KEY: %q", res.Spec.Env["OPENAI_API_KEY"])
	}
	if res.Timeout != 1234*time.Millisecond {
		t.Fatalf("unexpected timeout: %s", res.Timeout)
	}
}

func TestRegistryResolve_UnsupportedRunModeErrors(t *testing.T) {
	cfg := config.Config{
		Experts: []config.ExpertConfig{
			{
				ID:      "mcp",
				RunMode: "daemon",
				Command: "mcp",
			},
		},
	}

	r := NewRegistry(cfg)
	if _, err := r.Resolve("mcp", "hi", ""); err == nil {
		t.Fatalf("expected error")
	}
}
