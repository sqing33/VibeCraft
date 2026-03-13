package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestSaveTo_Writes0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	cfg := config.Default()
	if err := config.SaveTo(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("unexpected file perm: %v", got)
	}
}
