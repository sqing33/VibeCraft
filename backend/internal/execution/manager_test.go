package execution

import (
	"context"
	"os"
	"testing"
	"time"

	"vibecraft/backend/internal/paths"
	"vibecraft/backend/internal/runner"
)

func TestManager_StartOneshotWithOptions_ContextDeadlineMarksTimeout(t *testing.T) {
	grace := 50 * time.Millisecond
	r := runner.PTYRunner{DefaultGrace: grace}
	m := NewManager(r, grace, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	done := make(chan Execution, 1)
	exec, err := m.StartOneshotWithOptions(ctx, runner.RunSpec{
		Command: "bash",
		Args:    []string{"-lc", `echo "start"; sleep 10; echo "end"`},
	}, StartOptions{
		OnExit: func(exec Execution) {
			done <- exec
		},
	})
	if err != nil {
		t.Fatalf("StartOneshotWithOptions: %v", err)
	}
	t.Cleanup(func() {
		if p, err := paths.ExecutionLogPath(exec.ID); err == nil {
			_ = os.Remove(p)
		}
	})

	select {
	case final := <-done:
		if final.Status != StatusTimeout {
			t.Fatalf("expected status timeout, got %q (exit_code=%d signal=%q)", final.Status, final.ExitCode, final.Signal)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for execution to finish")
	}
}
