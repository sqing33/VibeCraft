package execution

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vibe-tree/backend/internal/id"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/runner"
	"vibe-tree/backend/internal/ws"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
	StatusTimeout   Status = "timeout"
)

type Execution struct {
	ID         string     `json:"execution_id"`
	WorkflowID string     `json:"workflow_id,omitempty"`
	NodeID     string     `json:"node_id,omitempty"`
	Status     Status     `json:"status"`
	Command    string     `json:"command"`
	Args       []string   `json:"args,omitempty"`
	Cwd        string     `json:"cwd,omitempty"`
	PID        int        `json:"pid,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	ExitCode   int        `json:"exit_code,omitempty"`
	Signal     string     `json:"signal,omitempty"`
}

type Manager struct {
	mu           sync.Mutex
	runner       runner.Runner
	defaultGrace time.Duration
	hub          *ws.Hub
	executions   map[string]*executionState
}

type StartOptions struct {
	WorkflowID string
	NodeID     string
	OnExit     func(exec Execution)
}

type executionState struct {
	exec            Execution
	logPath         string
	handle          runner.ProcessHandle
	cancelRequested bool
	onExit          func(exec Execution)
}

// NewManager 功能：创建执行管理器，用于启动/取消 execution 并驱动日志落盘与 WS 推送。
// 参数/返回：r 为 Runner；defaultGrace 为默认取消宽限期；hub 可为空（不推送 WS）。
// 失败场景：无（纯构造）。
// 副作用：无。
func NewManager(r runner.Runner, defaultGrace time.Duration, hub *ws.Hub) *Manager {
	return &Manager{
		runner:       r,
		defaultGrace: defaultGrace,
		hub:          hub,
		executions:   make(map[string]*executionState),
	}
}

// StartOneshot 功能：启动一次性 execution（PTY 模式）并异步落盘日志/推送事件。
// 参数/返回：ctx 控制执行生命周期；spec 为命令规格；返回 Execution 元数据与错误信息。
// 失败场景：日志路径解析失败、日志文件创建失败或进程启动失败时返回 error。
// 副作用：创建日志文件、启动子进程、启动 goroutine 进行日志流处理与状态收敛。
func (m *Manager) StartOneshot(ctx context.Context, spec runner.RunSpec) (Execution, error) {
	return m.StartOneshotWithOptions(ctx, spec, StartOptions{})
}

// StartOneshotWithOptions 功能：启动一次性 execution，并可携带 workflow/node 上下文与退出回调（用于落库/状态收敛）。
// 参数/返回：opts 可选提供 WorkflowID/NodeID 与 OnExit 回调；返回 Execution 元数据与错误信息。
// 失败场景：同 StartOneshot。
// 副作用：同 StartOneshot，并在退出时调用 opts.OnExit（如提供）。
func (m *Manager) StartOneshotWithOptions(ctx context.Context, spec runner.RunSpec, opts StartOptions) (Execution, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	executionID := id.New("ex_")
	logPath, err := paths.ExecutionLogPath(executionID)
	if err != nil {
		return Execution{}, err
	}
	if err := paths.EnsureDir(filepath.Dir(logPath)); err != nil {
		return Execution{}, err
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Execution{}, err
	}

	handle, err := m.runner.StartOneshot(ctx, spec)
	if err != nil {
		_ = logFile.Close()
		return Execution{}, err
	}

	exec := Execution{
		ID:         executionID,
		WorkflowID: opts.WorkflowID,
		NodeID:     opts.NodeID,
		Status:     StatusRunning,
		Command:    spec.Command,
		Args:       spec.Args,
		Cwd:        spec.Cwd,
		PID:        handle.PID(),
		StartedAt:  time.Now(),
	}

	st := &executionState{
		exec:    exec,
		logPath: logPath,
		handle:  handle,
		onExit:  opts.OnExit,
	}

	m.mu.Lock()
	m.executions[executionID] = st
	m.mu.Unlock()

	m.broadcast(ws.Envelope{
		Type:        "execution.started",
		Ts:          time.Now().UnixMilli(),
		WorkflowID:  exec.WorkflowID,
		NodeID:      exec.NodeID,
		ExecutionID: executionID,
		Payload: executionStartedPayload{
			Command: exec.Command,
			Args:    exec.Args,
			Cwd:     exec.Cwd,
		},
	})

	go m.streamToLogAndFinalize(ctx, executionID, st, logFile, handle)

	return exec, nil
}

// Cancel 功能：请求取消指定 execution（SIGTERM→grace→SIGKILL）。
// 参数/返回：executionID 为目标；成功返回 nil。
// 失败场景：execution 不存在或已结束时返回 os.ErrNotExist。
// 副作用：向子进程发送信号；最终状态会通过 execution.exited 事件收敛。
func (m *Manager) Cancel(executionID string) error {
	m.mu.Lock()
	st, ok := m.executions[executionID]
	if !ok || st.handle == nil {
		m.mu.Unlock()
		return os.ErrNotExist
	}
	st.cancelRequested = true
	handle := st.handle
	grace := m.defaultGrace
	m.mu.Unlock()

	return handle.Cancel(grace)
}

// Get 功能：读取当前内存态 execution 元数据（用于调试/展示）。
// 参数/返回：返回 Execution 与是否存在。
// 失败场景：无（未命中返回 false）。
// 副作用：无。
func (m *Manager) Get(executionID string) (Execution, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.executions[executionID]
	if !ok {
		return Execution{}, false
	}
	return st.exec, true
}

// streamToLogAndFinalize 功能：消费 PTY 输出，写入日志文件并推送 node.log，最终收敛 execution 状态并推送 execution.exited。
// 参数/返回：ctx 控制超时/取消语义；接收 executionID、状态指针、logFile 与 handle；无返回值。
// 失败场景：读写错误会导致日志提前结束，但仍会等待进程退出并尽力收敛状态。
// 副作用：写文件、推送 WS、关闭文件句柄、回收 handle。
func (m *Manager) streamToLogAndFinalize(ctx context.Context, executionID string, st *executionState, logFile *os.File, handle runner.ProcessHandle) {
	defer func() {
		_ = logFile.Close()
		_ = handle.Close()
	}()

	writer := bufio.NewWriterSize(logFile, 64*1024)
	lastFlush := time.Now()

	workflowID := st.exec.WorkflowID
	nodeID := st.exec.NodeID

	output := handle.Output()
	buf := make([]byte, 4096)
	for {
		n, err := output.Read(buf)
		if n > 0 {
			_, _ = writer.Write(buf[:n])
			now := time.Now()
			if now.Sub(lastFlush) >= 100*time.Millisecond {
				_ = writer.Flush()
				lastFlush = now
			}
			m.broadcast(ws.Envelope{
				Type:        "node.log",
				Ts:          time.Now().UnixMilli(),
				WorkflowID:  workflowID,
				NodeID:      nodeID,
				ExecutionID: executionID,
				Payload: nodeLogPayload{
					Chunk: string(buf[:n]),
				},
			})
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
	}

	_ = writer.Flush()

	exitRes, waitErr := handle.Wait()

	ctxErr := error(nil)
	if ctx != nil {
		ctxErr = ctx.Err()
	}

	endedAt := exitRes.EndedAt
	status := StatusFailed
	m.mu.Lock()
	if st.cancelRequested {
		status = StatusCanceled
	} else if errors.Is(ctxErr, context.DeadlineExceeded) {
		status = StatusTimeout
	} else if errors.Is(ctxErr, context.Canceled) {
		status = StatusCanceled
	} else if waitErr == nil && exitRes.ExitCode == 0 {
		status = StatusSucceeded
	}
	st.exec.Status = status
	st.exec.StartedAt = exitRes.StartedAt
	st.exec.EndedAt = &endedAt
	st.exec.ExitCode = exitRes.ExitCode
	st.exec.Signal = exitRes.Signal
	st.handle = nil
	onExit := st.onExit
	finalExec := st.exec
	m.mu.Unlock()

	if onExit != nil {
		func() {
			defer func() {
				_ = recover()
			}()
			onExit(finalExec)
		}()
	}

	durationMs := exitRes.EndedAt.Sub(exitRes.StartedAt).Milliseconds()
	m.broadcast(ws.Envelope{
		Type:        "execution.exited",
		Ts:          time.Now().UnixMilli(),
		WorkflowID:  workflowID,
		NodeID:      nodeID,
		ExecutionID: executionID,
		Payload: executionExitedPayload{
			Status:     string(status),
			ExitCode:   exitRes.ExitCode,
			Signal:     exitRes.Signal,
			DurationMs: durationMs,
		},
	})
}

type nodeLogPayload struct {
	Chunk string `json:"chunk"`
}

type executionStartedPayload struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Cwd     string   `json:"cwd,omitempty"`
}

type executionExitedPayload struct {
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Signal     string `json:"signal,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

func (m *Manager) broadcast(env ws.Envelope) {
	if m.hub == nil {
		return
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	m.hub.Broadcast(b)
}
