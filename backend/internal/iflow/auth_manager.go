package iflow

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"vibe-tree/backend/internal/id"
	"vibe-tree/backend/internal/runner"
)

const (
	BrowserAuthStatusStarting     = "starting"
	BrowserAuthStatusAwaitingCode = "awaiting_code"
	BrowserAuthStatusVerifying    = "verifying"
	BrowserAuthStatusSucceeded    = "succeeded"
	BrowserAuthStatusFailed       = "failed"
	BrowserAuthStatusCanceled     = "canceled"
)

type BrowserAuthSession struct {
	SessionID     string `json:"session_id"`
	Status        string `json:"status"`
	AuthURL       string `json:"auth_url,omitempty"`
	LastOutput    string `json:"last_output,omitempty"`
	Error         string `json:"error,omitempty"`
	CanSubmitCode bool   `json:"can_submit_code"`
	Authenticated bool   `json:"authenticated"`
	CommandPath   string `json:"command_path,omitempty"`
	StartedAt     int64  `json:"started_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type BrowserAuthManager struct {
	runner  runner.Runner
	timeout time.Duration

	mu       sync.RWMutex
	sessions map[string]*browserAuthSession
}

type browserAuthSession struct {
	state        BrowserAuthSession
	handle       runner.ProcessHandle
	cancel       context.CancelFunc
	autoSelected bool
	output       []string
}

var (
	ansiPattern      = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	titlePattern     = regexp.MustCompile(`\x1b\][^\x07]*(?:\x07|\x1b\\)`)
	oauthURLPrefix   = "https://iflow.cn/oauth?"
	authPromptMarker = "How would you like to authenticate for this project?"
)

// NewBrowserAuthManager 功能：创建 iFlow 浏览器认证会话管理器。
// 参数/返回：runner 用于启动 PTY；返回 manager。
// 失败场景：无。
// 副作用：无。
func NewBrowserAuthManager(r runner.Runner) *BrowserAuthManager {
	if r == nil {
		r = runner.PTYRunner{DefaultGrace: 500 * time.Millisecond}
	}
	return &BrowserAuthManager{
		runner:   r,
		timeout:  10 * time.Minute,
		sessions: map[string]*browserAuthSession{},
	}
}

// Start 功能：启动一个新的 iFlow 官方网页登录会话。
// 参数/返回：commandPath 可覆盖 iflow 命令路径；返回会话快照。
// 失败场景：HOME/bootstrap/PTY 启动失败时返回 error。
// 副作用：创建 PTY 进程并开始后台读取输出。
func (m *BrowserAuthManager) Start(commandPath string) (BrowserAuthSession, error) {
	if m == nil {
		return BrowserAuthSession{}, fmt.Errorf("iflow auth manager is not configured")
	}
	homeDir, err := EnsureHome()
	if err != nil {
		return BrowserAuthSession{}, err
	}
	if strings.TrimSpace(commandPath) == "" {
		commandPath = "iflow"
	}
	ctx, cancel := context.WithCancel(context.Background())
	handle, err := m.runner.StartOneshot(ctx, runner.RunSpec{
		Command: strings.TrimSpace(commandPath),
		Cwd:     os.TempDir(),
		Env: map[string]string{
			"HOME":                  homeDir,
			"IFLOW_TRUST_WORKSPACE": "1",
		},
	})
	if err != nil {
		cancel()
		return BrowserAuthSession{}, err
	}
	now := time.Now().UnixMilli()
	session := &browserAuthSession{
		state: BrowserAuthSession{
			SessionID:   id.New("ifa_"),
			Status:      BrowserAuthStatusStarting,
			CommandPath: strings.TrimSpace(commandPath),
			StartedAt:   now,
			UpdatedAt:   now,
		},
		handle: handle,
		cancel: cancel,
		output: make([]string, 0, 24),
	}
	m.mu.Lock()
	m.sessions[session.state.SessionID] = session
	m.mu.Unlock()
	go m.watch(session)
	return session.state, nil
}

// Get 功能：读取指定网页登录会话状态。
// 参数/返回：sessionID 为会话 id；返回会话快照。
// 失败场景：会话不存在时返回 error。
// 副作用：无。
func (m *BrowserAuthManager) Get(sessionID string) (BrowserAuthSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return BrowserAuthSession{}, os.ErrNotExist
	}
	return session.state, nil
}

// SubmitCode 功能：向网页登录 PTY 提交授权码。
// 参数/返回：sessionID 为会话 id，code 为浏览器返回的授权码。
// 失败场景：会话不存在、授权码为空或写入失败时返回 error。
// 副作用：向 PTY 写入授权码并推进认证状态。
func (m *BrowserAuthManager) SubmitCode(sessionID, code string) (BrowserAuthSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return BrowserAuthSession{}, os.ErrNotExist
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return BrowserAuthSession{}, fmt.Errorf("authorization_code is required")
	}
	if _, err := session.handle.WriteInput([]byte(code + "\r")); err != nil {
		return BrowserAuthSession{}, err
	}
	session.state.Status = BrowserAuthStatusVerifying
	session.state.CanSubmitCode = false
	session.state.UpdatedAt = time.Now().UnixMilli()
	return session.state, nil
}

// Cancel 功能：取消指定网页登录会话。
// 参数/返回：sessionID 为会话 id；返回取消后的快照。
// 失败场景：会话不存在或取消失败时返回 error。
// 副作用：终止 PTY 进程。
func (m *BrowserAuthManager) Cancel(sessionID string) (BrowserAuthSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return BrowserAuthSession{}, os.ErrNotExist
	}
	session.cancel()
	if session.handle != nil {
		_ = session.handle.Cancel(200 * time.Millisecond)
	}
	session.state.Status = BrowserAuthStatusCanceled
	session.state.CanSubmitCode = false
	session.state.UpdatedAt = time.Now().UnixMilli()
	return session.state, nil
}

func (m *BrowserAuthManager) watch(session *browserAuthSession) {
	if session == nil || session.handle == nil {
		return
	}
	statusTicker := time.NewTicker(700 * time.Millisecond)
	defer statusTicker.Stop()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		output := session.handle.Output()
		if output == nil {
			return
		}
		defer output.Close()
		scanner := bufio.NewScanner(output)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 2*1024*1024)
		for scanner.Scan() {
			cleaned := cleanOutput(scanner.Text())
			if strings.TrimSpace(cleaned) == "" {
				continue
			}
			m.appendOutput(session, cleaned)
			m.maybeAdvance(session)
		}
	}()
	waitDone := make(chan struct{})
	var waitErr error
	go func() {
		defer close(waitDone)
		_, waitErr = session.handle.Wait()
	}()
	deadline := time.NewTimer(m.timeout)
	defer deadline.Stop()
	for {
		select {
		case <-statusTicker.C:
			status, err := DetectBrowserAuthStatus()
			if err == nil && status.Authenticated {
				m.finishSuccess(session)
				return
			}
		case <-deadline.C:
			m.finishFailure(session, "网页登录超时，请重试")
			return
		case <-finished:
			m.maybeAdvance(session)
		case <-waitDone:
			if state := m.snapshot(session.state.SessionID); state.Authenticated || state.Status == BrowserAuthStatusCanceled {
				return
			}
			if waitErr != nil {
				m.finishFailure(session, waitErr.Error())
				return
			}
			if state := m.snapshot(session.state.SessionID); state.AuthURL != "" {
				m.finishFailure(session, "网页登录已结束，但未检测到登录完成")
				return
			}
			m.finishFailure(session, "iFlow 登录会话提前退出")
			return
		}
	}
}

func (m *BrowserAuthManager) maybeAdvance(session *browserAuthSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	joined := strings.Join(session.output, "\n")
	if !session.autoSelected && strings.Contains(joined, authPromptMarker) {
		session.autoSelected = true
		_, _ = session.handle.WriteInput([]byte("\r"))
		session.state.UpdatedAt = time.Now().UnixMilli()
	}
	if session.state.AuthURL == "" {
		if url := ParseBrowserAuthURL(joined); url != "" {
			session.state.AuthURL = url
			session.state.Status = BrowserAuthStatusAwaitingCode
			session.state.CanSubmitCode = true
			session.state.UpdatedAt = time.Now().UnixMilli()
		}
	}
	if strings.Contains(strings.ToLower(joined), "authentication failed") || strings.Contains(strings.ToLower(joined), "failed to get api key") {
		session.state.Status = BrowserAuthStatusFailed
		session.state.Error = "iFlow 官方认证失败，请检查授权码后重试"
		session.state.CanSubmitCode = false
		session.state.UpdatedAt = time.Now().UnixMilli()
	}
}

func (m *BrowserAuthManager) appendOutput(session *browserAuthSession, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session.output = append(session.output, strings.TrimSpace(line))
	if len(session.output) > 40 {
		session.output = append([]string(nil), session.output[len(session.output)-40:]...)
	}
	session.state.LastOutput = strings.Join(session.output, "\n")
	session.state.UpdatedAt = time.Now().UnixMilli()
}

func (m *BrowserAuthManager) finishSuccess(session *browserAuthSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session.state.Status = BrowserAuthStatusSucceeded
	session.state.Authenticated = true
	session.state.CanSubmitCode = false
	session.state.UpdatedAt = time.Now().UnixMilli()
	session.cancel()
	if session.handle != nil {
		_ = session.handle.Cancel(200 * time.Millisecond)
	}
}

func (m *BrowserAuthManager) finishFailure(session *browserAuthSession, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session.state.Authenticated || session.state.Status == BrowserAuthStatusCanceled {
		return
	}
	session.state.Status = BrowserAuthStatusFailed
	session.state.Error = strings.TrimSpace(message)
	session.state.CanSubmitCode = false
	session.state.UpdatedAt = time.Now().UnixMilli()
	session.cancel()
	if session.handle != nil {
		_ = session.handle.Cancel(200 * time.Millisecond)
	}
}

func (m *BrowserAuthManager) snapshot(sessionID string) BrowserAuthSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return BrowserAuthSession{}
	}
	return session.state
}

// ParseBrowserAuthURL 功能：从 iFlow TUI 输出中重建被折行的官方 OAuth 链接。
// 参数/返回：text 为清洗后的终端文本；命中时返回完整 URL。
// 失败场景：未命中时返回空字符串。
// 副作用：无。
func ParseBrowserAuthURL(text string) string {
	text = cleanOutput(text)
	idx := strings.Index(text, oauthURLPrefix)
	if idx < 0 {
		return ""
	}
	tail := text[idx:]
	stop := len(tail)
	if marker := strings.Index(tail, "2. Login to your iFlow account and authorize"); marker >= 0 {
		stop = marker
	}
	candidate := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, tail[:stop])
	candidate = strings.TrimSpace(candidate)
	if !strings.HasPrefix(candidate, oauthURLPrefix) {
		return ""
	}
	return candidate
}

func cleanOutput(raw string) string {
	cleaned := titlePattern.ReplaceAllString(raw, "")
	cleaned = ansiPattern.ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	return cleaned
}
