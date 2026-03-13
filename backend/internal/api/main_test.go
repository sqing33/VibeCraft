package api_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Integration tests should not depend on real network keys; SDK calls are mocked.
	// Provide dummy values so expert env template expansion can succeed.
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		_ = os.Setenv("ANTHROPIC_API_KEY", "test")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		_ = os.Setenv("OPENAI_API_KEY", "test")
	}

	os.Exit(m.Run())
}
