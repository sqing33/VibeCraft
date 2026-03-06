package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"vibe-tree/backend/internal/id"
)

type OrchestrationStatus string

const (
	OrchestrationStatusPlanning        OrchestrationStatus = "planning"
	OrchestrationStatusRunning         OrchestrationStatus = "running"
	OrchestrationStatusWaitingContinue OrchestrationStatus = "waiting_continue"
	OrchestrationStatusDone            OrchestrationStatus = "done"
	OrchestrationStatusFailed          OrchestrationStatus = "failed"
	OrchestrationStatusCanceled        OrchestrationStatus = "canceled"
)

type RoundStatus string

const (
	RoundStatusRunning   RoundStatus = "running"
	RoundStatusDone      RoundStatus = "done"
	RoundStatusFailed    RoundStatus = "failed"
	RoundStatusCanceled  RoundStatus = "canceled"
	RoundStatusRetryable RoundStatus = "retryable"
)

type AgentRunStatus string

const (
	AgentRunStatusQueued    AgentRunStatus = "queued"
	AgentRunStatusRunning   AgentRunStatus = "running"
	AgentRunStatusSucceeded AgentRunStatus = "succeeded"
	AgentRunStatusFailed    AgentRunStatus = "failed"
	AgentRunStatusCanceled  AgentRunStatus = "canceled"
	AgentRunStatusTimeout   AgentRunStatus = "timeout"
)

type SynthesisDecision string

const (
	SynthesisDecisionComplete   SynthesisDecision = "complete"
	SynthesisDecisionContinue   SynthesisDecision = "continue"
	SynthesisDecisionNeedsRetry SynthesisDecision = "needs_retry"
)

type Orchestration struct {
	ID                    string  `json:"orchestration_id"`
	Title                 string  `json:"title"`
	Goal                  string  `json:"goal"`
	WorkspacePath         string  `json:"workspace_path"`
	Status                string  `json:"status"`
	CurrentRound          int     `json:"current_round"`
	CreatedAt             int64   `json:"created_at"`
	UpdatedAt             int64   `json:"updated_at"`
	RunningAgentRunsCount int64   `json:"running_agent_runs_count"`
	ErrorMessage          *string `json:"error_message,omitempty"`
	Summary               *string `json:"summary,omitempty"`
}

type OrchestrationRound struct {
	ID              string  `json:"round_id"`
	OrchestrationID string  `json:"orchestration_id"`
	RoundIndex      int     `json:"round_index"`
	Goal            string  `json:"goal"`
	Status          string  `json:"status"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
	Summary         *string `json:"summary,omitempty"`
	SynthesisStepID *string `json:"synthesis_step_id,omitempty"`
}

type AgentRun struct {
	ID              string  `json:"agent_run_id"`
	OrchestrationID string  `json:"orchestration_id"`
	RoundID         string  `json:"round_id"`
	Role            string  `json:"role"`
	Title           string  `json:"title"`
	Goal            string  `json:"goal"`
	ExpertID        string  `json:"expert_id"`
	Intent          string  `json:"intent"`
	WorkspaceMode   string  `json:"workspace_mode"`
	WorkspacePath   string  `json:"workspace_path"`
	BranchName      *string `json:"branch_name,omitempty"`
	BaseRef         *string `json:"base_ref,omitempty"`
	WorktreePath    *string `json:"worktree_path,omitempty"`
	Status          string  `json:"status"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
	LastExecution   *string `json:"last_execution_id,omitempty"`
	ResultSummary   *string `json:"result_summary,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	ModifiedCode    bool    `json:"modified_code"`
}

type SynthesisStep struct {
	ID              string `json:"synthesis_step_id"`
	OrchestrationID string `json:"orchestration_id"`
	RoundID         string `json:"round_id"`
	Decision        string `json:"decision"`
	Summary         string `json:"summary"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type OrchestrationArtifact struct {
	ID              string  `json:"artifact_id"`
	OrchestrationID string  `json:"orchestration_id"`
	RoundID         *string `json:"round_id,omitempty"`
	AgentRunID      *string `json:"agent_run_id,omitempty"`
	SynthesisStepID *string `json:"synthesis_step_id,omitempty"`
	Kind            string  `json:"kind"`
	Title           string  `json:"title"`
	Summary         *string `json:"summary,omitempty"`
	PayloadJSON     *string `json:"payload_json,omitempty"`
	CreatedAt       int64   `json:"created_at"`
}

type AgentRunArtifactInput struct {
	Kind        string
	Title       string
	Summary     *string
	PayloadJSON *string
}

type OrchestrationDetail struct {
	Orchestration  Orchestration          `json:"orchestration"`
	Rounds         []OrchestrationRound   `json:"rounds"`
	AgentRuns      []AgentRun             `json:"agent_runs"`
	SynthesisSteps []SynthesisStep        `json:"synthesis_steps"`
	Artifacts      []OrchestrationArtifact `json:"artifacts"`
}

type PlannedRound struct {
	Goal      string
	AgentRuns []PlannedAgentRun
}

type PlannedAgentRun struct {
	Role          string
	Title         string
	Goal          string
	ExpertID      string
	Intent        string
	WorkspaceMode string
	WorkspacePath string
}

type CreateOrchestrationParams struct {
	Title         string
	Goal          string
	WorkspacePath string
	Round         PlannedRound
}

func defaultOrchestrationTitle(goal string) string {
	v := strings.TrimSpace(goal)
	if v == "" {
		return "Untitled Orchestration"
	}
	r := []rune(v)
	if len(r) > 40 {
		return string(r[:40])
	}
	return v
}

// CreateOrchestration 功能：创建 orchestration、首轮 round 与其 agent runs。
// 参数/返回：params 提供 goal/workspace 与首轮计划；返回创建后的 orchestration、round 与 agent runs。
// 失败场景：参数非法、写库失败或事件落库失败时返回 error。
// 副作用：写入 SQLite orchestration 相关表。
func (s *Store) CreateOrchestration(ctx context.Context, params CreateOrchestrationParams) (Orchestration, OrchestrationRound, []AgentRun, error) {
	if s == nil || s.db == nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("store not initialized")
	}
	goal := strings.TrimSpace(params.Goal)
	if goal == "" {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("%w: goal is required", ErrValidation)
	}
	workspace := strings.TrimSpace(params.WorkspacePath)
	if workspace == "" {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("%w: workspace_path is required", ErrValidation)
	}
	if len(params.Round.AgentRuns) == 0 {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("%w: at least one agent run is required", ErrValidation)
	}

	now := time.Now().UnixMilli()
	orch := Orchestration{
		ID:            id.New("or_"),
		Title:         defaultOrchestrationTitle(goal),
		Goal:          goal,
		WorkspacePath: workspace,
		Status:        string(OrchestrationStatusRunning),
		CurrentRound:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if strings.TrimSpace(params.Title) != "" {
		orch.Title = strings.TrimSpace(params.Title)
	}

	roundGoal := strings.TrimSpace(params.Round.Goal)
	if roundGoal == "" {
		roundGoal = goal
	}
	round := OrchestrationRound{
		ID:              id.New("rd_"),
		OrchestrationID: orch.ID,
		RoundIndex:      1,
		Goal:            roundGoal,
		Status:          string(RoundStatusRunning),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO orchestrations (id, title, goal, workspace_path, status, current_round, created_at, updated_at, error_message, summary)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL);`,
		orch.ID, orch.Title, orch.Goal, orch.WorkspacePath, orch.Status, orch.CurrentRound, orch.CreatedAt, orch.UpdatedAt,
	)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("insert orchestration: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO orchestration_rounds (id, orchestration_id, round_index, goal, status, created_at, updated_at, summary, synthesis_step_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL, NULL);`,
		round.ID, round.OrchestrationID, round.RoundIndex, round.Goal, round.Status, round.CreatedAt, round.UpdatedAt,
	)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("insert orchestration round: %w", err)
	}

	agentRuns := make([]AgentRun, 0, len(params.Round.AgentRuns))
	for _, planned := range params.Round.AgentRuns {
		run := AgentRun{
			ID:              id.New("ar_"),
			OrchestrationID: orch.ID,
			RoundID:         round.ID,
			Role:            strings.TrimSpace(planned.Role),
			Title:           strings.TrimSpace(planned.Title),
			Goal:            strings.TrimSpace(planned.Goal),
			ExpertID:        strings.TrimSpace(planned.ExpertID),
			Intent:          strings.TrimSpace(planned.Intent),
			WorkspaceMode:   strings.TrimSpace(planned.WorkspaceMode),
			WorkspacePath:   strings.TrimSpace(planned.WorkspacePath),
			Status:          string(AgentRunStatusQueued),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if run.Role == "" {
			run.Role = "implementer"
		}
		if run.Title == "" {
			run.Title = run.Role
		}
		if run.Goal == "" {
			run.Goal = goal
		}
		if run.Intent == "" {
			run.Intent = "modify"
		}
		if run.WorkspaceMode == "" {
			run.WorkspaceMode = "shared_workspace"
		}
		if run.WorkspacePath == "" {
			run.WorkspacePath = workspace
		}

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO agent_runs (
				id, orchestration_id, round_id, role, title, goal, expert_id, intent, workspace_mode, workspace_path,
				branch_name, base_ref, worktree_path, status, created_at, updated_at, last_execution_id, result_summary, error_message, modified_code
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?, ?, NULL, NULL, NULL, 0);`,
			run.ID, run.OrchestrationID, run.RoundID, run.Role, run.Title, run.Goal, run.ExpertID, run.Intent,
			run.WorkspaceMode, run.WorkspacePath, run.Status, run.CreatedAt, run.UpdatedAt,
		)
		if err != nil {
			return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("insert agent run: %w", err)
		}
		agentRuns = append(agentRuns, run)
	}

	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "orchestration", orch.ID, "orchestration.updated", now, map[string]any{"action": "created"}); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, err
	}
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "round", round.ID, "orchestration.round.updated", now, map[string]any{"action": "created", "round_id": round.ID}); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, err
	}
	for _, run := range agentRuns {
		if err := insertOrchestrationEvent(ctx, tx, orch.ID, "agent_run", run.ID, "orchestration.agent_run.updated", now, map[string]any{"action": "created", "agent_run_id": run.ID, "status": run.Status}); err != nil {
			return Orchestration{}, OrchestrationRound{}, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Orchestration{}, OrchestrationRound{}, nil, fmt.Errorf("commit: %w", err)
	}
	return orch, round, agentRuns, nil
}

// ListOrchestrations 功能：按更新时间倒序读取 orchestrations 列表。
// 参数/返回：limit 为最大条数；返回 orchestration 列表。
// 失败场景：查询失败或扫描失败时返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListOrchestrations(ctx context.Context, limit int) ([]Orchestration, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT o.id, o.title, o.goal, o.workspace_path, o.status, o.current_round, o.created_at, o.updated_at,
		        COALESCE(ar.running_agent_runs_count, 0) AS running_agent_runs_count, o.error_message, o.summary
		   FROM orchestrations o
		   LEFT JOIN (
		     SELECT orchestration_id, COUNT(1) AS running_agent_runs_count
		       FROM agent_runs
		      WHERE status = 'running'
		      GROUP BY orchestration_id
		   ) ar ON ar.orchestration_id = o.id
		  ORDER BY o.updated_at DESC
		  LIMIT ?;`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query orchestrations: %w", err)
	}
	defer rows.Close()
	out := make([]Orchestration, 0)
	for rows.Next() {
		var orch Orchestration
		if err := rows.Scan(&orch.ID, &orch.Title, &orch.Goal, &orch.WorkspacePath, &orch.Status, &orch.CurrentRound, &orch.CreatedAt, &orch.UpdatedAt, &orch.RunningAgentRunsCount, &orch.ErrorMessage, &orch.Summary); err != nil {
			return nil, fmt.Errorf("scan orchestration: %w", err)
		}
		out = append(out, orch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orchestrations: %w", err)
	}
	return out, nil
}

func getOrchestrationTx(ctx context.Context, tx *sql.Tx, orchestrationID string) (Orchestration, error) {
	var orch Orchestration
	err := tx.QueryRowContext(
		ctx,
		`SELECT o.id, o.title, o.goal, o.workspace_path, o.status, o.current_round, o.created_at, o.updated_at,
		        COALESCE(ar.running_agent_runs_count, 0) AS running_agent_runs_count, o.error_message, o.summary
		   FROM orchestrations o
		   LEFT JOIN (
		     SELECT orchestration_id, COUNT(1) AS running_agent_runs_count
		       FROM agent_runs
		      WHERE status = 'running'
		      GROUP BY orchestration_id
		   ) ar ON ar.orchestration_id = o.id
		  WHERE o.id = ?
		  LIMIT 1;`,
		orchestrationID,
	).Scan(&orch.ID, &orch.Title, &orch.Goal, &orch.WorkspacePath, &orch.Status, &orch.CurrentRound, &orch.CreatedAt, &orch.UpdatedAt, &orch.RunningAgentRunsCount, &orch.ErrorMessage, &orch.Summary)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Orchestration{}, os.ErrNotExist
		}
		return Orchestration{}, fmt.Errorf("query orchestration: %w", err)
	}
	return orch, nil
}

// GetOrchestration 功能：读取单个 orchestration。
// 参数/返回：orchestrationID 为目标 ID；返回 orchestration。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetOrchestration(ctx context.Context, orchestrationID string) (Orchestration, error) {
	if s == nil || s.db == nil {
		return Orchestration{}, fmt.Errorf("store not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Orchestration{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	orch, err := getOrchestrationTx(ctx, tx, orchestrationID)
	if err != nil {
		return Orchestration{}, err
	}
	return orch, nil
}

// GetOrchestrationDetail 功能：读取 orchestration 详情及其 rounds/agent runs/synthesis/artifacts。
// 参数/返回：orchestrationID 为目标 ID；返回完整详情。
// 失败场景：未命中返回 os.ErrNotExist；查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) GetOrchestrationDetail(ctx context.Context, orchestrationID string) (OrchestrationDetail, error) {
	if s == nil || s.db == nil {
		return OrchestrationDetail{}, fmt.Errorf("store not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OrchestrationDetail{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	orch, err := getOrchestrationTx(ctx, tx, orchestrationID)
	if err != nil {
		return OrchestrationDetail{}, err
	}
	rounds, err := listOrchestrationRoundsTx(ctx, tx, orchestrationID)
	if err != nil {
		return OrchestrationDetail{}, err
	}
	runs, err := listAgentRunsTx(ctx, tx, orchestrationID)
	if err != nil {
		return OrchestrationDetail{}, err
	}
	steps, err := listSynthesisStepsTx(ctx, tx, orchestrationID)
	if err != nil {
		return OrchestrationDetail{}, err
	}
	artifacts, err := listOrchestrationArtifactsTx(ctx, tx, orchestrationID)
	if err != nil {
		return OrchestrationDetail{}, err
	}
	return OrchestrationDetail{Orchestration: orch, Rounds: rounds, AgentRuns: runs, SynthesisSteps: steps, Artifacts: artifacts}, nil
}

// CountRunningExecutionSlots 功能：统计 workflow worker 与 orchestration agent run 共占用的运行槽位数。
// 参数/返回：无入参；返回运行中的执行槽位数。
// 失败场景：查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) CountRunningExecutionSlots(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	var workflowRunning int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM nodes WHERE node_type != 'master' AND status = 'running';`).Scan(&workflowRunning); err != nil {
		return 0, fmt.Errorf("count running workflow nodes: %w", err)
	}
	var orchestrationRunning int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM agent_runs WHERE status = 'running';`).Scan(&orchestrationRunning); err != nil {
		return 0, fmt.Errorf("count running agent runs: %w", err)
	}
	return workflowRunning + orchestrationRunning, nil
}

// ListRunnableQueuedAgentRuns 功能：读取当前可启动的 queued agent runs。
// 参数/返回：limit 为最大条数；返回可运行的 agent runs。
// 失败场景：查询失败返回 error。
// 副作用：读取 SQLite。
func (s *Store) ListRunnableQueuedAgentRuns(ctx context.Context, limit int) ([]AgentRun, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, orchestration_id, round_id, role, title, goal, expert_id, intent, workspace_mode, workspace_path,
		        branch_name, base_ref, worktree_path, status, created_at, updated_at, last_execution_id, result_summary, error_message, modified_code
		   FROM agent_runs
		  WHERE status = 'queued'
		    AND orchestration_id IN (SELECT id FROM orchestrations WHERE status = 'running')
		    AND round_id IN (SELECT id FROM orchestration_rounds WHERE status = 'running')
		  ORDER BY created_at ASC
		  LIMIT ?;`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query runnable agent runs: %w", err)
	}
	defer rows.Close()
	out := make([]AgentRun, 0)
	for rows.Next() {
		var run AgentRun
		var modifiedInt int
		if err := rows.Scan(&run.ID, &run.OrchestrationID, &run.RoundID, &run.Role, &run.Title, &run.Goal, &run.ExpertID, &run.Intent, &run.WorkspaceMode, &run.WorkspacePath, &run.BranchName, &run.BaseRef, &run.WorktreePath, &run.Status, &run.CreatedAt, &run.UpdatedAt, &run.LastExecution, &run.ResultSummary, &run.ErrorMessage, &modifiedInt); err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		run.ModifiedCode = modifiedInt != 0
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent runs: %w", err)
	}
	return out, nil
}

func listOrchestrationRoundsTx(ctx context.Context, tx *sql.Tx, orchestrationID string) ([]OrchestrationRound, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, orchestration_id, round_index, goal, status, created_at, updated_at, summary, synthesis_step_id FROM orchestration_rounds WHERE orchestration_id = ? ORDER BY round_index ASC;`, orchestrationID)
	if err != nil {
		return nil, fmt.Errorf("query rounds: %w", err)
	}
	defer rows.Close()
	out := make([]OrchestrationRound, 0)
	for rows.Next() {
		var round OrchestrationRound
		if err := rows.Scan(&round.ID, &round.OrchestrationID, &round.RoundIndex, &round.Goal, &round.Status, &round.CreatedAt, &round.UpdatedAt, &round.Summary, &round.SynthesisStepID); err != nil {
			return nil, fmt.Errorf("scan round: %w", err)
		}
		out = append(out, round)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rounds: %w", err)
	}
	return out, nil
}

func listAgentRunsTx(ctx context.Context, tx *sql.Tx, orchestrationID string) ([]AgentRun, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, orchestration_id, round_id, role, title, goal, expert_id, intent, workspace_mode, workspace_path, branch_name, base_ref, worktree_path, status, created_at, updated_at, last_execution_id, result_summary, error_message, modified_code FROM agent_runs WHERE orchestration_id = ? ORDER BY created_at ASC;`, orchestrationID)
	if err != nil {
		return nil, fmt.Errorf("query agent runs: %w", err)
	}
	defer rows.Close()
	out := make([]AgentRun, 0)
	for rows.Next() {
		var run AgentRun
		var modifiedInt int
		if err := rows.Scan(&run.ID, &run.OrchestrationID, &run.RoundID, &run.Role, &run.Title, &run.Goal, &run.ExpertID, &run.Intent, &run.WorkspaceMode, &run.WorkspacePath, &run.BranchName, &run.BaseRef, &run.WorktreePath, &run.Status, &run.CreatedAt, &run.UpdatedAt, &run.LastExecution, &run.ResultSummary, &run.ErrorMessage, &modifiedInt); err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		run.ModifiedCode = modifiedInt != 0
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent runs: %w", err)
	}
	return out, nil
}

func listSynthesisStepsTx(ctx context.Context, tx *sql.Tx, orchestrationID string) ([]SynthesisStep, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, orchestration_id, round_id, decision, summary, created_at, updated_at FROM synthesis_steps WHERE orchestration_id = ? ORDER BY created_at ASC;`, orchestrationID)
	if err != nil {
		return nil, fmt.Errorf("query synthesis steps: %w", err)
	}
	defer rows.Close()
	out := make([]SynthesisStep, 0)
	for rows.Next() {
		var step SynthesisStep
		if err := rows.Scan(&step.ID, &step.OrchestrationID, &step.RoundID, &step.Decision, &step.Summary, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan synthesis step: %w", err)
		}
		out = append(out, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate synthesis steps: %w", err)
	}
	return out, nil
}

func listOrchestrationArtifactsTx(ctx context.Context, tx *sql.Tx, orchestrationID string) ([]OrchestrationArtifact, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, orchestration_id, round_id, agent_run_id, synthesis_step_id, kind, title, summary, payload_json, created_at FROM orchestration_artifacts WHERE orchestration_id = ? ORDER BY created_at ASC;`, orchestrationID)
	if err != nil {
		return nil, fmt.Errorf("query artifacts: %w", err)
	}
	defer rows.Close()
	out := make([]OrchestrationArtifact, 0)
	for rows.Next() {
		var a OrchestrationArtifact
		if err := rows.Scan(&a.ID, &a.OrchestrationID, &a.RoundID, &a.AgentRunID, &a.SynthesisStepID, &a.Kind, &a.Title, &a.Summary, &a.PayloadJSON, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan artifact: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifacts: %w", err)
	}
	return out, nil
}

func insertOrchestrationEvent(ctx context.Context, tx *sql.Tx, orchestrationID, entityType, entityID, eventType string, ts int64, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal orchestration event payload: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO orchestration_events (id, orchestration_id, entity_type, entity_id, type, ts, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?);`, id.New("oev_"), orchestrationID, entityType, entityID, eventType, ts, string(payloadBytes))
	if err != nil {
		return fmt.Errorf("insert orchestration event: %w", err)
	}
	return nil
}
