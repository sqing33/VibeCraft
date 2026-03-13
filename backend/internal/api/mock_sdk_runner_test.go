package api_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"vibecraft/backend/internal/execution"
	"vibecraft/backend/internal/runner"
	"vibecraft/backend/internal/ws"
)

type mockSDKRunner struct{}

type mockProcessRunner struct {
	fallback runner.PTYRunner
}

func (r mockProcessRunner) StartOneshot(ctx context.Context, spec runner.RunSpec) (runner.ProcessHandle, error) {
	if spec.SDK != nil {
		return nil, errors.New("mockProcessRunner requires non-SDK spec")
	}
	joined := strings.Join(spec.Args, " ")
	if strings.Contains(joined, "codex_exec.sh") || strings.Contains(joined, "claude_exec.sh") || strings.TrimSpace(spec.Env["VIBECRAFT_CLI_FAMILY"]) != "" {
		runCtx, cancel := context.WithCancel(ctx)
		pr, pw := io.Pipe()
		h := &mockPipeHandle{ctx: runCtx, cancel: cancel, outR: pr, outW: pw, startedAt: time.Now(), done: make(chan struct{})}
		go h.run(func() error {
			prompt := strings.TrimSpace(spec.Env["VIBECRAFT_PROMPT"])
			output := "mock cli: " + prompt
			if strings.TrimSpace(spec.Env["VIBECRAFT_OUTPUT_SCHEMA"]) == "dag_v1" || strings.Contains(strings.ToLower(spec.Env["VIBECRAFT_SYSTEM_PROMPT"]), "single json object") {
				output = `{
  "workflow_title": "",
  "nodes": [
    {
      "id": "n1",
      "title": "Step 1",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock cli runner",
      "prompt": "echo '[n1] hello'; sleep 0.02; echo '[n1] done'"
    },
    {
      "id": "n2",
      "title": "Step 2",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock cli runner",
      "prompt": "echo '[n2] hello'; sleep 0.02; echo '[n2] done'"
    },
    {
      "id": "n3",
      "title": "Step 3",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock cli runner",
      "prompt": "echo '[n3] hello'; sleep 0.02; echo '[n3] done'"
    }
  ],
  "edges": [
    { "from": "n1", "to": "n2", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n2", "to": "n3", "type": "success", "source_handle": null, "target_handle": null }
  ]
}`
			}
			_, err := io.WriteString(pw, output)
			return err
		})
		return h, nil
	}
	return r.fallback.StartOneshot(ctx, spec)
}

func (r mockSDKRunner) StartOneshot(ctx context.Context, spec runner.RunSpec) (runner.ProcessHandle, error) {
	if spec.SDK == nil {
		return nil, errors.New("mockSDKRunner requires spec.SDK")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithCancel(ctx)
	pr, pw := io.Pipe()

	h := &mockPipeHandle{
		ctx:       runCtx,
		cancel:    cancel,
		outR:      pr,
		outW:      pw,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}

	go h.run(func() error {
		// 输出一个稳定 DAG，避免测试依赖真实网络/密钥。
		_, err := io.WriteString(pw, `{
  "workflow_title": "",
  "nodes": [
    {
      "id": "n1",
      "title": "Step 1",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock sdk runner",
      "prompt": "echo '[n1] hello'; sleep 0.02; echo '[n1] done'"
    },
    {
      "id": "n2",
      "title": "Step 2",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock sdk runner",
      "prompt": "echo '[n2] hello'; sleep 0.02; echo '[n2] done'"
    },
    {
      "id": "n3",
      "title": "Step 3",
      "type": "worker",
      "expert_id": "bash",
      "fallback_expert_id": "bash",
      "complexity": "low",
      "quality_tier": "fast",
      "model": null,
      "routing_reason": "mock sdk runner",
      "prompt": "echo '[n3] hello'; sleep 0.02; echo '[n3] done'"
    }
  ],
  "edges": [
    { "from": "n1", "to": "n2", "type": "success", "source_handle": null, "target_handle": null },
    { "from": "n2", "to": "n3", "type": "success", "source_handle": null, "target_handle": null }
  ]
}
`)
		return err
	})

	return h, nil
}

type mockPipeHandle struct {
	ctx    context.Context
	cancel context.CancelFunc

	outR *io.PipeReader
	outW *io.PipeWriter

	startedAt time.Time

	done chan struct{}

	finishOnce sync.Once
	mu         sync.Mutex
	exitRes    runner.ExitResult
	waitErr    error
}

func (h *mockPipeHandle) PID() int { return 0 }

func (h *mockPipeHandle) Output() io.ReadCloser { return h.outR }

func (h *mockPipeHandle) Wait() (runner.ExitResult, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitRes, h.waitErr
}

func (h *mockPipeHandle) Cancel(_ time.Duration) error {
	if h.cancel != nil {
		h.cancel()
	}
	h.finish(0, context.Canceled)
	return nil
}

func (h *mockPipeHandle) WriteInput(_ []byte) (int, error) { return 0, io.ErrClosedPipe }

func (h *mockPipeHandle) Close() error {
	if h.cancel != nil {
		h.cancel()
	}
	h.finish(0, nil)
	if h.outR != nil {
		_ = h.outR.Close()
	}
	return nil
}

func (h *mockPipeHandle) run(fn func() error) {
	err := fn()
	exitCode := 0
	if err != nil {
		exitCode = 1
	}
	h.finish(exitCode, err)
}

func (h *mockPipeHandle) finish(exitCode int, err error) {
	h.finishOnce.Do(func() {
		if h.outW != nil {
			_ = h.outW.CloseWithError(err)
		}

		h.mu.Lock()
		h.exitRes = runner.ExitResult{
			ExitCode:  exitCode,
			Signal:    "",
			StartedAt: h.startedAt,
			EndedAt:   time.Now(),
		}
		h.waitErr = err
		h.mu.Unlock()

		close(h.done)
	})
}

func newTestExecMgr(grace time.Duration, hub *ws.Hub) *execution.Manager {
	execRunner := runner.MultiRunner{
		Process: mockProcessRunner{fallback: runner.PTYRunner{DefaultGrace: grace}},
		SDK:     mockSDKRunner{},
	}
	return execution.NewManager(execRunner, grace, hub)
}
