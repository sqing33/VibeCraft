package orchestration

import (
	"testing"

	"vibe-tree/backend/internal/store"
)

func TestBuildNextRound_ReplansLatestRoundAgentRuns(t *testing.T) {
	detail := store.OrchestrationDetail{
		Orchestration: store.Orchestration{
			Goal:          "同时推进登录页和设置页",
			WorkspacePath: "/repo",
		},
		Rounds: []store.OrchestrationRound{
			{ID: "rd_1", RoundIndex: 1},
		},
		AgentRuns: []store.AgentRun{
			{ID: "ar_a", RoundID: "rd_1", Intent: "analyze", Goal: "分析登录页差异"},
			{ID: "ar_b", RoundID: "rd_1", Intent: "modify", Goal: "重构设置页"},
			{ID: "ar_c", RoundID: "rd_1", Intent: "verify", Goal: "验证现有改动"},
			{ID: "ar_old", RoundID: "rd_old", Intent: "modify", Goal: "旧轮次结果"},
		},
	}

	next := buildNextRound(detail, nil)
	if next.Goal == "" {
		t.Fatalf("expected next round goal")
	}
	if len(next.AgentRuns) != 2 {
		t.Fatalf("expected 2 next-round agent runs, got %d", len(next.AgentRuns))
	}

	first := next.AgentRuns[0]
	if first.Role != "implementer" || first.Intent != "modify" {
		t.Fatalf("expected analyze -> implementer/modify, got %+v", first)
	}
	if first.WorkspaceMode != "git_worktree" || first.WorkspacePath != "/repo" {
		t.Fatalf("expected modify run to use git_worktree in same workspace, got %+v", first)
	}

	second := next.AgentRuns[1]
	if second.Role != "verifier" || second.Intent != "verify" {
		t.Fatalf("expected modify -> verifier/verify, got %+v", second)
	}
	if second.WorkspaceMode != "shared_workspace" || second.WorkspacePath != "/repo" {
		t.Fatalf("expected verify run to use shared workspace, got %+v", second)
	}
	for _, run := range next.AgentRuns {
		if run.Goal == "验证现有改动" || run.Goal == "旧轮次结果" {
			t.Fatalf("expected planner to ignore verify-only and non-latest-round runs, got %+v", next.AgentRuns)
		}
	}
}
