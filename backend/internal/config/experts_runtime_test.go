package config_test

import (
	"testing"

	"vibe-tree/backend/internal/config"
)

func TestRebuildExperts_BackfillsBuiltinCLIExperts(t *testing.T) {
	cfg := config.Default()
	cfg.Experts = []config.ExpertConfig{
		{ID: "master", Label: "Master", Provider: "openai", Model: "gpt-5-codex", ManagedSource: config.ManagedSourceBuiltin},
		{ID: "codex", Label: "Codex", Provider: "cli", RuntimeKind: "cli", CLIFamily: "codex", Model: "gpt-5-codex", ManagedSource: config.ManagedSourceBuiltin},
		{ID: "claudecode", Label: "ClaudeCode", Provider: "cli", RuntimeKind: "cli", CLIFamily: "claude", Model: "claude-3-7-sonnet-latest", ManagedSource: config.ManagedSourceBuiltin},
	}

	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}

	foundIFLOW := false
	foundOpenCode := false
	for _, item := range cfg.Experts {
		switch item.ID {
		case "iflow":
			foundIFLOW = true
			if item.Provider != "cli" || item.CLIFamily != "iflow" {
				t.Fatalf("unexpected iflow expert: %#v", item)
			}
		case "opencode":
			foundOpenCode = true
			if item.Provider != "cli" || item.CLIFamily != "opencode" {
				t.Fatalf("unexpected opencode expert: %#v", item)
			}
		}
	}
	if !foundIFLOW {
		t.Fatalf("expected iflow expert to be backfilled")
	}
	if !foundOpenCode {
		t.Fatalf("expected opencode expert to be backfilled")
	}
}
