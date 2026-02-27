package runner

import (
	"context"
	"io"
	"time"
)

type RunSpec struct {
	Command string
	Args    []string
	Env     map[string]string
	Cwd     string
}

type ExitResult struct {
	ExitCode  int
	Signal    string
	StartedAt time.Time
	EndedAt   time.Time
}

type ProcessHandle interface {
	PID() int
	Output() io.ReadCloser
	Wait() (ExitResult, error)
	Cancel(grace time.Duration) error
	WriteInput(p []byte) (int, error)
	Close() error
}

type Runner interface {
	StartOneshot(ctx context.Context, spec RunSpec) (ProcessHandle, error)
}
