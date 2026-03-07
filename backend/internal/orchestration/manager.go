package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"vibe-tree/backend/internal/cliruntime"
	"vibe-tree/backend/internal/execution"
	"vibe-tree/backend/internal/executionflow"
	"vibe-tree/backend/internal/expert"
	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/store"
	"vibe-tree/backend/internal/workspace"
	"vibe-tree/backend/internal/ws"
)

type Manager struct {
	store          *store.Store
	executions     *execution.Manager
	experts        *expert.Registry
	hub            *ws.Hub
	maxConcurrency int
}

type Options struct {
	Store          *store.Store
	Executions     *execution.Manager
	Experts        *expert.Registry
	Hub            *ws.Hub
	MaxConcurrency int
}

// NewManager 功能：创建 orchestration 管理器，用于创建 orchestration 并调度 agent runs。
// 参数/返回：opts 注入 store/execution/expert/ws 依赖与并发上限；返回 *Manager。
// 失败场景：无（纯构造）。
// 副作用：无。
func NewManager(opts Options) *Manager {
	return &Manager{
		store:          opts.Store,
		executions:     opts.Executions,
		experts:        opts.Experts,
		hub:            opts.Hub,
		maxConcurrency: opts.MaxConcurrency,
	}
}

// Create 功能：根据用户 goal 创建一条 orchestration，并立即物化首轮 agent runs。
// 参数/返回：goal/workspace/title 为输入；返回完整 orchestration 详情。
// 失败场景：规划失败、store 未初始化或写库失败时返回 error。
// 副作用：写入 SQLite 并广播 orchestration/round/agent run 创建事件。
func (m *Manager) Create(ctx context.Context, title, goal, workspace string) (store.OrchestrationDetail, error) {
	if m == nil || m.store == nil {
		return store.OrchestrationDetail{}, fmt.Errorf("orchestration manager not configured")
	}
	plannedRound := limitPlannedRound(buildInitialRound(goal, workspace, m.experts), m.allowedPlanSize(ctx))
	orch, round, runs, err := m.store.CreateOrchestration(ctx, store.CreateOrchestrationParams{
		Title:         title,
		Goal:          goal,
		WorkspacePath: workspace,
		Round:         plannedRound,
	})
	if err != nil {
		return store.OrchestrationDetail{}, err
	}
	broadcastOrchestrationUpdated(m.hub, orch)
	broadcastRoundUpdated(m.hub, round)
	for _, run := range runs {
		broadcastAgentRunUpdated(m.hub, run)
	}
	return m.store.GetOrchestrationDetail(ctx, orch.ID)
}

// Continue 功能：在 waiting_continue 状态下生成下一轮 agent runs。
// 参数/返回：orchestrationID 为目标；返回更新后的 orchestration 详情。
// 失败场景：状态不允许、详情读取失败或写库失败时返回 error。
// 副作用：写入 SQLite 并广播新 round/agent runs。
func (m *Manager) Continue(ctx context.Context, orchestrationID string) (store.OrchestrationDetail, error) {
	if m == nil || m.store == nil {
		return store.OrchestrationDetail{}, fmt.Errorf("orchestration manager not configured")
	}
	detail, err := m.store.GetOrchestrationDetail(ctx, orchestrationID)
	if err != nil {
		return store.OrchestrationDetail{}, err
	}
	plannedRound := limitPlannedRound(buildNextRound(detail, m.experts), m.allowedPlanSize(ctx))
	orch, round, runs, err := m.store.ContinueOrchestration(ctx, orchestrationID, plannedRound)
	if err != nil {
		return store.OrchestrationDetail{}, err
	}
	broadcastOrchestrationUpdated(m.hub, orch)
	broadcastRoundUpdated(m.hub, round)
	for _, run := range runs {
		broadcastAgentRunUpdated(m.hub, run)
	}
	return m.store.GetOrchestrationDetail(ctx, orchestrationID)
}

// Tick 功能：推进一次 orchestration 调度，在可用并发额度内启动 queued agent runs。
// 参数/返回：ctx 控制本次调度超时；成功返回 nil。
// 失败场景：依赖缺失、查询失败或启动执行失败时返回 error。
// 副作用：可能启动 execution、更新 SQLite 并广播 WS 事件。
func (m *Manager) Tick(ctx context.Context) error {
	if m == nil || m.store == nil || m.executions == nil || m.experts == nil {
		return nil
	}
	if m.maxConcurrency <= 0 {
		return nil
	}
	running, err := m.store.CountRunningExecutionSlots(ctx)
	if err != nil {
		return err
	}
	slots := m.maxConcurrency - running
	if slots <= 0 {
		return nil
	}
	runs, err := m.store.ListRunnableQueuedAgentRuns(ctx, slots)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if err := m.startAgentRun(ctx, run); err != nil {
			logx.Warn("orchestration", "start-agent-run", "启动 agent run 失败", "err", err, "orchestration_id", run.OrchestrationID, "round_id", run.RoundID, "agent_run_id", run.ID)
		}
	}
	return nil
}

// Cancel 功能：取消指定 orchestration，并转发取消到底层 execution manager。
// 参数/返回：orchestrationID 为目标；返回更新后的 orchestration。
// 失败场景：状态不允许或 store/manager 失败时返回 error。
// 副作用：写入 SQLite，并向运行中的 execution 发送取消信号。
func (m *Manager) Cancel(ctx context.Context, orchestrationID string) (store.Orchestration, error) {
	if m == nil || m.store == nil {
		return store.Orchestration{}, fmt.Errorf("orchestration manager not configured")
	}
	res, err := m.store.CancelOrchestration(ctx, orchestrationID)
	if err != nil {
		return store.Orchestration{}, err
	}
	for _, executionID := range res.RunningExecutionIDs {
		if err := m.executions.Cancel(executionID); err != nil && !os.IsNotExist(err) {
			logx.Warn("orchestration", "cancel-execution", "取消 orchestration execution 失败", "execution_id", executionID, "err", err)
		}
	}
	broadcastOrchestrationUpdated(m.hub, res.Orchestration)
	return res.Orchestration, nil
}

// RetryAgentRun 功能：重试一个失败的 agent run，并等待后续 Tick 重新拉起 execution。
// 参数/返回：agentRunID 为目标；返回更新后的 orchestration/round/agent run。
// 失败场景：状态不允许或写库失败时返回 error。
// 副作用：更新 SQLite 并广播 WS 事件。
func (m *Manager) RetryAgentRun(ctx context.Context, agentRunID string) (store.Orchestration, store.OrchestrationRound, store.AgentRun, error) {
	if m == nil || m.store == nil {
		return store.Orchestration{}, store.OrchestrationRound{}, store.AgentRun{}, fmt.Errorf("orchestration manager not configured")
	}
	orch, round, run, err := m.store.RetryAgentRun(ctx, agentRunID)
	if err != nil {
		return store.Orchestration{}, store.OrchestrationRound{}, store.AgentRun{}, err
	}
	broadcastOrchestrationUpdated(m.hub, orch)
	broadcastRoundUpdated(m.hub, round)
	broadcastAgentRunUpdated(m.hub, run)
	return orch, round, run, nil
}

func (m *Manager) startAgentRun(ctx context.Context, run store.AgentRun) error {
	launchCtx, cancelLaunch := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelLaunch()
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	prepared, err := workspace.Prepare(launchCtx, run)
	if err != nil {
		return err
	}
	resolved, err := m.experts.Resolve(run.ExpertID, buildAgentPrompt(run), prepared.WorkspacePath)
	if err != nil {
		return err
	}
	artifactDir := ""
	artifacts := append([]store.AgentRunArtifactInput(nil), prepared.Artifacts...)
	if resolved.Provider == "cli" {
		if dir, err := cliruntime.AgentRunArtifactDir(run.OrchestrationID, run.ID); err == nil {
			artifactDir = dir
			resolved.Spec = cliruntime.PrepareRunSpec(resolved.Spec, dir)
			payload := mustJSON(map[string]any{"runtime_kind": resolved.RuntimeKind, "provider": resolved.Provider, "model": resolved.Model, "cli_family": resolved.CLIFamily, "artifact_dir": dir})
			summary := fmt.Sprintf("CLI runtime=%s family=%s artifact_dir=%s", resolved.RuntimeKind, resolved.CLIFamily, dir)
			artifacts = append(artifacts, store.AgentRunArtifactInput{Kind: "cli_runtime", Title: "CLI Runtime", Summary: &summary, PayloadJSON: &payload})
		}
	}

	updatedRun, _, err := m.store.UpdateAgentRunWorkspace(launchCtx, store.UpdateAgentRunWorkspaceParams{
		AgentRunID:    run.ID,
		WorkspaceMode: prepared.Mode,
		WorkspacePath: prepared.WorkspacePath,
		BranchName:    prepared.BranchName,
		BaseRef:       prepared.BaseRef,
		WorktreePath:  prepared.WorktreePath,
		Artifacts:     artifacts,
	})
	if err == nil {
		run = updatedRun
		broadcastAgentRunUpdated(m.hub, run)
	}
	if err != nil {
		return err
	}

	execCtx, cancelExec := executionflow.NewExecutionContext(resolved.Timeout)

	_, err = executionflow.StartRecordedExecution(execCtx, m.executions, resolved.Spec, execution.StartOptions{
		OrchestrationID: run.OrchestrationID,
		RoundID:         run.RoundID,
		AgentRunID:      run.ID,
		OnExit: func(final execution.Execution) {
			cancelExec()
			finalizeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			summary := executionflow.TailSummary(final.ID, 4000)
			if artifactDir != "" {
				if cliSummary := cliruntime.SummaryText(artifactDir); cliSummary != nil {
					summary = cliSummary
				}
			}
			inspection, _ := workspace.Inspect(finalizeCtx, run)
			if artifactDir != "" {
				if extra, err := cliruntime.CollectArtifacts(artifactDir); err == nil {
					inspection.Artifacts = append(inspection.Artifacts, extra...)
				}
			}
			orch, round, updatedRun, synthesis, _, err := m.store.FinalizeAgentRunExecution(finalizeCtx, store.FinalizeAgentRunExecutionParams{
				ExecutionID:     final.ID,
				OrchestrationID: final.OrchestrationID,
				RoundID:         final.RoundID,
				AgentRunID:      final.AgentRunID,
				Status:          string(final.Status),
				ExitCode:        final.ExitCode,
				Signal:          final.Signal,
				StartedAt:       final.StartedAt.UnixMilli(),
				EndedAt:         final.EndedAt.UnixMilli(),
				ErrorMessage:    executionflow.ErrorMessage(final),
				ResultSummary:   summary,
				ModifiedCode:    &inspection.ModifiedCode,
				Artifacts:       inspection.Artifacts,
			})
			if err != nil {
				logx.Warn("orchestration", "finalize-agent-run", "落库 orchestration execution 终态失败", "err", err, "orchestration_id", final.OrchestrationID, "round_id", final.RoundID, "agent_run_id", final.AgentRunID, "execution_id", final.ID)
				return
			}
			broadcastOrchestrationUpdated(m.hub, orch)
			broadcastRoundUpdated(m.hub, round)
			broadcastAgentRunUpdated(m.hub, updatedRun)
			if synthesis != nil {
				broadcastSynthesisUpdated(m.hub, *synthesis)
			}
		},
	}, func(exec execution.Execution) error {
		recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := m.store.StartAgentRunExecution(recordCtx, store.StartAgentRunExecutionParams{
			ExecutionID:     exec.ID,
			OrchestrationID: run.OrchestrationID,
			RoundID:         run.RoundID,
			AgentRunID:      run.ID,
			PID:             exec.PID,
			LogPath:         mustExecutionLogPath(exec.ID),
			StartedAt:       exec.StartedAt.UnixMilli(),
			Command:         exec.Command,
			Args:            exec.Args,
			Cwd:             exec.Cwd,
		})
		return err
	})
	if err != nil {
		cancelExec()
		return err
	}
	return nil
}

func buildInitialRound(goal, workspace string, experts *expert.Registry) store.PlannedRound {
	parts := splitGoal(goal)
	runs := make([]store.PlannedAgentRun, 0, len(parts))
	for _, part := range parts {
		role, intent := inferRoleAndIntent(part)
		expertID := chooseAgentExpert(experts, workspace, part)
		workspaceMode := defaultWorkspaceMode(intent)
		runs = append(runs, store.PlannedAgentRun{
			Role:          role,
			Title:         part,
			Goal:          part,
			ExpertID:      expertID,
			Intent:        intent,
			WorkspaceMode: workspaceMode,
			WorkspacePath: workspace,
		})
	}
	return store.PlannedRound{Goal: goal, AgentRuns: runs}
}

func buildNextRound(detail store.OrchestrationDetail, experts *expert.Registry) store.PlannedRound {
	if len(detail.Rounds) == 0 {
		return buildInitialRound(detail.Orchestration.Goal, detail.Orchestration.WorkspacePath, experts)
	}
	lastRound := detail.Rounds[len(detail.Rounds)-1]
	planned := store.PlannedRound{Goal: fmt.Sprintf("继续推进：%s", detail.Orchestration.Goal)}
	for _, run := range detail.AgentRuns {
		if run.RoundID != lastRound.ID {
			continue
		}
		nextIntent := "verify"
		nextRole := "verifier"
		nextGoal := fmt.Sprintf("验证上一轮结果：%s", run.Goal)
		switch run.Intent {
		case "analyze":
			nextIntent = "modify"
			nextRole = "implementer"
			nextGoal = fmt.Sprintf("根据上一轮分析继续实现：%s", run.Goal)
		case "modify":
			nextIntent = "verify"
			nextRole = "verifier"
			nextGoal = fmt.Sprintf("验证并总结上一轮修改：%s", run.Goal)
		case "verify":
			continue
		}
		expertID := chooseAgentExpert(experts, detail.Orchestration.WorkspacePath, nextGoal)
		planned.AgentRuns = append(planned.AgentRuns, store.PlannedAgentRun{
			Role:          nextRole,
			Title:         nextGoal,
			Goal:          nextGoal,
			ExpertID:      expertID,
			Intent:        nextIntent,
			WorkspaceMode: defaultWorkspaceMode(nextIntent),
			WorkspacePath: detail.Orchestration.WorkspacePath,
		})
	}
	if len(planned.AgentRuns) == 0 {
		goal := fmt.Sprintf("对上一轮结果做收尾与总结：%s", detail.Orchestration.Goal)
		planned.AgentRuns = append(planned.AgentRuns, store.PlannedAgentRun{
			Role:          "verifier",
			Title:         goal,
			Goal:          goal,
			ExpertID:      chooseAgentExpert(experts, detail.Orchestration.WorkspacePath, goal),
			Intent:        "verify",
			WorkspaceMode: defaultWorkspaceMode("verify"),
			WorkspacePath: detail.Orchestration.WorkspacePath,
		})
	}
	return planned
}

func splitGoal(goal string) []string {
	replacer := strings.NewReplacer("\n", "；", " and ", "；", "并且", "；", "以及", "；", "同时", "；", ",", "；", "，", "；", "、", "；")
	normalized := replacer.Replace(strings.TrimSpace(goal))
	rawParts := strings.Split(normalized, "；")
	out := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(goal)}
	}
	if len(out) > 4 {
		return out[:4]
	}
	return out
}

func inferRoleAndIntent(goal string) (string, string) {
	v := strings.ToLower(strings.TrimSpace(goal))
	switch {
	case strings.Contains(v, "分析") || strings.Contains(v, "差异") || strings.Contains(v, "compare") || strings.Contains(v, "audit"):
		return "analyzer", "analyze"
	case strings.Contains(v, "验证") || strings.Contains(v, "测试") || strings.Contains(v, "test") || strings.Contains(v, "check"):
		return "verifier", "verify"
	default:
		return "implementer", "modify"
	}
}

func chooseAgentExpert(reg *expert.Registry, workspace, prompt string) string {
	if reg == nil {
		return "demo"
	}
	candidates := []string{"codex", "claudecode", "demo"}
	known := reg.KnownIDs()
	for _, candidate := range candidates {
		if _, ok := known[candidate]; !ok {
			continue
		}
		if _, err := reg.Resolve(candidate, prompt, workspace); err == nil {
			return candidate
		}
	}
	ids := make([]string, 0, len(known))
	for id := range known {
		if id == "master" || id == "bash" {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, candidate := range ids {
		if _, err := reg.Resolve(candidate, prompt, workspace); err == nil {
			return candidate
		}
	}
	return "demo"
}

func buildAgentPrompt(run store.AgentRun) string {
	return fmt.Sprintf("你是 vibe-tree 的 %s agent。\n目标：%s\n工作目录：%s\n请输出简洁的执行摘要、关键发现以及下一步建议。", run.Role, run.Goal, run.WorkspacePath)
}

func mustExecutionLogPath(executionID string) string {
	path, err := executionLogPath(executionID)
	if err != nil {
		return executionID + ".log"
	}
	return path
}

func defaultWorkspaceMode(intent string) string {
	if strings.TrimSpace(intent) == "analyze" {
		return "read_only"
	}
	if strings.TrimSpace(intent) == "modify" {
		return "git_worktree"
	}
	return "shared_workspace"
}

func limitPlannedRound(round store.PlannedRound, maxConcurrency int) store.PlannedRound {
	if maxConcurrency <= 0 || len(round.AgentRuns) <= maxConcurrency {
		return round
	}
	round.AgentRuns = round.AgentRuns[:maxConcurrency]
	return round
}

func (m *Manager) allowedPlanSize(ctx context.Context) int {
	if m == nil || m.maxConcurrency <= 0 || m.store == nil {
		return 0
	}
	running, err := m.store.CountRunningExecutionSlots(ctx)
	if err != nil {
		return m.maxConcurrency
	}
	remaining := m.maxConcurrency - running
	if remaining <= 0 {
		return m.maxConcurrency
	}
	return remaining
}

func executionLogPath(executionID string) (string, error) {
	return paths.ExecutionLogPath(executionID)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func broadcastOrchestrationUpdated(hub *ws.Hub, orch store.Orchestration) {
	broadcast(hub, ws.Envelope{Type: "orchestration.updated", Ts: time.Now().UnixMilli(), OrchestrationID: orch.ID, Payload: orch})
}

func broadcastRoundUpdated(hub *ws.Hub, round store.OrchestrationRound) {
	broadcast(hub, ws.Envelope{Type: "orchestration.round.updated", Ts: time.Now().UnixMilli(), OrchestrationID: round.OrchestrationID, RoundID: round.ID, Payload: round})
}

func broadcastAgentRunUpdated(hub *ws.Hub, run store.AgentRun) {
	broadcast(hub, ws.Envelope{Type: "orchestration.agent_run.updated", Ts: time.Now().UnixMilli(), OrchestrationID: run.OrchestrationID, RoundID: run.RoundID, AgentRunID: run.ID, Payload: run})
}

func broadcastSynthesisUpdated(hub *ws.Hub, step store.SynthesisStep) {
	broadcast(hub, ws.Envelope{Type: "orchestration.synthesis.updated", Ts: time.Now().UnixMilli(), OrchestrationID: step.OrchestrationID, RoundID: step.RoundID, Payload: step})
}

func broadcast(hub *ws.Hub, env ws.Envelope) {
	if hub == nil {
		return
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	hub.Broadcast(b)
}
