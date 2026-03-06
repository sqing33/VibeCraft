package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"vibe-tree/backend/internal/id"
)

type AgentRunExecution struct {
	ID         string  `json:"execution_id"`
	AgentRunID string  `json:"agent_run_id"`
	Attempt    int     `json:"attempt"`
	PID        *int    `json:"pid,omitempty"`
	ExitCode   *int    `json:"exit_code,omitempty"`
	Status     string  `json:"status"`
	LogPath    string  `json:"log_path"`
	StartedAt  int64   `json:"started_at"`
	EndedAt    *int64  `json:"ended_at,omitempty"`
	Signal     *string `json:"signal,omitempty"`
	Error      *string `json:"error_message,omitempty"`
}

type StartAgentRunExecutionParams struct {
	ExecutionID      string
	OrchestrationID  string
	RoundID          string
	AgentRunID       string
	Attempt          int
	PID              int
	LogPath          string
	StartedAt        int64
	Command          string
	Args             []string
	Cwd              string
}

// StartAgentRunExecution 功能：为 agent run 创建一条 running execution 记录，并更新 agent run 的 last_execution_id/status。
// 参数/返回：params 指定 execution 元数据；成功返回更新后的 AgentRun。
// 失败场景：agent run 不存在或关联不匹配时返回 error。
// 副作用：写入 SQLite agent_run_executions/agent_runs/orchestration_events。
func (s *Store) StartAgentRunExecution(ctx context.Context, params StartAgentRunExecutionParams) (AgentRun, error) {
	if s == nil || s.db == nil {
		return AgentRun{}, fmt.Errorf("store not initialized")
	}
	if params.ExecutionID == "" || params.AgentRunID == "" || params.OrchestrationID == "" || params.RoundID == "" {
		return AgentRun{}, fmt.Errorf("%w: missing execution/orchestration/round/agent_run id", ErrValidation)
	}
	if params.LogPath == "" {
		return AgentRun{}, fmt.Errorf("%w: log_path is required", ErrValidation)
	}
	if params.StartedAt <= 0 {
		params.StartedAt = time.Now().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AgentRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	run, err := getAgentRunTx(ctx, tx, params.AgentRunID)
	if err != nil {
		return AgentRun{}, err
	}
	if run.OrchestrationID != params.OrchestrationID || run.RoundID != params.RoundID {
		return AgentRun{}, fmt.Errorf("%w: agent run context mismatch", ErrValidation)
	}

	if params.Attempt <= 0 {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(attempt), 0) FROM agent_run_executions WHERE agent_run_id = ?;`, params.AgentRunID).Scan(&params.Attempt); err != nil {
			return AgentRun{}, fmt.Errorf("query max agent run attempt: %w", err)
		}
		params.Attempt += 1
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO agent_run_executions (id, agent_run_id, attempt, pid, exit_code, status, log_path, started_at, ended_at, signal, error_message) VALUES (?, ?, ?, ?, NULL, ?, ?, ?, NULL, NULL, NULL);`, params.ExecutionID, params.AgentRunID, params.Attempt, params.PID, "running", params.LogPath, params.StartedAt)
	if err != nil {
		return AgentRun{}, fmt.Errorf("insert agent_run_execution: %w", err)
	}

	run.Status = string(AgentRunStatusRunning)
	run.UpdatedAt = params.StartedAt
	run.LastExecution = &params.ExecutionID
	if _, err := tx.ExecContext(ctx, `UPDATE agent_runs SET status = ?, updated_at = ?, last_execution_id = ? WHERE id = ?;`, run.Status, run.UpdatedAt, params.ExecutionID, run.ID); err != nil {
		return AgentRun{}, fmt.Errorf("update agent run execution: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET status = ?, updated_at = ? WHERE id = ?;`, string(OrchestrationStatusRunning), params.StartedAt, params.OrchestrationID); err != nil {
		return AgentRun{}, fmt.Errorf("touch orchestration running: %w", err)
	}
	if err := insertOrchestrationEvent(ctx, tx, params.OrchestrationID, "agent_run", params.AgentRunID, "orchestration.agent_run.updated", params.StartedAt, map[string]any{"action": "execution.started", "agent_run_id": params.AgentRunID, "execution_id": params.ExecutionID, "status": run.Status, "command": params.Command, "args": params.Args, "cwd": params.Cwd}); err != nil {
		return AgentRun{}, err
	}

	if err := tx.Commit(); err != nil {
		return AgentRun{}, fmt.Errorf("commit: %w", err)
	}
	return run, nil
}

type FinalizeAgentRunExecutionParams struct {
	ExecutionID     string
	OrchestrationID string
	RoundID         string
	AgentRunID      string
	Status          string
	ExitCode        int
	Signal          string
	StartedAt       int64
	EndedAt         int64
	ErrorMessage    string
	ResultSummary   *string
	ModifiedCode    *bool
	Artifacts       []AgentRunArtifactInput
}

// FinalizeAgentRunExecution 功能：收敛 agent run execution 终态，并在一轮完成时生成 synthesis。
// 参数/返回：params 指定退出信息与可选结果摘要；返回更新后的 orchestration/round/agent run 与可选 synthesis/artifacts。
// 失败场景：记录不存在或写库失败时返回 error。
// 副作用：写入 SQLite agent_run_executions/agent_runs/orchestration_* 表。
func (s *Store) FinalizeAgentRunExecution(ctx context.Context, params FinalizeAgentRunExecutionParams) (Orchestration, OrchestrationRound, AgentRun, *SynthesisStep, []OrchestrationArtifact, error) {
	if s == nil || s.db == nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("store not initialized")
	}
	if params.ExecutionID == "" || params.AgentRunID == "" || params.OrchestrationID == "" || params.RoundID == "" {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("%w: missing execution/orchestration/round/agent_run id", ErrValidation)
	}
	if params.EndedAt <= 0 {
		params.EndedAt = time.Now().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	orch, err := getOrchestrationTx(ctx, tx, params.OrchestrationID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
	}
	round, err := getOrchestrationRoundTx(ctx, tx, params.RoundID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
	}
	run, err := getAgentRunTx(ctx, tx, params.AgentRunID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
	}
	if run.OrchestrationID != params.OrchestrationID || run.RoundID != params.RoundID {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("%w: agent run context mismatch", ErrValidation)
	}

	execErr := any(nil)
	if params.ErrorMessage != "" && (params.Status == string(AgentRunStatusFailed) || params.Status == string(AgentRunStatusCanceled) || params.Status == string(AgentRunStatusTimeout)) {
		execErr = params.ErrorMessage
		run.ErrorMessage = &params.ErrorMessage
	} else {
		run.ErrorMessage = nil
	}
	if params.ResultSummary != nil {
		run.ResultSummary = params.ResultSummary
	}
	if params.ModifiedCode != nil {
		run.ModifiedCode = *params.ModifiedCode
	}
	run.Status = params.Status
	run.UpdatedAt = params.EndedAt

	res, err := tx.ExecContext(ctx, `UPDATE agent_run_executions SET status = ?, exit_code = ?, started_at = ?, ended_at = ?, signal = ?, error_message = ? WHERE id = ?;`, params.Status, params.ExitCode, params.StartedAt, params.EndedAt, nullableString(params.Signal), execErr, params.ExecutionID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("update agent_run_execution: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, os.ErrNotExist
	}

	modifiedInt := 0
	if run.ModifiedCode {
		modifiedInt = 1
	}
	if _, err := tx.ExecContext(ctx, `UPDATE agent_runs SET status = ?, updated_at = ?, result_summary = ?, error_message = ?, modified_code = ? WHERE id = ?;`, run.Status, run.UpdatedAt, run.ResultSummary, run.ErrorMessage, modifiedInt, run.ID); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("update agent run: %w", err)
	}

	artifacts := make([]OrchestrationArtifact, 0, 2)
	if len(params.Artifacts) > 0 {
		created, err := insertAgentRunArtifactsTx(ctx, tx, orch.ID, round.ID, run.ID, nil, params.EndedAt, params.Artifacts)
		if err != nil {
			return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
		}
		artifacts = append(artifacts, created...)
	}
	if run.ResultSummary != nil || run.ErrorMessage != nil {
		summary := run.ResultSummary
		if summary == nil {
			summary = run.ErrorMessage
		}
		artifact := OrchestrationArtifact{ID: id.New("oa_"), OrchestrationID: orch.ID, RoundID: &round.ID, AgentRunID: &run.ID, Kind: "agent_run_summary", Title: run.Title, Summary: summary, CreatedAt: params.EndedAt}
		if _, err := tx.ExecContext(ctx, `INSERT INTO orchestration_artifacts (id, orchestration_id, round_id, agent_run_id, synthesis_step_id, kind, title, summary, payload_json, created_at) VALUES (?, ?, ?, ?, NULL, ?, ?, ?, NULL, ?);`, artifact.ID, artifact.OrchestrationID, artifact.RoundID, artifact.AgentRunID, artifact.Kind, artifact.Title, artifact.Summary, artifact.CreatedAt); err != nil {
			return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("insert agent run artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}

	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "agent_run", run.ID, "orchestration.agent_run.updated", params.EndedAt, map[string]any{"action": "execution.exited", "agent_run_id": run.ID, "execution_id": params.ExecutionID, "status": run.Status}); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
	}

	remaining, err := countNonTerminalAgentRunsTx(ctx, tx, round.ID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
	}
	var synthesis *SynthesisStep
	if remaining == 0 {
		if orch.Status == string(OrchestrationStatusCanceled) {
			round.Status = string(RoundStatusCanceled)
			if _, err := tx.ExecContext(ctx, `UPDATE orchestration_rounds SET status = ?, updated_at = ? WHERE id = ?;`, round.Status, params.EndedAt, round.ID); err != nil {
				return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("update canceled round: %w", err)
			}
		} else {
			synthesis, artifacts, err = finalizeRoundSynthesisTx(ctx, tx, orch, round, params.EndedAt, artifacts)
			if err != nil {
				return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
			}
			round, err = getOrchestrationRoundTx(ctx, tx, round.ID)
			if err != nil {
				return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
			}
			orch, err = getOrchestrationTx(ctx, tx, orch.ID)
			if err != nil {
				return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, err
			}
		}
	} else {
		orch.UpdatedAt = params.EndedAt
		if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET updated_at = ? WHERE id = ?;`, orch.UpdatedAt, orch.ID); err != nil {
			return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("touch orchestration updated_at: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, nil, nil, fmt.Errorf("commit: %w", err)
	}
	return orch, round, run, synthesis, artifacts, nil
}

func finalizeRoundSynthesisTx(ctx context.Context, tx *sql.Tx, orch Orchestration, round OrchestrationRound, now int64, existing []OrchestrationArtifact) (*SynthesisStep, []OrchestrationArtifact, error) {
	runs, err := listAgentRunsByRoundTx(ctx, tx, round.ID)
	if err != nil {
		return nil, nil, err
	}
	roundArtifacts, err := listRoundArtifactsTx(ctx, tx, round.ID)
	if err != nil {
		return nil, nil, err
	}
	hasFailure := false
	parts := make([]string, 0, len(runs)+1)
	parts = append(parts, fmt.Sprintf("Round %d 汇总：", round.RoundIndex))
	for _, run := range runs {
			fallback := pointerString(run.Goal)
			piece := fmt.Sprintf("- [%s] %s：%s", run.Status, run.Title, firstNonEmpty(run.ResultSummary, run.ErrorMessage, fallback))
		parts = append(parts, piece)
		switch run.Status {
		case string(AgentRunStatusFailed), string(AgentRunStatusCanceled), string(AgentRunStatusTimeout):
			hasFailure = true
		}
	}
	for _, artifact := range roundArtifacts {
		if artifact.Summary == nil || strings.TrimSpace(*artifact.Summary) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("- [artifact:%s] %s：%s", artifact.Kind, artifact.Title, strings.TrimSpace(*artifact.Summary)))
	}
	decision := string(SynthesisDecisionComplete)
	roundStatus := string(RoundStatusDone)
	orchStatus := string(OrchestrationStatusDone)
	if hasFailure {
		decision = string(SynthesisDecisionNeedsRetry)
		roundStatus = string(RoundStatusRetryable)
		orchStatus = string(OrchestrationStatusFailed)
	} else if shouldContinueRound(runs, round.RoundIndex) {
		decision = string(SynthesisDecisionContinue)
		roundStatus = string(RoundStatusDone)
		orchStatus = string(OrchestrationStatusWaitingContinue)
		parts = append(parts, "", "建议：当前轮已完成，等待用户继续进入下一轮。")
	}
	summary := strings.Join(parts, "\n")
	step := &SynthesisStep{ID: id.New("sy_"), OrchestrationID: orch.ID, RoundID: round.ID, Decision: decision, Summary: summary, CreatedAt: now, UpdatedAt: now}
	if _, err := tx.ExecContext(ctx, `INSERT INTO synthesis_steps (id, orchestration_id, round_id, decision, summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?);`, step.ID, step.OrchestrationID, step.RoundID, step.Decision, step.Summary, step.CreatedAt, step.UpdatedAt); err != nil {
		return nil, nil, fmt.Errorf("insert synthesis step: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orchestration_rounds SET status = ?, updated_at = ?, summary = ?, synthesis_step_id = ? WHERE id = ?;`, roundStatus, now, summary, step.ID, round.ID); err != nil {
		return nil, nil, fmt.Errorf("update round synthesis: %w", err)
	}
	var errMsg any = nil
	if orchStatus == string(OrchestrationStatusFailed) {
		errMsg = summary
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET status = ?, updated_at = ?, summary = ?, error_message = ? WHERE id = ?;`, orchStatus, now, summary, errMsg, orch.ID); err != nil {
		return nil, nil, fmt.Errorf("update orchestration synthesis: %w", err)
	}
	artifact := OrchestrationArtifact{ID: id.New("oa_"), OrchestrationID: orch.ID, RoundID: &round.ID, SynthesisStepID: &step.ID, Kind: "synthesis_summary", Title: "Round Synthesis", Summary: &summary, CreatedAt: now}
	if _, err := tx.ExecContext(ctx, `INSERT INTO orchestration_artifacts (id, orchestration_id, round_id, agent_run_id, synthesis_step_id, kind, title, summary, payload_json, created_at) VALUES (?, ?, ?, NULL, ?, ?, ?, ?, NULL, ?);`, artifact.ID, artifact.OrchestrationID, artifact.RoundID, artifact.SynthesisStepID, artifact.Kind, artifact.Title, artifact.Summary, artifact.CreatedAt); err != nil {
		return nil, nil, fmt.Errorf("insert synthesis artifact: %w", err)
	}
	existing = append(existing, artifact)
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "synthesis", step.ID, "orchestration.synthesis.updated", now, map[string]any{"action": "created", "round_id": round.ID, "decision": step.Decision}); err != nil {
		return nil, nil, err
	}
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "orchestration", orch.ID, "orchestration.updated", now, map[string]any{"action": "synthesized", "status": orchStatus}); err != nil {
		return nil, nil, err
	}
	return step, existing, nil
}

func listRoundArtifactsTx(ctx context.Context, tx *sql.Tx, roundID string) ([]OrchestrationArtifact, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, orchestration_id, round_id, agent_run_id, synthesis_step_id, kind, title, summary, payload_json, created_at FROM orchestration_artifacts WHERE round_id = ? ORDER BY created_at ASC;`, roundID)
	if err != nil {
		return nil, fmt.Errorf("query round artifacts: %w", err)
	}
	defer rows.Close()
	out := make([]OrchestrationArtifact, 0)
	for rows.Next() {
		var artifact OrchestrationArtifact
		if err := rows.Scan(&artifact.ID, &artifact.OrchestrationID, &artifact.RoundID, &artifact.AgentRunID, &artifact.SynthesisStepID, &artifact.Kind, &artifact.Title, &artifact.Summary, &artifact.PayloadJSON, &artifact.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan round artifact: %w", err)
		}
		out = append(out, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate round artifacts: %w", err)
	}
	return out, nil
}

// RetryAgentRun 功能：将失败的 agent run 重置为 queued，并清除本轮 synthesis 以便重新收敛。
// 参数/返回：agentRunID 为目标；返回更新后的 orchestration/round/agent run。
// 失败场景：状态不可重试或写库失败时返回 error。
// 副作用：写入 SQLite agent_runs/orchestrations/orchestration_rounds/synthesis_steps/orchestration_artifacts。
func (s *Store) RetryAgentRun(ctx context.Context, agentRunID string) (Orchestration, OrchestrationRound, AgentRun, error) {
	if s == nil || s.db == nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("store not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	run, err := getAgentRunTx(ctx, tx, agentRunID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, err
	}
	switch run.Status {
	case string(AgentRunStatusFailed), string(AgentRunStatusCanceled), string(AgentRunStatusTimeout):
	default:
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("%w: agent run status %q not retryable", ErrConflict, run.Status)
	}
	orch, err := getOrchestrationTx(ctx, tx, run.OrchestrationID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, err
	}
	round, err := getOrchestrationRoundTx(ctx, tx, run.RoundID)
	if err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, err
	}
	now := time.Now().UnixMilli()
	run.Status = string(AgentRunStatusQueued)
	run.UpdatedAt = now
	run.ResultSummary = nil
	run.ErrorMessage = nil
	if _, err := tx.ExecContext(ctx, `UPDATE agent_runs SET status = ?, updated_at = ?, result_summary = NULL, error_message = NULL WHERE id = ?;`, run.Status, run.UpdatedAt, run.ID); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("retry agent run: %w", err)
	}
	if round.SynthesisStepID != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM orchestration_artifacts WHERE synthesis_step_id = ?;`, *round.SynthesisStepID); err != nil {
			return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("delete synthesis artifacts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM synthesis_steps WHERE id = ?;`, *round.SynthesisStepID); err != nil {
			return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("delete synthesis step: %w", err)
		}
		round.SynthesisStepID = nil
	}
	round.Status = string(RoundStatusRunning)
	round.UpdatedAt = now
	round.Summary = nil
	if _, err := tx.ExecContext(ctx, `UPDATE orchestration_rounds SET status = ?, updated_at = ?, summary = NULL, synthesis_step_id = NULL WHERE id = ?;`, round.Status, round.UpdatedAt, round.ID); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("update round retry: %w", err)
	}
	orch.Status = string(OrchestrationStatusRunning)
	orch.UpdatedAt = now
	orch.ErrorMessage = nil
	orch.Summary = nil
	if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET status = ?, updated_at = ?, error_message = NULL, summary = NULL WHERE id = ?;`, orch.Status, orch.UpdatedAt, orch.ID); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("update orchestration retry: %w", err)
	}
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "agent_run", run.ID, "orchestration.agent_run.updated", now, map[string]any{"action": "retry.queued", "agent_run_id": run.ID, "status": run.Status}); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return Orchestration{}, OrchestrationRound{}, AgentRun{}, fmt.Errorf("commit: %w", err)
	}
	return orch, round, run, nil
}

type CancelOrchestrationResult struct {
	Orchestration        Orchestration
	RunningExecutionIDs  []string
}

// CancelOrchestration 功能：取消 orchestration，并返回需要取消的运行中 execution_id 列表。
// 参数/返回：orchestrationID 为目标；返回更新后的 orchestration 与运行中的 execution_id。
// 失败场景：状态不允许或写库失败时返回 error。
// 副作用：写入 SQLite orchestration/agent_runs/orchestration_rounds。
func (s *Store) CancelOrchestration(ctx context.Context, orchestrationID string) (CancelOrchestrationResult, error) {
	if s == nil || s.db == nil {
		return CancelOrchestrationResult{}, fmt.Errorf("store not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	orch, err := getOrchestrationTx(ctx, tx, orchestrationID)
	if err != nil {
		return CancelOrchestrationResult{}, err
	}
	switch orch.Status {
	case string(OrchestrationStatusRunning), string(OrchestrationStatusPlanning), string(OrchestrationStatusFailed):
	default:
		return CancelOrchestrationResult{}, fmt.Errorf("%w: orchestration status %q cannot be canceled", ErrConflict, orch.Status)
	}
	now := time.Now().UnixMilli()
	orch.Status = string(OrchestrationStatusCanceled)
	orch.UpdatedAt = now
	if _, err := tx.ExecContext(ctx, `UPDATE orchestrations SET status = ?, updated_at = ? WHERE id = ?;`, orch.Status, orch.UpdatedAt, orch.ID); err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("update orchestration canceled: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orchestration_rounds SET status = ?, updated_at = ? WHERE orchestration_id = ? AND status = 'running';`, string(RoundStatusCanceled), now, orch.ID); err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("cancel orchestration rounds: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE agent_runs SET status = ?, updated_at = ?, error_message = 'canceled' WHERE orchestration_id = ? AND status = 'queued';`, string(AgentRunStatusCanceled), now, orch.ID); err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("cancel queued agent runs: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT last_execution_id FROM agent_runs WHERE orchestration_id = ? AND status = 'running' AND last_execution_id IS NOT NULL;`, orch.ID)
	if err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("query running orchestration executions: %w", err)
	}
	defer rows.Close()
	executionIDs := make([]string, 0)
	for rows.Next() {
		var executionID string
		if err := rows.Scan(&executionID); err != nil {
			return CancelOrchestrationResult{}, fmt.Errorf("scan running orchestration execution: %w", err)
		}
		executionIDs = append(executionIDs, executionID)
	}
	if err := rows.Err(); err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("iterate running orchestration executions: %w", err)
	}
	if err := insertOrchestrationEvent(ctx, tx, orch.ID, "orchestration", orch.ID, "orchestration.updated", now, map[string]any{"action": "canceled", "status": orch.Status}); err != nil {
		return CancelOrchestrationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CancelOrchestrationResult{}, fmt.Errorf("commit: %w", err)
	}
	return CancelOrchestrationResult{Orchestration: orch, RunningExecutionIDs: executionIDs}, nil
}

func getAgentRunTx(ctx context.Context, tx *sql.Tx, agentRunID string) (AgentRun, error) {
	var run AgentRun
	var modifiedInt int
	err := tx.QueryRowContext(ctx, `SELECT id, orchestration_id, round_id, role, title, goal, expert_id, intent, workspace_mode, workspace_path, branch_name, base_ref, worktree_path, status, created_at, updated_at, last_execution_id, result_summary, error_message, modified_code FROM agent_runs WHERE id = ? LIMIT 1;`, agentRunID).Scan(&run.ID, &run.OrchestrationID, &run.RoundID, &run.Role, &run.Title, &run.Goal, &run.ExpertID, &run.Intent, &run.WorkspaceMode, &run.WorkspacePath, &run.BranchName, &run.BaseRef, &run.WorktreePath, &run.Status, &run.CreatedAt, &run.UpdatedAt, &run.LastExecution, &run.ResultSummary, &run.ErrorMessage, &modifiedInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AgentRun{}, os.ErrNotExist
		}
		return AgentRun{}, fmt.Errorf("query agent run: %w", err)
	}
	run.ModifiedCode = modifiedInt != 0
	return run, nil
}

func getOrchestrationRoundTx(ctx context.Context, tx *sql.Tx, roundID string) (OrchestrationRound, error) {
	var round OrchestrationRound
	err := tx.QueryRowContext(ctx, `SELECT id, orchestration_id, round_index, goal, status, created_at, updated_at, summary, synthesis_step_id FROM orchestration_rounds WHERE id = ? LIMIT 1;`, roundID).Scan(&round.ID, &round.OrchestrationID, &round.RoundIndex, &round.Goal, &round.Status, &round.CreatedAt, &round.UpdatedAt, &round.Summary, &round.SynthesisStepID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrchestrationRound{}, os.ErrNotExist
		}
		return OrchestrationRound{}, fmt.Errorf("query orchestration round: %w", err)
	}
	return round, nil
}

func listAgentRunsByRoundTx(ctx context.Context, tx *sql.Tx, roundID string) ([]AgentRun, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, orchestration_id, round_id, role, title, goal, expert_id, intent, workspace_mode, workspace_path, branch_name, base_ref, worktree_path, status, created_at, updated_at, last_execution_id, result_summary, error_message, modified_code FROM agent_runs WHERE round_id = ? ORDER BY created_at ASC;`, roundID)
	if err != nil {
		return nil, fmt.Errorf("query agent runs by round: %w", err)
	}
	defer rows.Close()
	out := make([]AgentRun, 0)
	for rows.Next() {
		var run AgentRun
		var modifiedInt int
		if err := rows.Scan(&run.ID, &run.OrchestrationID, &run.RoundID, &run.Role, &run.Title, &run.Goal, &run.ExpertID, &run.Intent, &run.WorkspaceMode, &run.WorkspacePath, &run.BranchName, &run.BaseRef, &run.WorktreePath, &run.Status, &run.CreatedAt, &run.UpdatedAt, &run.LastExecution, &run.ResultSummary, &run.ErrorMessage, &modifiedInt); err != nil {
			return nil, fmt.Errorf("scan agent run by round: %w", err)
		}
		run.ModifiedCode = modifiedInt != 0
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent runs by round: %w", err)
	}
	return out, nil
}

func countNonTerminalAgentRunsTx(ctx context.Context, tx *sql.Tx, roundID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM agent_runs WHERE round_id = ? AND status IN ('queued', 'running');`, roundID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count non-terminal agent runs: %w", err)
	}
	return n, nil
}

func firstNonEmpty(values ...*string) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		if v := strings.TrimSpace(*value); v != "" {
			return v
		}
	}
	return ""
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func pointerString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func shouldContinueRound(runs []AgentRun, roundIndex int) bool {
	if roundIndex >= 2 {
		return false
	}
	for _, run := range runs {
		if run.Intent == "analyze" || run.Intent == "modify" {
			return true
		}
	}
	return false
}
