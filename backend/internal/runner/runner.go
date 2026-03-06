package runner

import (
	"context"
	"io"
	"time"
)

type RunSpec struct {
	// Command/Args/Env/Cwd 功能：本地进程（PTY）执行字段。
	// 参数/返回：Command 为空时视为无效 spec。
	// 失败场景：进程启动失败、权限不足或命令不存在时返回 error。
	// 副作用：启动子进程并产生输出流（由 Runner 决定）。
	Command string
	Args    []string
	Env     map[string]string
	Cwd     string

	// SDK 功能：当不为空时表示走 SDK 驱动（OpenAI/Anthropic/Demo），不启动外部 CLI。
	// 参数/返回：Provider/Model/Prompt 等字段由 expert.Resolve 填充。
	// 失败场景：provider 不支持、鉴权失败或网络错误时返回 error。
	// 副作用：可能发起外部网络请求并产生输出流。
	SDK *SDKSpec
	// SDKFallbacks 功能：当主 SDK 请求失败时按顺序重试候选模型。
	// 参数/返回：仅对 SDK spec 生效；每个 fallback 携带独立 SDK 参数与敏感 env。
	// 失败场景：fallback provider/model 缺失或全部尝试失败时由调用方收敛错误。
	// 副作用：可能触发额外一次或多次网络请求。
	SDKFallbacks []SDKFallback
}

type SDKFallback struct {
	SDK SDKSpec
	Env map[string]string
}

type SDKSpec struct {
	Provider        string
	Model           string
	LLMModelID      string
	Prompt          string
	Instructions    string
	BaseURL         string
	MaxOutputTokens int
	Temperature     *float64
	OutputSchema    string
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
