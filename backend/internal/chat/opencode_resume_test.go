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
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/store"
)

type opencodeMockRunner struct {
	mu              sync.Mutex
	calls           int
	resumeSessionID string
}

func (r *opencodeMockRunner) StartOneshot(ctx context.Context, spec runner.RunSpec) (runner.ProcessHandle, error) {
	r.mu.Lock()
	r.calls += 1
	callIndex := r.calls
	if callIndex == 1 {
		r.resumeSessionID = strings.TrimSpace(spec.Env["VIBECRAFT_RESUME_SESSION_ID"])
	}
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
		return &opencodeMockHandle{out: io.NopCloser(strings.NewReader("")), exitCode: 1}, nil
	}

	_ = os.WriteFile(filepath.Join(artifactDir, "final_message.md"), []byte("reconstructed answer\n"), 0o644)
	_ = os.WriteFile(filepath.Join(artifactDir, "summary.json"), []byte(`{"status":"ok","summary":"reconstructed answer","modified_code":false,"next_action":"","key_files":[]}`), 0o644)
	payload, _ := json.Marshal(map[string]any{
		"tool_id":    "opencode",
		"session_id": "opencode-new-session",
		"model":      spec.Env["VIBECRAFT_MODEL"],
		"resumed":    false,
		"call_index": callIndex,
	})
	_ = os.WriteFile(filepath.Join(artifactDir, "session.json"), payload, 0o644)
	return &opencodeMockHandle{out: io.NopCloser(strings.NewReader("")), exitCode: 0}, nil
}

type opencodeMockHandle struct {
	out      io.ReadCloser
	exitCode int
}

func (h *opencodeMockHandle) PID() int { return 0 }

func (h *opencodeMockHandle) Output() io.ReadCloser { return h.out }

func (h *opencodeMockHandle) Cancel(grace time.Duration) error { return nil }

func (h *opencodeMockHandle) WriteInput(p []byte) (int, error) { return len(p), nil }

func (h *opencodeMockHandle) Close() error {
	if h.out != nil {
		return h.out.Close()
	}
	return nil
}

func (h *opencodeMockHandle) Wait() (runner.ExitResult, error) {
	now := time.Now()
	return runner.ExitResult{ExitCode: h.exitCode, StartedAt: now, EndedAt: now}, nil
}

func TestRunTurn_OpenCodeResumeFailureFallsBackToReconstructedPrompt(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	oldSession := "opencode-old-session"
	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "opencode",
		ExpertID:      "opencode",
		CLIToolID:     pointer("opencode"),
		ModelID:       pointer("claude-sonnet"),
		CLISessionID:  pointer(oldSession),
		Provider:      "cli",
		Model:         "claude-3-7-sonnet",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	mockRunner := &opencodeMockRunner{}
	mgr := chat.NewManager(st, nil, chat.Options{KeepRecent: 4, Runner: mockRunner})
	result, err := mgr.RunTurn(context.Background(), chat.TurnParams{
		Session:    sess,
		ExpertID:   "opencode",
		UserInput:  "continue please",
		ModelInput: "continue please",
		Spec: runner.RunSpec{
			Command: "bash",
			Args:    []string{"scripts/agent-runtimes/opencode_exec.sh"},
			Env: map[string]string{
				"VIBECRAFT_CLI_FAMILY":  "opencode",
				"VIBECRAFT_CLI_TOOL_ID": "opencode",
				"VIBECRAFT_MODEL":       "claude-3-7-sonnet",
				"VIBECRAFT_MODEL_ID":    "claude-sonnet",
			},
			Cwd: ".",
		},
		Provider: "cli",
		Model:    "claude-3-7-sonnet",
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
	if updated.CLISessionID == nil || *updated.CLISessionID != "opencode-new-session" {
		t.Fatalf("cli_session_id = %#v, want opencode-new-session", updated.CLISessionID)
	}
	if mockRunner.calls != 2 {
		t.Fatalf("runner calls = %d, want 2", mockRunner.calls)
	}
	if mockRunner.resumeSessionID != oldSession {
		t.Fatalf("resume session id = %q, want %q", mockRunner.resumeSessionID, oldSession)
	}
}
