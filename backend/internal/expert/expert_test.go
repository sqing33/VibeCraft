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
				RunMode: "oneshot",
				Command: "codex",
				Args:    []string{"{{prompt}}"},
				Env:     map[string]string{"OPENAI_API_KEY": "${OPENAI_API_KEY}"},
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
