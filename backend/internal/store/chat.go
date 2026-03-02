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

type ChatSession struct {
	ID            string  `json:"session_id"`
	Title         string  `json:"title"`
	ExpertID      string  `json:"expert_id"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	WorkspacePath string  `json:"workspace_path"`
	Status        string  `json:"status"`
	Summary       *string `json:"summary,omitempty"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
	LastTurn      int64   `json:"last_turn"`
}

type ChatMessage struct {
	ID                string  `json:"message_id"`
	SessionID         string  `json:"session_id"`
	Turn              int64   `json:"turn"`
	Role              string  `json:"role"`
	ContentText       string  `json:"content_text"`
	TokenIn           *int64  `json:"token_in,omitempty"`
	TokenOut          *int64  `json:"token_out,omitempty"`
	ProviderMessageID *string `json:"provider_message_id,omitempty"`
	CreatedAt         int64   `json:"created_at"`
}

type ChatAnchor struct {
	SessionID         string  `json:"session_id"`
	Provider          string  `json:"provider"`
	PreviousResponse  *string `json:"previous_response_id,omitempty"`
	ContainerID       *string `json:"container_id,omitempty"`
	ProviderMessageID *string `json:"provider_message_id,omitempty"`
	UpdatedAt         int64   `json:"updated_at"`
}

type ChatCompaction struct {
	ID           string `json:"compaction_id"`
	SessionID    string `json:"session_id"`
	FromTurn     int64  `json:"from_turn"`
	ToTurn       int64  `json:"to_turn"`
	BeforeTokens int64  `json:"before_tokens"`
	AfterTokens  int64  `json:"after_tokens"`
	SummaryDelta string `json:"summary_delta"`
	CreatedAt    int64  `json:"created_at"`
}

type CreateChatSessionParams struct {
	Title         string
	ExpertID      string
	Provider      string
	Model         string
	WorkspacePath string
}

func (s *Store) CreateChatSession(ctx context.Context, params CreateChatSessionParams) (ChatSession, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, fmt.Errorf("store not initialized")
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "Untitled Session"
	}
	expertID := strings.TrimSpace(params.ExpertID)
	if expertID == "" {
		return ChatSession{}, fmt.Errorf("%w: expert_id is required", ErrValidation)
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider != "openai" && provider != "anthropic" && provider != "demo" {
		return ChatSession{}, fmt.Errorf("%w: unsupported provider %q", ErrValidation, params.Provider)
	}
	model := strings.TrimSpace(params.Model)
	if provider == "demo" && model == "" {
		model = "demo"
	}
	if model == "" {
		return ChatSession{}, fmt.Errorf("%w: model is required", ErrValidation)
	}
	workspace := strings.TrimSpace(params.WorkspacePath)
	if workspace == "" {
		workspace = "."
	}

	now := time.Now().UnixMilli()
	session := ChatSession{
		ID:            id.New("cs_"),
		Title:         title,
		ExpertID:      expertID,
		Provider:      provider,
		Model:         model,
		WorkspacePath: workspace,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
		LastTurn:      0,
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO chat_sessions (id, title, expert_id, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, 0);`,
		session.ID,
		session.Title,
		session.ExpertID,
		session.Provider,
		session.Model,
		session.WorkspacePath,
		session.Status,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return ChatSession{}, fmt.Errorf("insert chat session: %w", err)
	}
	return session, nil
}

func (s *Store) ListChatSessions(ctx context.Context, limit int) ([]ChatSession, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, title, expert_id, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn
		   FROM chat_sessions
		  ORDER BY updated_at DESC
		  LIMIT ?;`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query chat sessions: %w", err)
	}
	defer rows.Close()
	out := make([]ChatSession, 0)
	for rows.Next() {
		sess, err := scanChatSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat sessions: %w", err)
	}
	return out, nil
}

func (s *Store) GetChatSession(ctx context.Context, sessionID string) (ChatSession, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, fmt.Errorf("store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ChatSession{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, title, expert_id, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn
		   FROM chat_sessions
		  WHERE id = ?
		  LIMIT 1;`,
		sessionID,
	)
	sess, err := scanChatSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatSession{}, os.ErrNotExist
		}
		return ChatSession{}, fmt.Errorf("query chat session: %w", err)
	}
	return sess, nil
}

type PatchChatSessionParams struct {
	Title  *string
	Status *string
}

func (s *Store) PatchChatSession(ctx context.Context, sessionID string, patch PatchChatSessionParams) (ChatSession, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, fmt.Errorf("store not initialized")
	}
	if patch.Title == nil && patch.Status == nil {
		return ChatSession{}, fmt.Errorf("%w: empty patch", ErrValidation)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ChatSession{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, title, expert_id, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn
		   FROM chat_sessions
		  WHERE id = ?
		  LIMIT 1;`,
		sessionID,
	)
	sess, err := scanChatSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatSession{}, os.ErrNotExist
		}
		return ChatSession{}, fmt.Errorf("query chat session: %w", err)
	}

	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return ChatSession{}, fmt.Errorf("%w: title is required", ErrValidation)
		}
		sess.Title = title
	}
	if patch.Status != nil {
		status := strings.TrimSpace(*patch.Status)
		if status != "active" && status != "archived" {
			return ChatSession{}, fmt.Errorf("%w: invalid status %q", ErrValidation, status)
		}
		sess.Status = status
	}
	sess.UpdatedAt = time.Now().UnixMilli()

	_, err = tx.ExecContext(
		ctx,
		`UPDATE chat_sessions
		    SET title = ?, status = ?, updated_at = ?
		  WHERE id = ?;`,
		sess.Title,
		sess.Status,
		sess.UpdatedAt,
		sess.ID,
	)
	if err != nil {
		return ChatSession{}, fmt.Errorf("update chat session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ChatSession{}, fmt.Errorf("commit patch chat session: %w", err)
	}
	return sess, nil
}

type AppendChatMessageParams struct {
	SessionID         string
	Role              string
	ContentText       string
	TokenIn           *int64
	TokenOut          *int64
	ProviderMessageID *string
	CreatedAt         int64
}

func (s *Store) AppendChatMessage(ctx context.Context, params AppendChatMessageParams) (ChatMessage, error) {
	if s == nil || s.db == nil {
		return ChatMessage{}, fmt.Errorf("store not initialized")
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return ChatMessage{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	role := strings.TrimSpace(params.Role)
	if role != "user" && role != "assistant" && role != "system" && role != "tool" {
		return ChatMessage{}, fmt.Errorf("%w: unsupported role %q", ErrValidation, params.Role)
	}
	content := params.ContentText
	if strings.TrimSpace(content) == "" {
		return ChatMessage{}, fmt.Errorf("%w: content_text is required", ErrValidation)
	}
	createdAt := params.CreatedAt
	if createdAt <= 0 {
		createdAt = time.Now().UnixMilli()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentTurn int64
	if err := tx.QueryRowContext(ctx, `SELECT last_turn FROM chat_sessions WHERE id = ? LIMIT 1;`, sessionID).Scan(&currentTurn); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatMessage{}, os.ErrNotExist
		}
		return ChatMessage{}, fmt.Errorf("query session last_turn: %w", err)
	}
	nextTurn := currentTurn + 1
	msg := ChatMessage{
		ID:                id.New("cm_"),
		SessionID:         sessionID,
		Turn:              nextTurn,
		Role:              role,
		ContentText:       content,
		TokenIn:           params.TokenIn,
		TokenOut:          params.TokenOut,
		ProviderMessageID: params.ProviderMessageID,
		CreatedAt:         createdAt,
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO chat_messages (id, session_id, turn, role, content_text, token_in, token_out, provider_message_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		msg.ID,
		msg.SessionID,
		msg.Turn,
		msg.Role,
		msg.ContentText,
		msg.TokenIn,
		msg.TokenOut,
		msg.ProviderMessageID,
		msg.CreatedAt,
	)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("insert chat message: %w", err)
	}
	_, err = tx.ExecContext(
		ctx,
		`UPDATE chat_sessions
		    SET last_turn = ?, updated_at = ?
		  WHERE id = ?;`,
		nextTurn,
		msg.CreatedAt,
		sessionID,
	)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("update chat session last_turn: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ChatMessage{}, fmt.Errorf("commit append chat message: %w", err)
	}
	return msg, nil
}

func (s *Store) ListChatMessages(ctx context.Context, sessionID string, limit int) ([]ChatMessage, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, session_id, turn, role, content_text, token_in, token_out, provider_message_id, created_at
		   FROM chat_messages
		  WHERE session_id = ?
		  ORDER BY turn DESC
		  LIMIT ?;`,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query chat messages: %w", err)
	}
	defer rows.Close()
	out := make([]ChatMessage, 0)
	for rows.Next() {
		msg, err := scanChatMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

type UpsertChatAnchorParams struct {
	SessionID         string
	Provider          string
	PreviousResponse  *string
	ContainerID       *string
	ProviderMessageID *string
}

func (s *Store) UpsertChatAnchor(ctx context.Context, params UpsertChatAnchorParams) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider != "openai" && provider != "anthropic" && provider != "demo" {
		return fmt.Errorf("%w: unsupported provider %q", ErrValidation, params.Provider)
	}
	now := time.Now().UnixMilli()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO chat_anchors (session_id, provider, previous_response_id, container_id, provider_message_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		   provider = excluded.provider,
		   previous_response_id = excluded.previous_response_id,
		   container_id = excluded.container_id,
		   provider_message_id = excluded.provider_message_id,
		   updated_at = excluded.updated_at;`,
		sessionID,
		provider,
		params.PreviousResponse,
		params.ContainerID,
		params.ProviderMessageID,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert chat anchor: %w", err)
	}
	return nil
}

func (s *Store) GetChatAnchor(ctx context.Context, sessionID string) (ChatAnchor, error) {
	if s == nil || s.db == nil {
		return ChatAnchor{}, fmt.Errorf("store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ChatAnchor{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(
		ctx,
		`SELECT session_id, provider, previous_response_id, container_id, provider_message_id, updated_at
		   FROM chat_anchors
		  WHERE session_id = ?
		  LIMIT 1;`,
		sessionID,
	)
	anchor, err := scanChatAnchor(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatAnchor{}, os.ErrNotExist
		}
		return ChatAnchor{}, fmt.Errorf("query chat anchor: %w", err)
	}
	return anchor, nil
}

type CreateChatCompactionParams struct {
	SessionID    string
	FromTurn     int64
	ToTurn       int64
	BeforeTokens int64
	AfterTokens  int64
	SummaryDelta string
	CreatedAt    int64
}

func (s *Store) CreateChatCompaction(ctx context.Context, params CreateChatCompactionParams) (ChatCompaction, error) {
	if s == nil || s.db == nil {
		return ChatCompaction{}, fmt.Errorf("store not initialized")
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return ChatCompaction{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	if params.FromTurn <= 0 || params.ToTurn <= 0 || params.ToTurn < params.FromTurn {
		return ChatCompaction{}, fmt.Errorf("%w: invalid turn range", ErrValidation)
	}
	if strings.TrimSpace(params.SummaryDelta) == "" {
		return ChatCompaction{}, fmt.Errorf("%w: summary_delta is required", ErrValidation)
	}
	createdAt := params.CreatedAt
	if createdAt <= 0 {
		createdAt = time.Now().UnixMilli()
	}
	record := ChatCompaction{
		ID:           id.New("cc_"),
		SessionID:    sessionID,
		FromTurn:     params.FromTurn,
		ToTurn:       params.ToTurn,
		BeforeTokens: params.BeforeTokens,
		AfterTokens:  params.AfterTokens,
		SummaryDelta: params.SummaryDelta,
		CreatedAt:    createdAt,
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO chat_compactions (id, session_id, from_turn, to_turn, before_tokens, after_tokens, summary_delta, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		record.ID,
		record.SessionID,
		record.FromTurn,
		record.ToTurn,
		record.BeforeTokens,
		record.AfterTokens,
		record.SummaryDelta,
		record.CreatedAt,
	)
	if err != nil {
		return ChatCompaction{}, fmt.Errorf("insert chat compaction: %w", err)
	}
	return record, nil
}

func (s *Store) UpdateChatSummary(ctx context.Context, sessionID string, summary string) (ChatSession, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, fmt.Errorf("store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ChatSession{}, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	now := time.Now().UnixMilli()
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE chat_sessions
		    SET summary = ?, updated_at = ?
		  WHERE id = ?;`,
		summary,
		now,
		sessionID,
	)
	if err != nil {
		return ChatSession{}, fmt.Errorf("update chat summary: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ChatSession{}, os.ErrNotExist
	}
	return s.GetChatSession(ctx, sessionID)
}

func (s *Store) ForkChatSession(ctx context.Context, sessionID string, title string) (ChatSession, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, fmt.Errorf("store not initialized")
	}
	source, err := s.GetChatSession(ctx, sessionID)
	if err != nil {
		return ChatSession{}, err
	}
	nextTitle := strings.TrimSpace(title)
	if nextTitle == "" {
		nextTitle = source.Title + " (fork)"
	}
	now := time.Now().UnixMilli()
	fork := ChatSession{
		ID:            id.New("cs_"),
		Title:         nextTitle,
		ExpertID:      source.ExpertID,
		Provider:      source.Provider,
		Model:         source.Model,
		WorkspacePath: source.WorkspacePath,
		Status:        "active",
		Summary:       source.Summary,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastTurn:      0,
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO chat_sessions (id, title, expert_id, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		fork.ID,
		fork.Title,
		fork.ExpertID,
		fork.Provider,
		fork.Model,
		fork.WorkspacePath,
		fork.Status,
		fork.Summary,
		fork.CreatedAt,
		fork.UpdatedAt,
		fork.LastTurn,
	)
	if err != nil {
		return ChatSession{}, fmt.Errorf("insert fork session: %w", err)
	}
	return fork, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanChatSession(s scanner) (ChatSession, error) {
	var session ChatSession
	if err := s.Scan(
		&session.ID,
		&session.Title,
		&session.ExpertID,
		&session.Provider,
		&session.Model,
		&session.WorkspacePath,
		&session.Status,
		&session.Summary,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.LastTurn,
	); err != nil {
		return ChatSession{}, err
	}
	return session, nil
}

func scanChatMessage(s scanner) (ChatMessage, error) {
	var msg ChatMessage
	if err := s.Scan(
		&msg.ID,
		&msg.SessionID,
		&msg.Turn,
		&msg.Role,
		&msg.ContentText,
		&msg.TokenIn,
		&msg.TokenOut,
		&msg.ProviderMessageID,
		&msg.CreatedAt,
	); err != nil {
		return ChatMessage{}, err
	}
	return msg, nil
}

func scanChatAnchor(s scanner) (ChatAnchor, error) {
	var anchor ChatAnchor
	if err := s.Scan(
		&anchor.SessionID,
		&anchor.Provider,
		&anchor.PreviousResponse,
		&anchor.ContainerID,
		&anchor.ProviderMessageID,
		&anchor.UpdatedAt,
	); err != nil {
		return ChatAnchor{}, err
	}
	return anchor, nil
}
