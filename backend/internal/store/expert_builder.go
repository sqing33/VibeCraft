package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"vibe-tree/backend/internal/id"
)

type ExpertBuilderSession struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	TargetExpertID   *string `json:"target_expert_id,omitempty"`
	BuilderModelID   string  `json:"builder_model_id"`
	Status           string  `json:"status"`
	LatestSnapshotID *string `json:"latest_snapshot_id,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

type ExpertBuilderMessage struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	Role        string `json:"role"`
	ContentText string `json:"content_text"`
	CreatedAt   int64  `json:"created_at"`
}

type ExpertBuilderSnapshot struct {
	ID               string   `json:"id"`
	SessionID        string   `json:"session_id"`
	Version          int64    `json:"version"`
	AssistantMessage string   `json:"assistant_message"`
	DraftJSON        string   `json:"draft_json"`
	RawJSON          *string  `json:"raw_json,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	CreatedAt        int64    `json:"created_at"`
}

type CreateExpertBuilderSessionParams struct {
	Title          string
	TargetExpertID string
	BuilderModelID string
}

type PatchExpertBuilderSessionParams struct {
	Title            *string
	TargetExpertID   *string
	Status           *string
	LatestSnapshotID *string
}

type CreateExpertBuilderSnapshotParams struct {
	SessionID        string
	AssistantMessage string
	DraftJSON        string
	RawJSON          *string
	Warnings         []string
	CreatedAt        int64
}

func (s *Store) CreateExpertBuilderSession(ctx context.Context, params CreateExpertBuilderSessionParams) (ExpertBuilderSession, error) {
	if s == nil || s.db == nil {
		return ExpertBuilderSession{}, fmt.Errorf("store not initialized")
	}
	builderModelID := strings.TrimSpace(params.BuilderModelID)
	if builderModelID == "" {
		return ExpertBuilderSession{}, fmt.Errorf("%w: builder_model_id is required", ErrValidation)
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "专家生成会话"
	}
	now := time.Now().UnixMilli()
	sess := ExpertBuilderSession{
		ID:             id.New("ebs_"),
		Title:          title,
		TargetExpertID: trimOrNil(pointerString(params.TargetExpertID)),
		BuilderModelID: builderModelID,
		Status:         "draft",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO expert_builder_sessions (id, title, target_expert_id, builder_model_id, status, latest_snapshot_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		sess.ID, sess.Title, sess.TargetExpertID, sess.BuilderModelID, sess.Status, sess.LatestSnapshotID, sess.CreatedAt, sess.UpdatedAt)
	if err != nil {
		return ExpertBuilderSession{}, fmt.Errorf("insert expert builder session: %w", err)
	}
	return sess, nil
}

func (s *Store) ListExpertBuilderSessions(ctx context.Context, limit int, targetExpertID string) ([]ExpertBuilderSession, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	targetExpertID = strings.TrimSpace(targetExpertID)
	query := `SELECT id, title, target_expert_id, builder_model_id, status, latest_snapshot_id, created_at, updated_at FROM expert_builder_sessions`
	args := []any{}
	if targetExpertID != "" {
		query += ` WHERE target_expert_id = ?`
		args = append(args, targetExpertID)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?;`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query expert builder sessions: %w", err)
	}
	defer rows.Close()
	out := make([]ExpertBuilderSession, 0)
	for rows.Next() {
		sess, err := scanExpertBuilderSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expert builder sessions: %w", err)
	}
	return out, nil
}

func (s *Store) GetExpertBuilderSession(ctx context.Context, sessionID string) (ExpertBuilderSession, error) {
	if s == nil || s.db == nil {
		return ExpertBuilderSession{}, fmt.Errorf("store not initialized")
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, title, target_expert_id, builder_model_id, status, latest_snapshot_id, created_at, updated_at FROM expert_builder_sessions WHERE id = ? LIMIT 1;`, strings.TrimSpace(sessionID))
	sess, err := scanExpertBuilderSession(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return ExpertBuilderSession{}, os.ErrNotExist
		}
		return ExpertBuilderSession{}, fmt.Errorf("query expert builder session: %w", err)
	}
	return sess, nil
}

func (s *Store) PatchExpertBuilderSession(ctx context.Context, sessionID string, patch PatchExpertBuilderSessionParams) (ExpertBuilderSession, error) {
	if s == nil || s.db == nil {
		return ExpertBuilderSession{}, fmt.Errorf("store not initialized")
	}
	sess, err := s.GetExpertBuilderSession(ctx, sessionID)
	if err != nil {
		return ExpertBuilderSession{}, err
	}
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return ExpertBuilderSession{}, fmt.Errorf("%w: title is required", ErrValidation)
		}
		sess.Title = title
	}
	if patch.TargetExpertID != nil {
		sess.TargetExpertID = trimOrNil(patch.TargetExpertID)
	}
	if patch.Status != nil {
		status := strings.TrimSpace(*patch.Status)
		if status != "draft" && status != "published" && status != "archived" {
			return ExpertBuilderSession{}, fmt.Errorf("%w: invalid status %q", ErrValidation, status)
		}
		sess.Status = status
	}
	if patch.LatestSnapshotID != nil {
		sess.LatestSnapshotID = trimOrNil(patch.LatestSnapshotID)
	}
	sess.UpdatedAt = time.Now().UnixMilli()
	_, err = s.db.ExecContext(ctx, `UPDATE expert_builder_sessions SET title = ?, target_expert_id = ?, status = ?, latest_snapshot_id = ?, updated_at = ? WHERE id = ?;`,
		sess.Title, sess.TargetExpertID, sess.Status, sess.LatestSnapshotID, sess.UpdatedAt, sess.ID)
	if err != nil {
		return ExpertBuilderSession{}, fmt.Errorf("update expert builder session: %w", err)
	}
	return sess, nil
}

func (s *Store) AppendExpertBuilderMessage(ctx context.Context, sessionID, role, content string) (ExpertBuilderMessage, error) {
	if s == nil || s.db == nil {
		return ExpertBuilderMessage{}, fmt.Errorf("store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	role = strings.TrimSpace(role)
	content = strings.TrimSpace(content)
	if sessionID == "" {
		return ExpertBuilderMessage{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	if role != "user" && role != "assistant" && role != "system" {
		return ExpertBuilderMessage{}, fmt.Errorf("%w: unsupported role %q", ErrValidation, role)
	}
	if content == "" {
		return ExpertBuilderMessage{}, fmt.Errorf("%w: content_text is required", ErrValidation)
	}
	msg := ExpertBuilderMessage{ID: id.New("ebm_"), SessionID: sessionID, Role: role, ContentText: content, CreatedAt: time.Now().UnixMilli()}
	_, err := s.db.ExecContext(ctx, `INSERT INTO expert_builder_messages (id, session_id, role, content_text, created_at) VALUES (?, ?, ?, ?, ?);`, msg.ID, msg.SessionID, msg.Role, msg.ContentText, msg.CreatedAt)
	if err != nil {
		return ExpertBuilderMessage{}, fmt.Errorf("insert expert builder message: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE expert_builder_sessions SET updated_at = ? WHERE id = ?;`, msg.CreatedAt, sessionID)
	return msg, nil
}

func (s *Store) ListExpertBuilderMessages(ctx context.Context, sessionID string) ([]ExpertBuilderMessage, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, role, content_text, created_at FROM expert_builder_messages WHERE session_id = ? ORDER BY created_at ASC;`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("query expert builder messages: %w", err)
	}
	defer rows.Close()
	out := make([]ExpertBuilderMessage, 0)
	for rows.Next() {
		msg, err := scanExpertBuilderMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expert builder messages: %w", err)
	}
	return out, nil
}

func (s *Store) CreateExpertBuilderSnapshot(ctx context.Context, params CreateExpertBuilderSnapshotParams) (ExpertBuilderSnapshot, error) {
	if s == nil || s.db == nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("store not initialized")
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return ExpertBuilderSnapshot{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	if strings.TrimSpace(params.AssistantMessage) == "" || strings.TrimSpace(params.DraftJSON) == "" {
		return ExpertBuilderSnapshot{}, fmt.Errorf("%w: assistant_message and draft_json are required", ErrValidation)
	}
	createdAt := params.CreatedAt
	if createdAt <= 0 {
		createdAt = time.Now().UnixMilli()
	}
	warningsJSON, err := json.Marshal(params.Warnings)
	if err != nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("marshal snapshot warnings: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM expert_builder_snapshots WHERE session_id = ?;`, sessionID).Scan(&version); err != nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("query expert builder snapshot version: %w", err)
	}
	version++
	snapshot := ExpertBuilderSnapshot{ID: id.New("ebsnp_"), SessionID: sessionID, Version: version, AssistantMessage: strings.TrimSpace(params.AssistantMessage), DraftJSON: strings.TrimSpace(params.DraftJSON), RawJSON: trimOrNil(params.RawJSON), Warnings: append([]string(nil), params.Warnings...), CreatedAt: createdAt}
	_, err = tx.ExecContext(ctx, `INSERT INTO expert_builder_snapshots (id, session_id, version, assistant_message, draft_json, raw_json, warnings_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		snapshot.ID, snapshot.SessionID, snapshot.Version, snapshot.AssistantMessage, snapshot.DraftJSON, snapshot.RawJSON, string(warningsJSON), snapshot.CreatedAt)
	if err != nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("insert expert builder snapshot: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE expert_builder_sessions SET latest_snapshot_id = ?, updated_at = ? WHERE id = ?;`, snapshot.ID, snapshot.CreatedAt, sessionID)
	if err != nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("update expert builder session latest snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("commit expert builder snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *Store) ListExpertBuilderSnapshots(ctx context.Context, sessionID string) ([]ExpertBuilderSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, version, assistant_message, draft_json, raw_json, warnings_json, created_at FROM expert_builder_snapshots WHERE session_id = ? ORDER BY version DESC;`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("query expert builder snapshots: %w", err)
	}
	defer rows.Close()
	out := make([]ExpertBuilderSnapshot, 0)
	for rows.Next() {
		snapshot, err := scanExpertBuilderSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expert builder snapshots: %w", err)
	}
	return out, nil
}

func (s *Store) GetExpertBuilderSnapshot(ctx context.Context, snapshotID string) (ExpertBuilderSnapshot, error) {
	if s == nil || s.db == nil {
		return ExpertBuilderSnapshot{}, fmt.Errorf("store not initialized")
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, session_id, version, assistant_message, draft_json, raw_json, warnings_json, created_at FROM expert_builder_snapshots WHERE id = ? LIMIT 1;`, strings.TrimSpace(snapshotID))
	snapshot, err := scanExpertBuilderSnapshot(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return ExpertBuilderSnapshot{}, os.ErrNotExist
		}
		return ExpertBuilderSnapshot{}, fmt.Errorf("query expert builder snapshot: %w", err)
	}
	return snapshot, nil
}

func scanExpertBuilderSession(s scanner) (ExpertBuilderSession, error) {
	var sess ExpertBuilderSession
	if err := s.Scan(&sess.ID, &sess.Title, &sess.TargetExpertID, &sess.BuilderModelID, &sess.Status, &sess.LatestSnapshotID, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
		return ExpertBuilderSession{}, err
	}
	return sess, nil
}

func scanExpertBuilderMessage(s scanner) (ExpertBuilderMessage, error) {
	var msg ExpertBuilderMessage
	if err := s.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.ContentText, &msg.CreatedAt); err != nil {
		return ExpertBuilderMessage{}, err
	}
	return msg, nil
}

func scanExpertBuilderSnapshot(s scanner) (ExpertBuilderSnapshot, error) {
	var snapshot ExpertBuilderSnapshot
	var warningsJSON string
	if err := s.Scan(&snapshot.ID, &snapshot.SessionID, &snapshot.Version, &snapshot.AssistantMessage, &snapshot.DraftJSON, &snapshot.RawJSON, &warningsJSON, &snapshot.CreatedAt); err != nil {
		return ExpertBuilderSnapshot{}, err
	}
	if strings.TrimSpace(warningsJSON) != "" {
		_ = json.Unmarshal([]byte(warningsJSON), &snapshot.Warnings)
	}
	return snapshot, nil
}

func pointerString(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}
