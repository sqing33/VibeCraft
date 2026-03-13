package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"vibecraft/backend/internal/id"
)

func TestCreateOrchestration_MaterializesFirstRound(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	orch, round, runs, err := st.CreateOrchestration(ctx, CreateOrchestrationParams{
		Title:         "project orchestration",
		Goal:          "同时分析聊天页并重构设置页",
		WorkspacePath: ".",
		Round: PlannedRound{
			Goal: "round-1",
			AgentRuns: []PlannedAgentRun{
				{Role: "analyzer", Title: "分析聊天页", Goal: "分析聊天页", ExpertID: "demo", Intent: "analyze", WorkspaceMode: "read_only", WorkspacePath: "."},
				{Role: "implementer", Title: "重构设置页", Goal: "重构设置页", ExpertID: "demo", Intent: "modify", WorkspaceMode: "git_worktree", WorkspacePath: "."},
			},
		},
	})
	if err != nil {
		t.Fatalf("create orchestration: %v", err)
	}
	if orch.Status != string(OrchestrationStatusRunning) {
		t.Fatalf("expected orchestration running, got %q", orch.Status)
	}
	if orch.CurrentRound != 1 {
		t.Fatalf("expected current round 1, got %d", orch.CurrentRound)
	}
	if round.RoundIndex != 1 || round.Status != string(RoundStatusRunning) {
		t.Fatalf("unexpected first round: %+v", round)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 agent runs, got %d", len(runs))
	}
	for _, run := range runs {
		if run.Status != string(AgentRunStatusQueued) {
			t.Fatalf("expected queued agent run, got %+v", run)
		}
		if run.OrchestrationID != orch.ID || run.RoundID != round.ID {
			t.Fatalf("unexpected orchestration linkage: %+v", run)
		}
	}

	detail, err := st.GetOrchestrationDetail(ctx, orch.ID)
	if err != nil {
		t.Fatalf("get orchestration detail: %v", err)
	}
	if len(detail.Rounds) != 1 {
		t.Fatalf("expected 1 round, got %d", len(detail.Rounds))
	}
	if len(detail.AgentRuns) != 2 {
		t.Fatalf("expected 2 agent runs in detail, got %d", len(detail.AgentRuns))
	}
	if len(detail.SynthesisSteps) != 0 {
		t.Fatalf("expected no synthesis before execution, got %d", len(detail.SynthesisSteps))
	}

	queued, err := st.ListRunnableQueuedAgentRuns(ctx, 10)
	if err != nil {
		t.Fatalf("list runnable queued agent runs: %v", err)
	}
	if len(queued) != 2 {
		t.Fatalf("expected 2 queued runs, got %d", len(queued))
	}
}

func TestFinalizeAgentRunExecution_UsesRoundBarrierBeforeContinueSynthesis(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	orch, round, runs := mustCreateOrchestrationForTest(t, st, PlannedRound{
		Goal: "round-1",
		AgentRuns: []PlannedAgentRun{
			{Role: "analyzer", Title: "分析聊天页", Goal: "分析聊天页", ExpertID: "demo", Intent: "analyze", WorkspaceMode: "read_only", WorkspacePath: "."},
			{Role: "implementer", Title: "改进工作流页", Goal: "改进工作流页", ExpertID: "demo", Intent: "modify", WorkspaceMode: "git_worktree", WorkspacePath: "."},
		},
	})

	startedAt := time.Now().UnixMilli()
	exec1 := mustStartAgentRunExecutionForTest(t, st, orch.ID, round.ID, runs[0].ID, startedAt)
	exec2 := mustStartAgentRunExecutionForTest(t, st, orch.ID, round.ID, runs[1].ID, startedAt+1)

	analysisSummary := "完成聊天页差异分析"
	orchAfterFirst, roundAfterFirst, runAfterFirst, synthesis, _, err := st.FinalizeAgentRunExecution(ctx, FinalizeAgentRunExecutionParams{
		ExecutionID:     exec1,
		OrchestrationID: orch.ID,
		RoundID:         round.ID,
		AgentRunID:      runs[0].ID,
		Status:          string(AgentRunStatusSucceeded),
		ExitCode:        0,
		StartedAt:       startedAt,
		EndedAt:         startedAt + 10,
		ResultSummary:   &analysisSummary,
		Artifacts: []AgentRunArtifactInput{
			{Kind: "analysis_summary", Title: "分析摘要", Summary: &analysisSummary},
		},
	})
	if err != nil {
		t.Fatalf("finalize first agent run: %v", err)
	}
	if synthesis != nil {
		t.Fatalf("expected no synthesis before round barrier, got %+v", synthesis)
	}
	if orchAfterFirst.Status != string(OrchestrationStatusRunning) {
		t.Fatalf("expected orchestration still running, got %q", orchAfterFirst.Status)
	}
	if roundAfterFirst.Status != string(RoundStatusRunning) {
		t.Fatalf("expected round still running, got %q", roundAfterFirst.Status)
	}
	if runAfterFirst.Status != string(AgentRunStatusSucceeded) {
		t.Fatalf("expected first run succeeded, got %q", runAfterFirst.Status)
	}

	detailBeforeBarrier, err := st.GetOrchestrationDetail(ctx, orch.ID)
	if err != nil {
		t.Fatalf("get detail before barrier: %v", err)
	}
	if len(detailBeforeBarrier.SynthesisSteps) != 0 {
		t.Fatalf("expected no synthesis before round barrier, got %d", len(detailBeforeBarrier.SynthesisSteps))
	}

	modifySummary := "已完成工作流页调整"
	modified := true
	orchAfterBarrier, roundAfterBarrier, runAfterBarrier, synthesis, artifacts, err := st.FinalizeAgentRunExecution(ctx, FinalizeAgentRunExecutionParams{
		ExecutionID:     exec2,
		OrchestrationID: orch.ID,
		RoundID:         round.ID,
		AgentRunID:      runs[1].ID,
		Status:          string(AgentRunStatusSucceeded),
		ExitCode:        0,
		StartedAt:       startedAt + 1,
		EndedAt:         startedAt + 20,
		ResultSummary:   &modifySummary,
		ModifiedCode:    &modified,
		Artifacts: []AgentRunArtifactInput{
			{Kind: "code_change_summary", Title: "代码变更", Summary: &modifySummary},
		},
	})
	if err != nil {
		t.Fatalf("finalize second agent run: %v", err)
	}
	if synthesis == nil {
		t.Fatalf("expected synthesis after round barrier")
	}
	if synthesis.Decision != string(SynthesisDecisionContinue) {
		t.Fatalf("expected continue synthesis, got %q", synthesis.Decision)
	}
	if orchAfterBarrier.Status != string(OrchestrationStatusWaitingContinue) {
		t.Fatalf("expected waiting_continue, got %q", orchAfterBarrier.Status)
	}
	if roundAfterBarrier.Status != string(RoundStatusDone) {
		t.Fatalf("expected round done, got %q", roundAfterBarrier.Status)
	}
	if !runAfterBarrier.ModifiedCode {
		t.Fatalf("expected modify run to mark modified_code")
	}
	if !hasArtifactKind(artifacts, "code_change_summary") || !hasArtifactKind(artifacts, "synthesis_summary") {
		t.Fatalf("expected code_change_summary and synthesis_summary artifacts, got %+v", artifacts)
	}

	detailAfterBarrier, err := st.GetOrchestrationDetail(ctx, orch.ID)
	if err != nil {
		t.Fatalf("get detail after barrier: %v", err)
	}
	if len(detailAfterBarrier.SynthesisSteps) != 1 {
		t.Fatalf("expected 1 synthesis step, got %d", len(detailAfterBarrier.SynthesisSteps))
	}
	if detailAfterBarrier.Rounds[0].SynthesisStepID == nil {
		t.Fatalf("expected round to reference synthesis step")
	}
}

func TestContinueOrchestration_CreatesNextRound(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	orch, round, runs := mustCreateOrchestrationForTest(t, st, PlannedRound{
		Goal: "round-1",
		AgentRuns: []PlannedAgentRun{
			{Role: "analyzer", Title: "分析登录页", Goal: "分析登录页", ExpertID: "demo", Intent: "analyze", WorkspaceMode: "read_only", WorkspacePath: "."},
		},
	})

	startedAt := time.Now().UnixMilli()
	execID := mustStartAgentRunExecutionForTest(t, st, orch.ID, round.ID, runs[0].ID, startedAt)
	summary := "登录页分析完成"
	orch, round, _, synthesis, _, err := st.FinalizeAgentRunExecution(ctx, FinalizeAgentRunExecutionParams{
		ExecutionID:     execID,
		OrchestrationID: orch.ID,
		RoundID:         round.ID,
		AgentRunID:      runs[0].ID,
		Status:          string(AgentRunStatusSucceeded),
		ExitCode:        0,
		StartedAt:       startedAt,
		EndedAt:         startedAt + 10,
		ResultSummary:   &summary,
	})
	if err != nil {
		t.Fatalf("finalize first round: %v", err)
	}
	if synthesis == nil || synthesis.Decision != string(SynthesisDecisionContinue) {
		t.Fatalf("expected continue synthesis, got %+v", synthesis)
	}
	if orch.Status != string(OrchestrationStatusWaitingContinue) {
		t.Fatalf("expected waiting_continue before continue, got %q", orch.Status)
	}

	nextGoal := "根据登录页分析继续实现"
	orch, nextRound, nextRuns, err := st.ContinueOrchestration(ctx, orch.ID, PlannedRound{
		Goal: nextGoal,
		AgentRuns: []PlannedAgentRun{
			{Role: "implementer", Title: nextGoal, Goal: nextGoal, ExpertID: "demo", Intent: "modify", WorkspaceMode: "git_worktree", WorkspacePath: "."},
		},
	})
	if err != nil {
		t.Fatalf("continue orchestration: %v", err)
	}
	if orch.Status != string(OrchestrationStatusRunning) {
		t.Fatalf("expected running after continue, got %q", orch.Status)
	}
	if orch.CurrentRound != 2 {
		t.Fatalf("expected current round 2, got %d", orch.CurrentRound)
	}
	if nextRound.RoundIndex != 2 || nextRound.Status != string(RoundStatusRunning) {
		t.Fatalf("unexpected next round: %+v", nextRound)
	}
	if len(nextRuns) != 1 || nextRuns[0].Status != string(AgentRunStatusQueued) {
		t.Fatalf("unexpected next round runs: %+v", nextRuns)
	}

	detail, err := st.GetOrchestrationDetail(ctx, orch.ID)
	if err != nil {
		t.Fatalf("get orchestration detail after continue: %v", err)
	}
	if len(detail.Rounds) != 2 {
		t.Fatalf("expected 2 rounds after continue, got %d", len(detail.Rounds))
	}
	if len(detail.SynthesisSteps) != 1 {
		t.Fatalf("expected round-1 synthesis to remain queryable, got %d", len(detail.SynthesisSteps))
	}
}

func TestRetryAgentRun_PreservesExecutionHistoryAndReplacesSynthesis(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	orch, round, runs := mustCreateOrchestrationForTest(t, st, PlannedRound{
		Goal: "round-1",
		AgentRuns: []PlannedAgentRun{
			{Role: "verifier", Title: "验证改动", Goal: "验证改动", ExpertID: "demo", Intent: "verify", WorkspaceMode: "shared_workspace", WorkspacePath: "."},
		},
	})

	startedAt := time.Now().UnixMilli()
	firstExecID := mustStartAgentRunExecutionForTest(t, st, orch.ID, round.ID, runs[0].ID, startedAt)
	orch, round, run, synthesis, _, err := st.FinalizeAgentRunExecution(ctx, FinalizeAgentRunExecutionParams{
		ExecutionID:     firstExecID,
		OrchestrationID: orch.ID,
		RoundID:         round.ID,
		AgentRunID:      runs[0].ID,
		Status:          string(AgentRunStatusFailed),
		ExitCode:        1,
		StartedAt:       startedAt,
		EndedAt:         startedAt + 10,
		ErrorMessage:    "verification failed",
	})
	if err != nil {
		t.Fatalf("finalize failed agent run: %v", err)
	}
	if synthesis == nil || synthesis.Decision != string(SynthesisDecisionNeedsRetry) {
		t.Fatalf("expected needs_retry synthesis, got %+v", synthesis)
	}
	if orch.Status != string(OrchestrationStatusFailed) || round.Status != string(RoundStatusRetryable) {
		t.Fatalf("unexpected failed orchestration state: orch=%+v round=%+v", orch, round)
	}
	if run.Status != string(AgentRunStatusFailed) {
		t.Fatalf("expected failed agent run, got %q", run.Status)
	}
	assertAgentRunAttemptStats(t, st, run.ID, 1, 1)

	orch, round, run, err = st.RetryAgentRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("retry agent run: %v", err)
	}
	if orch.Status != string(OrchestrationStatusRunning) {
		t.Fatalf("expected running after retry, got %q", orch.Status)
	}
	if round.Status != string(RoundStatusRunning) || round.SynthesisStepID != nil {
		t.Fatalf("expected running round without synthesis after retry, got %+v", round)
	}
	if run.Status != string(AgentRunStatusQueued) || run.ResultSummary != nil || run.ErrorMessage != nil {
		t.Fatalf("expected queued retry run with cleared summaries, got %+v", run)
	}

	detailAfterRetry, err := st.GetOrchestrationDetail(ctx, orch.ID)
	if err != nil {
		t.Fatalf("get orchestration detail after retry: %v", err)
	}
	if len(detailAfterRetry.SynthesisSteps) != 0 {
		t.Fatalf("expected failed synthesis removed after retry, got %d", len(detailAfterRetry.SynthesisSteps))
	}

	secondStartedAt := startedAt + 20
	secondExecID := mustStartAgentRunExecutionForTest(t, st, orch.ID, round.ID, run.ID, secondStartedAt)
	assertAgentRunAttemptStats(t, st, run.ID, 2, 2)
	verifySummary := "验证通过"
	orch, round, run, synthesis, _, err = st.FinalizeAgentRunExecution(ctx, FinalizeAgentRunExecutionParams{
		ExecutionID:     secondExecID,
		OrchestrationID: orch.ID,
		RoundID:         round.ID,
		AgentRunID:      run.ID,
		Status:          string(AgentRunStatusSucceeded),
		ExitCode:        0,
		StartedAt:       secondStartedAt,
		EndedAt:         secondStartedAt + 10,
		ResultSummary:   &verifySummary,
	})
	if err != nil {
		t.Fatalf("finalize retried agent run: %v", err)
	}
	if synthesis == nil || synthesis.Decision != string(SynthesisDecisionComplete) {
		t.Fatalf("expected complete synthesis after retry success, got %+v", synthesis)
	}
	if orch.Status != string(OrchestrationStatusDone) || round.Status != string(RoundStatusDone) {
		t.Fatalf("expected done orchestration after retry success, got orch=%+v round=%+v", orch, round)
	}
	if run.Status != string(AgentRunStatusSucceeded) {
		t.Fatalf("expected succeeded run after retry, got %q", run.Status)
	}
}

func mustCreateOrchestrationForTest(t *testing.T, st *Store, round PlannedRound) (Orchestration, OrchestrationRound, []AgentRun) {
	t.Helper()
	orch, createdRound, runs, err := st.CreateOrchestration(context.Background(), CreateOrchestrationParams{
		Title:         "test orchestration",
		Goal:          "推进项目开发",
		WorkspacePath: ".",
		Round:         round,
	})
	if err != nil {
		t.Fatalf("create orchestration: %v", err)
	}
	return orch, createdRound, runs
}

func mustStartAgentRunExecutionForTest(t *testing.T, st *Store, orchestrationID, roundID, agentRunID string, startedAt int64) string {
	t.Helper()
	execID := id.New("ex_")
	if _, err := st.StartAgentRunExecution(context.Background(), StartAgentRunExecutionParams{
		ExecutionID:     execID,
		OrchestrationID: orchestrationID,
		RoundID:         roundID,
		AgentRunID:      agentRunID,
		PID:             100,
		LogPath:         filepath.Join(t.TempDir(), execID+".log"),
		StartedAt:       startedAt,
		Command:         "bash",
		Args:            []string{"-lc", "echo test"},
		Cwd:             ".",
	}); err != nil {
		t.Fatalf("start agent run execution: %v", err)
	}
	return execID
}

func hasArtifactKind(artifacts []OrchestrationArtifact, kind string) bool {
	for _, artifact := range artifacts {
		if artifact.Kind == kind {
			return true
		}
	}
	return false
}

func assertAgentRunAttemptStats(t *testing.T, st *Store, agentRunID string, wantCount, wantMaxAttempt int) {
	t.Helper()
	var gotCount int
	var gotMaxAttempt int
	if err := st.db.QueryRowContext(context.Background(), `SELECT COUNT(*), COALESCE(MAX(attempt), 0) FROM agent_run_executions WHERE agent_run_id = ?;`, agentRunID).Scan(&gotCount, &gotMaxAttempt); err != nil {
		t.Fatalf("query agent run attempts: %v", err)
	}
	if gotCount != wantCount || gotMaxAttempt != wantMaxAttempt {
		t.Fatalf("unexpected attempt stats: count=%d max_attempt=%d", gotCount, gotMaxAttempt)
	}
}
