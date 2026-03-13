package chat_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vibecraft/backend/internal/chat"
	"vibecraft/backend/internal/config"
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/store"
)

type iflowMockRunner struct {
	mu    sync.Mutex
	calls int
}

func (r *iflowMockRunner) StartOneshot(ctx context.Context, spec runner.RunSpec) (runner.ProcessHandle, error) {
	r.mu.Lock()
	r.calls += 1
	callIndex := r.calls
	r.mu.Unlock()

	artifactDir := strings.TrimSpace(spec.Env["VIBECRAFT_ARTIFACT_DIR"])
	if artifactDir != "" {
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(spec.Env["VIBECRAFT_RESUME_SESSION_ID"]) != "" {
		_ = os.WriteFile(filepath.Join(artifactDir, "summary.json"), []byte(`{"status":"error","summary":"resume failed","modified_code":false,"next_action":"retry without resume","key_files":[]}`), 0o644)
		_ = os.WriteFile(filepath.Join(artifactDir, "final_message.md"), []byte("resume failed\n"), 0o644)
		return &iflowMockHandle{out: io.NopCloser(strings.NewReader("")), exitCode: 1}, nil
	}

	_ = os.WriteFile(filepath.Join(artifactDir, "final_message.md"), []byte("reconstructed answer\n"), 0o644)
	_ = os.WriteFile(filepath.Join(artifactDir, "summary.json"), []byte(`{"status":"ok","summary":"reconstructed answer","modified_code":false,"next_action":"","key_files":[]}`), 0o644)
	payload, _ := json.Marshal(map[string]any{
		"tool_id":    "iflow",
		"session_id": "iflow-new-session",
		"model":      spec.Env["VIBECRAFT_MODEL"],
		"resumed":    false,
		"call_index": callIndex,
	})
	_ = os.WriteFile(filepath.Join(artifactDir, "session.json"), payload, 0o644)
	return &iflowMockHandle{out: io.NopCloser(strings.NewReader("")), exitCode: 0}, nil
}

type iflowMockHandle struct {
	out      io.ReadCloser
	exitCode int
}

func (h *iflowMockHandle) PID() int { return 0 }

func (h *iflowMockHandle) Output() io.ReadCloser { return h.out }

func (h *iflowMockHandle) Cancel(grace time.Duration) error { return nil }

func (h *iflowMockHandle) WriteInput(p []byte) (int, error) { return len(p), nil }

func (h *iflowMockHandle) Close() error {
	if h.out != nil {
		return h.out.Close()
	}
	return nil
}

func (h *iflowMockHandle) Wait() (runner.ExitResult, error) {
	now := time.Now()
	return runner.ExitResult{ExitCode: h.exitCode, StartedAt: now, EndedAt: now}, nil
}

func TestRunTurn_IFLOWResumeFailureFallsBackToReconstructedPrompt(t *testing.T) {

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	if err := config.RebuildExperts(&cfg); err != nil {
		t.Fatalf("rebuild experts: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	oldSession := "iflow-old-session"
	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "iflow",
		ExpertID:      "iflow",
		CLIToolID:     pointer("iflow"),
		ModelID:       pointer("qwen3-coder"),
		CLISessionID:  pointer(oldSession),
		Provider:      "cli",
		Model:         "qwen3-coder",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	mockRunner := &iflowMockRunner{}
	mgr := chat.NewManager(st, nil, chat.Options{KeepRecent: 4, Runner: mockRunner})
	result, err := mgr.RunTurn(context.Background(), chat.TurnParams{
		Session:    sess,
		ExpertID:   "iflow",
		UserInput:  "continue please",
		ModelInput: "continue please",
		Spec: runner.RunSpec{
			Command: "bash",
			Args:    []string{"scripts/agent-runtimes/iflow_exec.sh"},
			Env: map[string]string{
				"VIBECRAFT_CLI_FAMILY":  "iflow",
				"VIBECRAFT_CLI_TOOL_ID": "iflow",
				"VIBECRAFT_MODEL":       "qwen3-coder",
				"VIBECRAFT_MODEL_ID":    "qwen3-coder",
			},
			Cwd: ".",
		},
		Provider: "cli",
		Model:    "qwen3-coder",
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if got := result.AssistantMessage.ContentText; got != "reconstructed answer" {
		t.Fatalf("assistant content = %q, want reconstructed answer", got)
	}
	updated, err := st.GetChatSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if updated.CLISessionID == nil || *updated.CLISessionID != "iflow-new-session" {
		t.Fatalf("cli_session_id = %#v, want iflow-new-session", updated.CLISessionID)
	}
	if mockRunner.calls != 2 {
		t.Fatalf("runner calls = %d, want 2", mockRunner.calls)
	}
}
