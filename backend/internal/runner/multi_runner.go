package runner

import (
	"context"
	"errors"
)

// MultiRunner 功能：按 RunSpec 类型路由到 PTY（进程）或 SDK runner。
// 参数/返回：spec.SDK!=nil 时走 SDK；否则走进程 runner；返回 ProcessHandle 与错误。
// 失败场景：对应 runner 未配置或下游启动失败时返回 error。
// 副作用：可能启动子进程或发起外部 SDK 网络请求。
type MultiRunner struct {
	Process Runner
	SDK     Runner
}

func (r MultiRunner) StartOneshot(ctx context.Context, spec RunSpec) (ProcessHandle, error) {
	if spec.SDK != nil {
		if r.SDK == nil {
			return nil, errors.New("sdk runner not configured")
		}
		return r.SDK.StartOneshot(ctx, spec)
	}
	if r.Process == nil {
		return nil, errors.New("process runner not configured")
	}
	return r.Process.StartOneshot(ctx, spec)
}
