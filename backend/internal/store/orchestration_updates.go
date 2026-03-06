package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"vibe-tree/backend/internal/id"
)

type UpdateAgentRunWorkspaceParams struct {
	AgentRunID      string
	WorkspaceMode   string
	WorkspacePath   string
	BranchName      *string
	BaseRef         *string
	WorktreePath    *string
	Artifacts       []AgentRunArtifactInput
}

// UpdateAgentRunWorkspace 功能：更新 agent run 的 workspace 元数据，并写入相关 artifact。
// 参数/返回：params 指定 workspace 字段与 artifact；返回更新后的 AgentRun 与新写入 artifact。
// 失败场景：agent run 不存在或写库失败时返回 error。
// 副作用：写入 SQLite agent_runs/orchestration_artifacts/orchestration_events。
func (s *Store) UpdateAgentRunWorkspace(ctx context.Context, params UpdateAgentRunWorkspaceParams) (AgentRun, []OrchestrationArtifact, error) {
	if s == nil || s.db == nil {
		return AgentRun{}, nil, fmt.Errorf("store not initialized")
	}
	if strings.TrimSpace(params.AgentRunID) == "" {
		return AgentRun{}, nil, fmt.Errorf("%w: agent_run_id is required", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AgentRun{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	run, err := getAgentRunTx(ctx, tx, params.AgentRunID)
	if err != nil {
		return AgentRun{}, nil, err
	}
	now := time.Now().UnixMilli()
	run.WorkspaceMode = strings.TrimSpace(params.WorkspaceMode)
	if run.WorkspaceMode == "" {
		run.WorkspaceMode = "shared_workspace"
	}
	run.WorkspacePath = strings.TrimSpace(params.WorkspacePath)
	if run.WorkspacePath == "" {
		run.WorkspacePath = "."
	}
	run.BranchName = params.BranchName
	run.BaseRef = params.BaseRef
	run.WorktreePath = params.WorktreePath
	run.UpdatedAt = now
	if _, err := tx.ExecContext(ctx, `UPDATE agent_runs SET workspace_mode = ?, workspace_path = ?, branch_name = ?, base_ref = ?, worktree_path = ?, updated_at = ? WHERE id = ?;`, run.WorkspaceMode, run.WorkspacePath, run.BranchName, run.BaseRef, run.WorktreePath, run.UpdatedAt, run.ID); err != nil {
		return AgentRun{}, nil, fmt.Errorf("update agent run workspace: %w", err)
	}
	artifacts, err := insertAgentRunArtifactsTx(ctx, tx, run.OrchestrationID, run.RoundID, run.ID, nil, now, params.Artifacts)
	if err != nil {
		return AgentRun{}, nil, err
	}
	if err := insertOrchestrationEvent(ctx, tx, run.OrchestrationID, "agent_run", run.ID, "orchestration.agent_run.updated", now, map[string]any{"action": "workspace.updated", "agent_run_id": run.ID, "workspace_mode": run.WorkspaceMode}); err != nil {
		return AgentRun{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return AgentRun{}, nil, fmt.Errorf("commit: %w", err)
	}
	return run, artifacts, nil
}

// ContinueOrchestration 功能：在 waiting_continue 状态下创建下一轮 round 与 agent runs。
// 参数/返回：orchestrationID 为目标，round 指定下一轮计划；返回更新后的 orchestration、round 与新 agent runs。
// 失败场景：状态不允许或写库失败时返回 error。
// 副作用：写入 SQLite orchestration_rounds/agent_runs/orchestration_events。
func (s *Store) ContinueOrchestration(ctx context.Context, orchestrationID string, round PlannedRound) (Orchestration, OrchestrationRound, []AgentRun, error) {
	if s == nil || s.db == nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("store not initialized")
	}
	if len(round.AgentRuns) == 0 {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("%w: next round requires at least one agent run", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	orch, err := getOrchestrationTx(ctx, tx, orchestrationID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, err
	}
	if orch.Status != string(OrchestrationStatusWaitingContinue) {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("%w: orchestration status %q cannot continue", ErrConflict, orch.Status)
	}
	now := time.Now().UnixMilli()
	nextRound := OrchestrationRound{
		ID:              id.New("rd_"),
		OrchestrationID: orch.ID,
		RoundIndex:      orch.CurrentRound + 1,
		Goal:            strings.TrimSpace(round.Goal),
		Status:          string(RoundStatusRunning),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if nextRound.Goal == "" {
		nextRound.Goal = orch.Goal
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO orchestration_rounds (id, orchestration_id, round_index, goal, status, created_at, updated_at, summary, synthesis_step_id) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, NULL);`, nextRound.ID, nextRound.OrchestrationID, nextRound.RoundIndex, nextRound.Goal, nextRound.Status, nextRound.CreatedAt, nextRound.UpdatedAt); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("insert continue round: %w", err)
	}
	runs := make([]AgentRun, 0, len(round.AgentRuns))
	for _, planned := range round.AgentRuns {
		run := AgentRun{ID: id.New("ar_"), OrchestrationID: orch.ID, RoundID: nextRound.ID, Role: strings.TrimSpace(planned.Role), Title: strings.TrimSpace(planned.Title), Goal: strings.TrimSpace(planned.Goal), ExpertID: strings.TrimSpace(planned.ExpertID), Intent: strings.TrimSpace(planned.Intent), WorkspaceMode: strings.TrimSpace(planned.WorkspaceMode), WorkspacePath: strings.TrimSpace(planned.WorkspacePath), Status: string(AgentRunStatusQueued), CreatedAt: now, UpdatedAt: now}
		if run.Role == "" {
			run.Role = "implementer"
		}
		if run.Title == "" {
			run.Title = run.Role
		}
		if run.Goal == "" {
			run.Goal = nextRound.Goal
		}
		if run.Intent == "" {
			run.Intent = "modify"
		}
		if run.WorkspaceMode == "" {
			run.WorkspaceMode = "shared_workspace"
		}
		if run.WorkspacePath == "" {
			run.WorkspacePath = orch.WorkspacePath
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO agent_runs (id, orchestration_id, round_id, role, title, goal, expert_id, intent, workspace_mode, workspace_path, branch_name, base_ref, worktree_path, status, created_at, updated_at, last_execution_id, result_summary, error_message, modified_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?, ?, NULL, NULL, NULL, 0);`, run.ID, run.OrchestrationID, run.RoundID, run.Role, run.Title, run.Goal, run.ExpertID, run.Intent, run.WorkspaceMode, run.WorkspacePath, run.Status, run.CreatedAt, run.UpdatedAt); err != nil {
			return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("insert continue agent run: %w", err)
		}
		runs = append(runs, run)
	}
	orch.Status = string(OrchestrationStatusRunning)
	orch.CurrentRound = nextRound.RoundIndex
	orch.UpdatedAt = now
	orch.ErrorMessage = nil
	if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET status = ?, current_round = ?, updated_at = ?, error_message = NULL WHERE id = ?;`, orch.Status, orch.CurrentRound, orch.UpdatedAt, orch.ID); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("update orchestration continue: %w", err)
	}
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "round", nextRound.ID, "orchestration.round.updated", now, map[string]any{"action": "created", "round_id": nextRound.ID, "round_index": nextRound.RoundIndex}); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, err
	}
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "orchestration", orch.ID, "orchestration.updated", now, map[string]any{"action": "continued", "status": orch.Status, "current_round": orch.CurrentRound}); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, err
	}
	for _, run := range runs {
		if err := insertOrchestrationEvent(ctx, tx, orch.ID, "agent_run", run.ID, "orchestration.agent_run.updated", now, map[string]any{"action": "created", "agent_run_id": run.ID, "status": run.Status}); err != nil {
			return Orchestration{}, OrchestrationRound{}, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("commit: %w", err)
	}
	return orch, nextRound, runs, nil
}

// GetAgentRun 功能：读取单个 agent run。
// 参数/返回：agentRunID 为目标 ID；返回 AgentRun。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetAgentRun(ctx context.Context, agentRunID string) (AgentRun, error) {
	if s == nil || s.db == nil {
		return AgentRun{}, fmt.Errorf("store not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AgentRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	return getAgentRunTx(ctx, tx, agentRunID)
}

// GetSynthesisStep 功能：读取单个 synthesis step。
// 参数/返回：synthesisStepID 为目标 ID；返回 SynthesisStep。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetSynthesisStep(ctx context.Context, synthesisStepID string) (SynthesisStep, error) {
	if s == nil || s.db == nil {
		return SynthesisStep{}, fmt.Errorf("store not initialized")
	}
	var step SynthesisStep
	err := s.db.QueryRowContext(ctx, `SELECT id, orchestration_id, round_id, decision, summary, created_at, updated_at FROM synthesis_steps WHERE id = ? LIMIT 1;`, synthesisStepID).Scan(&step.ID, &step.OrchestrationID, &step.RoundID, &step.Decision, &step.Summary, &step.CreatedAt, &step.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return SynthesisStep{}, os.ErrNotExist
		}
		return SynthesisStep{}, fmt.Errorf("query synthesis step: %w", err)
	}
	return step, nil
}

func insertAgentRunArtifactsTx(ctx context.Context, tx *sql.Tx, orchestrationID, roundID, agentRunID string, synthesisStepID *string, now int64, inputs []AgentRunArtifactInput) ([]OrchestrationArtifact, error) {
	artifacts := make([]OrchestrationArtifact, 0, len(inputs))
	for _, input := range inputs {
		kind := strings.TrimSpace(input.Kind)
		title := strings.TrimSpace(input.Title)
		if kind == "" || title == "" {
			continue
		}
		roundRef := pointerString(roundID)
		agentRef := pointerString(agentRunID)
		artifact := OrchestrationArtifact{ID: id.New("oa_"), OrchestrationID: orchestrationID, RoundID: roundRef, AgentRunID: agentRef, SynthesisStepID: synthesisStepID, Kind: kind, Title: title, Summary: input.Summary, PayloadJSON: input.PayloadJSON, CreatedAt: now}
		if _, err := tx.ExecContext(ctx, `INSERT INTO orchestration_artifacts (id, orchestration_id, round_id, agent_run_id, synthesis_step_id, kind, title, summary, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, artifact.ID, artifact.OrchestrationID, artifact.RoundID, artifact.AgentRunID, artifact.SynthesisStepID, artifact.Kind, artifact.Title, artifact.Summary, artifact.PayloadJSON, artifact.CreatedAt); err != nil {
			return nil, fmt.Errorf("insert orchestration artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

