package runner_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"vibe-tree/backend/internal/runner"
)

func TestNormalizeBaseURL_OpenAI_AppendsV1(t *testing.T) {
	got := runner.NormalizeBaseURL("openai", "https://example.com")
	if got != "https://example.com/v1" {
		t.Fatalf("unexpected: %q", got)
	}

	got = runner.NormalizeBaseURL("openai", "https://example.com/")
	if got != "https://example.com/v1" {
		t.Fatalf("unexpected: %q", got)
	}

	got = runner.NormalizeBaseURL("openai", "https://example.com/api")
	if got != "https://example.com/api/v1" {
		t.Fatalf("unexpected: %q", got)
	}

	got = runner.NormalizeBaseURL("openai", "https://example.com/v1")
	if got != "https://example.com/v1" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestNormalizeBaseURL_Anthropic_RemovesV1(t *testing.T) {
	got := runner.NormalizeBaseURL("anthropic", "https://example.com/v1")
	if got != "https://example.com" {
		t.Fatalf("unexpected: %q", got)
	}

	got = runner.NormalizeBaseURL("anthropic", "https://example.com/api/v1")
	if got != "https://example.com/api" {
		t.Fatalf("unexpected: %q", got)
	}

	got = runner.NormalizeBaseURL("anthropic", "https://example.com")
	if got != "https://example.com" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestSDKRunner_StartOneshot_FallsBackToSecondary(t *testing.T) {
	t.Parallel()

	r := runner.NewSDKRunner()
	h, err := r.StartOneshot(context.Background(), runner.RunSpec{
		SDK: &runner.SDKSpec{
			Provider: "broken-provider",
			Model:    "broken-model",
			Prompt:   "hello",
		},
		SDKFallbacks: []runner.SDKFallback{{
			SDK: runner.SDKSpec{Provider: "demo", Model: "demo", Prompt: "hello"},
		}},
	})
	if err != nil {
		t.Fatalf("start oneshot: %v", err)
	}
	defer h.Close()
	b, err := io.ReadAll(h.Output())
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	res, err := h.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", res.ExitCode)
	}
	out := string(b)
	if !strings.Contains(out, "fallback retry #1") {
		t.Fatalf("expected fallback notice, got: %s", out)
	}
}
