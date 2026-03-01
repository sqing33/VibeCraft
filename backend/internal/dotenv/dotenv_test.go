package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Disabled(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("ANTHROPIC_API_KEY=new\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	backendDir := filepath.Join(repo, "backend")
	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(backendDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("VIBE_TREE_DOTENV", "0")
	t.Setenv("VIBE_TREE_DOTENV_PATH", "")
	t.Setenv("ANTHROPIC_API_KEY", "old")

	res, err := Load()
	if err != nil {
		t.Fatalf("Load() err: %v", err)
	}
	if res.SkippedReason != "disabled" {
		t.Fatalf("SkippedReason=%q, want %q", res.SkippedReason, "disabled")
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "old" {
		t.Fatalf("ANTHROPIC_API_KEY=%q, want %q", got, "old")
	}
}

func TestLoad_ExplicitPath_Overrides(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "custom.env")
	if err := os.WriteFile(envPath, []byte("ANTHROPIC_API_KEY=new\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	t.Setenv("VIBE_TREE_DOTENV", "")
	t.Setenv("VIBE_TREE_DOTENV_PATH", envPath)
	t.Setenv("ANTHROPIC_API_KEY", "old")

	res, err := Load()
	if err != nil {
		t.Fatalf("Load() err: %v", err)
	}
	if !res.Attempted || !res.Loaded {
		t.Fatalf("Attempted=%v Loaded=%v, want true/true", res.Attempted, res.Loaded)
	}
	if res.Path != envPath {
		t.Fatalf("Path=%q, want %q", res.Path, envPath)
	}
	if res.Keys != 1 {
		t.Fatalf("Keys=%d, want 1", res.Keys)
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "new" {
		t.Fatalf("ANTHROPIC_API_KEY=%q, want %q", got, "new")
	}
}

func TestLoad_RepoRootDiscovery(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("ANTHROPIC_API_KEY=new\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	backendDir := filepath.Join(repo, "backend")
	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(backendDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("VIBE_TREE_DOTENV", "")
	t.Setenv("VIBE_TREE_DOTENV_PATH", "")
	t.Setenv("ANTHROPIC_API_KEY", "old")

	res, err := Load()
	if err != nil {
		t.Fatalf("Load() err: %v", err)
	}
	if !res.Attempted || !res.Loaded {
		t.Fatalf("Attempted=%v Loaded=%v, want true/true", res.Attempted, res.Loaded)
	}

	absRepo, err := filepath.Abs(repo)
	if err != nil {
		t.Fatalf("abs repo: %v", err)
	}
	wantPath := filepath.Join(absRepo, ".env")
	if res.Path != wantPath {
		t.Fatalf("Path=%q, want %q", res.Path, wantPath)
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "new" {
		t.Fatalf("ANTHROPIC_API_KEY=%q, want %q", got, "new")
	}
}

func TestLoad_NoRepoRoot_Skips(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "nested")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("VIBE_TREE_DOTENV", "")
	t.Setenv("VIBE_TREE_DOTENV_PATH", "")

	res, err := Load()
	if err != nil {
		t.Fatalf("Load() err: %v", err)
	}
	if res.Attempted {
		t.Fatalf("Attempted=true, want false")
	}
	if res.SkippedReason != "no_repo_root" {
		t.Fatalf("SkippedReason=%q, want %q", res.SkippedReason, "no_repo_root")
	}
}
