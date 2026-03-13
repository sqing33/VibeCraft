package store

import (
	"context"
	"testing"
	"time"

	"vibecraft/backend/internal/id"
)

func TestRecoverOrchestrationsAfterRestart_MarksRunningAgentRunsFailed(t *testing.T) {
	st := openTestStore(t)
	orch, round, runs, err := st.CreateOrchestration(context.Background(), CreateOrchestrationParams{
		Title:         "recover-orch",
		Goal:          "analyze and change",
		WorkspacePath: ".",
		Round: PlannedRound{
			Goal:      "round-1",
			AgentRuns: []PlannedAgentRun{{Role: "implementer", Title: "change", Goal: "change", ExpertID: "demo", Intent: "modify", WorkspaceMode: "shared_workspace", WorkspacePath: "."}},
		},
	})
	if err != nil {
		t.Fatalf("create orchestration: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one agent run, got %d", len(runs))
	}
	run := runs[0]
	now := time.Now().UnixMilli()
	if _, err := st.StartAgentRunExecution(context.Background(), StartAgentRunExecutionParams{
		ExecutionID:     id.New("ex_"),
		OrchestrationID: orch.ID,
		RoundID:         round.ID,
		AgentRunID:      run.ID,
		PID:             123,
		LogPath:         "test.log",
		StartedAt:       now,
		Command:         "bash",
		Args:            []string{"-lc", "sleep 10"},
		Cwd:             ".",
	}); err != nil {
		t.Fatalf("start agent run execution: %v", err)
	}

	updated, err := st.RecoverOrchestrationsAfterRestart(context.Background())
	if err != nil {
		t.Fatalf("recover orchestrations after restart: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 recovered execution, got %d", updated)
	}

	gotOrch, err := st.GetOrchestration(context.Background(), orch.ID)
	if err != nil {
		t.Fatalf("get orchestration: %v", err)
	}
	if gotOrch.Status != string(OrchestrationStatusFailed) {
		t.Fatalf("expected orchestration failed, got %q", gotOrch.Status)
	}
	gotRun, err := st.GetAgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get agent run: %v", err)
	}
	if gotRun.Status != string(AgentRunStatusFailed) {
		t.Fatalf("expected agent run failed, got %q", gotRun.Status)
	}
	detail, err := st.GetOrchestrationDetail(context.Background(), orch.ID)
	if err != nil {
		t.Fatalf("get orchestration detail: %v", err)
	}
	if len(detail.Rounds) != 1 || detail.Rounds[0].Status != string(RoundStatusRetryable) {
		t.Fatalf("expected retryable round after recovery, got %+v", detail.Rounds)
	}
}
