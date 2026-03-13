package executionflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vibecraft/backend/internal/execution"
	"vibecraft/backend/internal/runner"
)

// StartRecordedExecution 功能：统一启动 execution，并在记录启动信息失败时自动取消该 execution。
// 参数/返回：ctx/spec/opts 控制 execution 启动；recordStart 负责把 execution 关联写入 store；返回 execution 元数据。
// 失败场景：execution 启动失败或 recordStart 返回 error 时返回 error。
// 副作用：可能启动 execution，并在 recordStart 失败时触发取消。
func StartRecordedExecution(
	ctx context.Context,
	executions *execution.Manager,
	spec runner.RunSpec,
	opts execution.StartOptions,
	recordStart func(exec execution.Execution) error,
) (execution.Execution, error) {
	if executions == nil {
		return execution.Execution{}, fmt.Errorf("execution manager not configured")
	}
	exec, err := executions.StartOneshotWithOptions(ctx, spec, opts)
	if err != nil {
		return execution.Execution{}, err
	}
	if recordStart == nil {
		return exec, nil
	}
	if err := recordStart(exec); err != nil {
		_ = executions.Cancel(exec.ID)
		return execution.Execution{}, err
	}
	return exec, nil
}

// NewExecutionContext 功能：按超时约束创建 execution 上下文。
// 参数/返回：timeout<=0 时返回 background context；否则返回带 timeout 的 context 与 cancel。
// 失败场景：无。
// 副作用：无。
func NewExecutionContext(timeout time.Duration) (context.Context, func()) {
	if timeout > 0 {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.Background(), func() {}
}

// ErrorMessage 功能：根据 execution 终态生成统一错误文案。
// 参数/返回：exec 为终态 execution；成功返回错误文案或空串。
// 失败场景：无。
// 副作用：无。
func ErrorMessage(exec execution.Execution) string {
	switch exec.Status {
	case execution.StatusFailed:
		if exec.Signal != "" {
			return fmt.Sprintf("signal=%s exit_code=%d", exec.Signal, exec.ExitCode)
		}
		return fmt.Sprintf("exit_code=%d", exec.ExitCode)
	case execution.StatusCanceled:
		return "canceled"
	case execution.StatusTimeout:
		return "timeout"
	default:
		return ""
	}
}

// TailSummary 功能：读取 execution 日志尾部作为结果摘要。
// 参数/返回：executionID/tailBytes 控制读取内容；读取失败或为空时返回 nil。
// 失败场景：无（内部吞掉错误，返回 nil）。
// 副作用：读取磁盘日志文件。
func TailSummary(executionID string, tailBytes int64) *string {
	b, err := execution.ReadLogTail(executionID, tailBytes)
	if err != nil || len(b) == 0 {
		return nil
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return nil
	}
	return &s
}
