package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// App struct
type App struct {
	ctx context.Context

	daemonURL    string
	daemonCmd    *exec.Cmd
	daemonCancel context.CancelFunc
	daemonExited chan error
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{daemonExited: make(chan error, 1)}
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx

	host, port := resolveDaemonHostPort()
	a.daemonURL = fmt.Sprintf("http://%s:%d", host, port)

	if err := a.ensureDaemonRunning(host, port); err != nil {
		fmt.Printf("start daemon failed: %v\n", err)
	}
}

// domReady is called after front-end resources have been loaded
func (a App) domReady(ctx context.Context) {
	// Add your action here
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.stopDaemon()
}

// DaemonURL 功能：返回 desktop 启动/复用的 daemon base URL。
// 参数/返回：无入参；返回形如 `http://127.0.0.1:7777` 的字符串。
// 失败场景：无（未就绪时返回默认 URL）。
// 副作用：无。
func (a *App) DaemonURL() string {
	if strings.TrimSpace(a.daemonURL) == "" {
		return "http://127.0.0.1:7777"
	}
	return a.daemonURL
}

// WaitForDaemon 功能：等待 daemon health 可达（供前端 boot 流程使用）。
// 参数/返回：timeoutMs 为等待超时毫秒数；返回 daemon 是否可达。
// 失败场景：timeout 内 health 始终不可达时返回 false。
// 副作用：对 daemon 发起 health 请求。
func (a *App) WaitForDaemon(timeoutMs int) bool {
	timeout := 30 * time.Second
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	deadline := time.Now().Add(timeout)
	url := a.DaemonURL()
	for time.Now().Before(deadline) {
		if healthOK(url, 500*time.Millisecond) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return healthOK(url, 800*time.Millisecond)
}

// OpenDataDir 功能：在系统文件管理器中打开 vibe-tree 数据目录。
// 参数/返回：无入参；成功返回数据目录路径；失败返回 error。
// 失败场景：无法解析 home 目录、或系统打开命令执行失败时返回 error。
// 副作用：启动系统命令（xdg-open/open/explorer）。
func (a *App) OpenDataDir() (string, error) {
	dataDir, err := resolveDataDir()
	if err != nil {
		return "", err
	}
	return dataDir, openInFileManager(dataDir)
}

func resolveDataDir() (string, error) {
	xdgDataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if xdgDataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		xdgDataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdgDataHome, "vibe-tree"), nil
}

func openInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Run()
	case "windows":
		return exec.Command("explorer", path).Run()
	default:
		return exec.Command("xdg-open", path).Run()
	}
}

func resolveDaemonHostPort() (host string, port int) {
	host = strings.TrimSpace(os.Getenv("VIBE_TREE_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}

	port = 7777
	if raw := strings.TrimSpace(os.Getenv("VIBE_TREE_PORT")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 65535 {
			port = v
		}
	}

	return host, port
}

func (a *App) ensureDaemonRunning(host string, port int) error {
	url := fmt.Sprintf("http://%s:%d", host, port)
	if healthOK(url, 250*time.Millisecond) {
		return nil
	}

	daemonPath := strings.TrimSpace(os.Getenv("VIBE_TREE_DAEMON_PATH"))
	if daemonPath == "" {
		daemonPath = "vibe-tree-daemon"
	}

	daemonCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(daemonCtx, daemonPath)
	cmd.Env = append(os.Environ(),
		"VIBE_TREE_HOST="+host,
		fmt.Sprintf("VIBE_TREE_PORT=%d", port),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}

	a.daemonCmd = cmd
	a.daemonCancel = cancel
	go func() {
		a.daemonExited <- cmd.Wait()
	}()

	return nil
}

func (a *App) stopDaemon() {
	if a.daemonCancel == nil || a.daemonCmd == nil {
		return
	}

	a.daemonCancel()

	select {
	case <-a.daemonExited:
	case <-time.After(2 * time.Second):
		_ = a.daemonCmd.Process.Kill()
		<-a.daemonExited
	}
}

func healthOK(daemonURL string, timeout time.Duration) bool {
	client := http.Client{Timeout: timeout}
	res, err := client.Get(daemonURL + "/api/v1/health")
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 300
}
