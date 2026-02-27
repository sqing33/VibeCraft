package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type PTYRunner struct {
	DefaultGrace time.Duration
}

// StartOneshot 功能：以 PTY 模式启动一个一次性进程并返回可取消/可等待的句柄。
// 参数/返回：接收执行规格 spec；返回 ProcessHandle 与错误信息。
// 失败场景：命令为空、子进程启动失败或 PTY 初始化失败时返回 error。
// 副作用：启动子进程并创建 PTY 设备；ctx 取消时会触发默认 grace 的取消逻辑。
func (r PTYRunner) StartOneshot(ctx context.Context, spec RunSpec) (ProcessHandle, error) {
	if strings.TrimSpace(spec.Command) == "" {
		return nil, errors.New("command is required")
	}

	cmd := exec.Command(spec.Command, spec.Args...)
	if spec.Cwd != "" {
		cmd.Dir = spec.Cwd
	}
	cmd.Env = mergeEnv(os.Environ(), spec.Env)

	startedAt := time.Now()
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	h := &ptyProcessHandle{
		cmd:          cmd,
		ptyFile:      ptyFile,
		startedAt:    startedAt,
		defaultGrace: r.DefaultGrace,
		waitCh:       make(chan struct{}),
	}

	go h.waitAndFinalize()

	if ctx.Done() != nil {
		go func() {
			<-ctx.Done()
			_ = h.Cancel(h.defaultGrace)
		}()
	}

	return h, nil
}

type ptyProcessHandle struct {
	cmd          *exec.Cmd
	ptyFile      *os.File
	startedAt    time.Time
	defaultGrace time.Duration

	mu              sync.Mutex
	cancelRequested bool

	waitOnce sync.Once
	waitCh   chan struct{}
	exitRes  ExitResult
	waitErr  error
}

func (h *ptyProcessHandle) PID() int {
	if h.cmd.Process == nil {
		return 0
	}
	return h.cmd.Process.Pid
}

func (h *ptyProcessHandle) Output() io.ReadCloser {
	return h.ptyFile
}

func (h *ptyProcessHandle) WriteInput(p []byte) (int, error) {
	if h.ptyFile == nil {
		return 0, errors.New("pty is not available")
	}
	return h.ptyFile.Write(p)
}

func (h *ptyProcessHandle) Close() error {
	if h.ptyFile == nil {
		return nil
	}
	return h.ptyFile.Close()
}

func (h *ptyProcessHandle) Wait() (ExitResult, error) {
	<-h.waitCh
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.exitRes, h.waitErr
}

func (h *ptyProcessHandle) Cancel(grace time.Duration) error {
	h.mu.Lock()
	h.cancelRequested = true
	h.mu.Unlock()

	if grace <= 0 {
		grace = 0
	}

	if err := h.signal(syscall.SIGTERM); err != nil {
		return err
	}

	if grace > 0 {
		select {
		case <-h.waitCh:
			return nil
		case <-time.After(grace):
		}
	}

	return h.signal(syscall.SIGKILL)
}

func (h *ptyProcessHandle) waitAndFinalize() {
	h.waitOnce.Do(func() {
		err := h.cmd.Wait()
		endedAt := time.Now()

		exitCode, signal := exitInfoFromProcessState(h.cmd.ProcessState)
		exitRes := ExitResult{
			ExitCode:  exitCode,
			Signal:    signal,
			StartedAt: h.startedAt,
			EndedAt:   endedAt,
		}

		h.mu.Lock()
		h.exitRes = exitRes
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				h.waitErr = err
			}
		}
		h.mu.Unlock()

		close(h.waitCh)
	})
}

func (h *ptyProcessHandle) signal(sig syscall.Signal) error {
	if h.cmd.Process == nil {
		return errors.New("process not started")
	}

	pid := h.cmd.Process.Pid
	if pid == 0 {
		return errors.New("invalid pid")
	}

	if err := syscall.Kill(-pid, sig); err == nil {
		return nil
	} else if errors.Is(err, syscall.ESRCH) {
		return nil
	}

	if err := h.cmd.Process.Signal(sig); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	return nil
}

func exitInfoFromProcessState(ps *os.ProcessState) (exitCode int, signal string) {
	if ps == nil {
		return -1, ""
	}

	if ws, ok := ps.Sys().(syscall.WaitStatus); ok {
		if ws.Signaled() {
			sig := ws.Signal()
			return 128 + int(sig), sig.String()
		}
		if ws.Exited() {
			return ws.ExitStatus(), ""
		}
	}

	code := ps.ExitCode()
	return code, ""
}

func mergeEnv(base []string, override map[string]string) []string {
	if len(override) == 0 {
		return base
	}

	envMap := make(map[string]string, len(base)+len(override))
	for _, kv := range base {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		envMap[k] = v
	}
	for k, v := range override {
		envMap[k] = v
	}

	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+envMap[k])
	}
	return out
}
