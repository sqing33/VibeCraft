package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/executionflow"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/ws"
)

type Scheduler struct {
	mu sync.Mutex

	store          *store.Store
	executions     *execution.Manager
	experts        *expert.Registry
	hub            *ws.Hub
	maxConcurrency int
}

type Options struct {
	Store          *store.Store
	Executions     *execution.Manager
	Hub            *ws.Hub
	Experts        *expert.Registry
	MaxConcurrency int
}

// New 功能：创建 workflow 调度器（MVP：只调度 queued worker nodes，遵循依赖与全局并发上限）。
// 参数/返回：opts 注入 store/execution manager/hub 与最大并发；返回 Scheduler。
// 失败场景：无（缺依赖会在 Tick 中返回 error）。
// 副作用：无（纯构造）。
func New(opts Options) *Scheduler {
	return &Scheduler{
		store:          opts.Store,
		executions:     opts.Executions,
		experts:        opts.Experts,
		hub:            opts.Hub,
		maxConcurrency: opts.MaxConcurrency,
	}
}

// Tick 功能：推进一次调度：在并发额度内启动 runnable queued worker nodes。
// 参数/返回：ctx 控制超时；成功返回 nil。
// 失败场景：依赖缺失、读取/写入 DB 失败或启动进程失败时返回 error。
// 副作用：可能启动子进程、写日志文件、写入 SQLite 并广播 WS 事件。
func (s *Scheduler) Tick(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		return fmt.Errorf("store not configured")
	}
	if s.executions == nil {
		return fmt.Errorf("execution manager not configured")
	}
	if s.maxConcurrency <= 0 {
		return fmt.Errorf("invalid max_concurrency=%d", s.maxConcurrency)
	}

	running, err := s.store.CountRunningWorkerNodes(ctx)
	if err != nil {
		return err
	}
	slots := s.maxConcurrency - running
	if slots <= 0 {
		return nil
	}

	nodes, err := s.store.ListRunnableQueuedWorkerNodes(ctx, slots)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if err := s.startNode(ctx, n); err != nil {
			// 启动失败直接 fail-fast（MVP）。
			logx.Warn("workflow-scheduler", "start-node", "启动节点失败，标记 workflow/node failed", "err", err, "workflow_id", n.WorkflowID, "node_id", n.ID)
			_ = s.store.MarkNodeAndWorkflowFailed(context.Background(), n.WorkflowID, n.ID, err.Error())
		}
	}
	return nil
}

func (s *Scheduler) startNode(ctx context.Context, n store.RunnableNode) error {
	if s.experts == nil {
		return fmt.Errorf("expert registry not configured")
	}

	resolved, err := s.experts.Resolve(n.ExpertID, n.Prompt, n.WorkspacePath)
	if err != nil {
		return err
	}

	execCtx, cancelExec := executionflow.NewExecutionContext(resolved.Timeout)
	_, err = executionflow.StartRecordedExecution(execCtx, s.executions, resolved.Spec, execution.StartOptions{
		WorkflowID: n.WorkflowID,
		NodeID:     n.ID,
		OnExit: func(final execution.Execution) {
			cancelExec()

			finalErr := executionflow.ErrorMessage(final)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			endedAt := time.Now().UnixMilli()
			if final.EndedAt != nil {
				endedAt = final.EndedAt.UnixMilli()
			}
			startedAt := final.StartedAt.UnixMilli()
			summary := executionflow.TailSummary(final.ID, 4000)

			updatedWf, updatedNodes, err := s.store.FinalizeExecution(ctx, store.FinalizeExecutionParams{
				ExecutionID:   final.ID,
				WorkflowID:    final.WorkflowID,
				NodeID:        final.NodeID,
				Status:        string(final.Status),
				ExitCode:      final.ExitCode,
				Signal:        final.Signal,
				StartedAt:     startedAt,
				EndedAt:       endedAt,
				ErrorMessage:  finalErr,
				ResultSummary: summary,
			})
			if err != nil {
				logx.Warn("workflow-scheduler", "finalize-execution", "execution 终态落库失败", "err", err, "workflow_id", final.WorkflowID, "node_id", final.NodeID, "execution_id", final.ID)
				return
			}

			broadcastWorkflowUpdated(s.hub, updatedWf)
			for _, nn := range updatedNodes {
				broadcastNodeUpdated(s.hub, nn)
			}
		},
	}, func(exec execution.Execution) error {
		_, err := s.store.StartExecution(ctx, store.StartExecutionParams{
			ExecutionID: exec.ID,
			WorkflowID:  n.WorkflowID,
			NodeID:      n.ID,
			Attempt:     0,
			PID:         exec.PID,
			LogPath:     mustSchedulerExecutionLogPath(exec.ID),
			StartedAt:   exec.StartedAt.UnixMilli(),
			Command:     exec.Command,
			Args:        exec.Args,
			Cwd:         exec.Cwd,
		})
		return err
	})
	if err != nil {
		cancelExec()
		return err
	}
	if updatedNode, err := s.store.GetNode(ctx, n.ID); err == nil {
		broadcastNodeUpdated(s.hub, updatedNode)
	}
	return nil
}

func mustSchedulerExecutionLogPath(executionID string) string {
	path, err := paths.ExecutionLogPath(executionID)
	if err != nil {
		return executionID + ".log"
	}
	return path
}

func broadcastWorkflowUpdated(hub *ws.Hub, wf store.Workflow) {
	if hub == nil {
		return
	}
	env := ws.Envelope{
		Type:       "workflow.updated",
		Ts:         time.Now().UnixMilli(),
		WorkflowID: wf.ID,
		Payload:    wf,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	hub.Broadcast(b)
}

func broadcastNodeUpdated(hub *ws.Hub, node store.Node) {
	if hub == nil {
		return
	}
	env := ws.Envelope{
		Type:       "node.updated",
		Ts:         time.Now().UnixMilli(),
		WorkflowID: node.WorkflowID,
		NodeID:     node.ID,
		Payload:    node,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	hub.Broadcast(b)
}
