package chat_test

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vibecraft/backend/internal/chat"
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/store"
)

type opencodeLoopGuardRunner struct {
	handle *opencodeLoopGuardHandle
}

func (r *opencodeLoopGuardRunner) StartOneshot(ctx context.Context, spec runner.RunSpec) (runner.ProcessHandle, error) {
	var lines []string
	for i := 0; i < 48; i++ {
		lines = append(lines,
			fmt.Sprintf(`{"type":"step_start","sessionID":"ses_loop","part":{"id":"part_start_%d","sessionID":"ses_loop","messageID":"msg_%d","type":"step-start"}}`, i, i),
			fmt.Sprintf(`{"type":"step_finish","sessionID":"ses_loop","part":{"id":"part_finish_%d","sessionID":"ses_loop","messageID":"msg_%d","type":"step-finish","reason":"unknown"}}`, i, i),
		)
	}
	r.handle = &opencodeLoopGuardHandle{out: io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n"))}
	return r.handle, nil
}

type opencodeLoopGuardHandle struct {
	out       io.ReadCloser
	mu        sync.Mutex
	cancelled bool
}

func (h *opencodeLoopGuardHandle) PID() int { return 0 }

func (h *opencodeLoopGuardHandle) Output() io.ReadCloser { return h.out }

func (h *opencodeLoopGuardHandle) Cancel(grace time.Duration) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cancelled = true
	return nil
}

func (h *opencodeLoopGuardHandle) WriteInput(p []byte) (int, error) { return len(p), nil }

func (h *opencodeLoopGuardHandle) Close() error {
	if h.out != nil {
		return h.out.Close()
	}
	return nil
}

func (h *opencodeLoopGuardHandle) Wait() (runner.ExitResult, error) {
	now := time.Now()
	return runner.ExitResult{ExitCode: 1, StartedAt: now, EndedAt: now}, nil
}

func (h *opencodeLoopGuardHandle) wasCancelled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cancelled
}

func TestRunTurn_OpenCodeBlankStepLoopReturnsHelpfulMessage(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	sess, err := st.CreateChatSession(context.Background(), store.CreateChatSessionParams{
		Title:         "opencode loop",
		ExpertID:      "opencode",
		CLIToolID:     pointer("opencode"),
		ModelID:       pointer("minimax-2.5"),
		Provider:      "cli",
		Model:         "openai/minimax-2.5",
		WorkspacePath: ".",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	runnerMock := &opencodeLoopGuardRunner{}
	mgr := chat.NewManager(st, nil, chat.Options{KeepRecent: 4, Runner: runnerMock})
	result, err := mgr.RunTurn(context.Background(), chat.TurnParams{
		Session:    sess,
		ExpertID:   "opencode",
		UserInput:  "你是什么模型",
		ModelInput: "你是什么模型",
		Spec: runner.RunSpec{
			Command: "bash",
			Args:    []string{"scripts/agent-runtimes/opencode_exec.sh"},
			Env: map[string]string{
				"VIBECRAFT_CLI_FAMILY":  "opencode",
				"VIBECRAFT_CLI_TOOL_ID": "opencode",
				"VIBECRAFT_MODEL":       "openai/minimax-2.5",
				"VIBECRAFT_MODEL_ID":    "minimax-2.5",
			},
			Cwd: ".",
		},
		Provider: "cli",
		Model:    "openai/minimax-2.5",
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if !strings.Contains(result.AssistantMessage.ContentText, "持续执行空步骤") {
		t.Fatalf("assistant content = %q, want loop-guard hint", result.AssistantMessage.ContentText)
	}
	if runnerMock.handle == nil || !runnerMock.handle.wasCancelled() {
		t.Fatalf("expected loop guard to cancel process")
	}
}
