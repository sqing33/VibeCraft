package runner_test

import (
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

