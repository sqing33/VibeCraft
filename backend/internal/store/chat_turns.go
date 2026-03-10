package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"vibe-tree/backend/internal/id"
)

type ChatTurn struct {
	ID                         string         `json:"turn_id"`
	SessionID                  string         `json:"session_id"`
	UserMessageID              string         `json:"user_message_id"`
	AssistantMessageID         *string        `json:"assistant_message_id,omitempty"`
	Turn                       int64          `json:"turn"`
	Status                     string         `json:"status"`
	ExpertID                   string         `json:"expert_id,omitempty"`
	Provider                   string         `json:"provider,omitempty"`
	Model                      string         `json:"model,omitempty"`
	ModelInput                 *string        `json:"model_input,omitempty"`
	ContextMode                *string        `json:"context_mode,omitempty"`
	ThinkingTranslationApplied bool           `json:"thinking_translation_applied"`
	ThinkingTranslationFailed  bool           `json:"thinking_translation_failed"`
	TokenIn                    *int64         `json:"token_in,omitempty"`
	TokenOut                   *int64         `json:"token_out,omitempty"`
	CachedInputTokens          *int64         `json:"cached_input_tokens,omitempty"`
	CreatedAt                  int64          `json:"created_at"`
	UpdatedAt                  int64          `json:"updated_at"`
	CompletedAt                *int64         `json:"completed_at,omitempty"`
	Items                      []ChatTurnItem `json:"items,omitempty"`
}

type ChatTurnItem struct {
	EntryID     string         `json:"entry_id"`
	Seq         int            `json:"seq"`
	Kind        string         `json:"kind"`
	Status      string         `json:"status"`
	ContentText string         `json:"content_text"`
	Meta        map[string]any `json:"meta,omitempty"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

type CreateChatTurnParams struct {
	SessionID                  string
	UserMessageID              string
	Turn                       int64
	ExpertID                   *string
	Provider                   *string
	Model                      *string
	ModelInput                 *string
	ThinkingTranslationApplied bool
}

type StartChatTurnParams = CreateChatTurnParams

type UpsertChatTurnItemParams struct {
	TurnID        string
	SessionID     string
	UserMessageID string
	EntryID       string
	Seq           int
	Kind          string
	Status        string
	Op            string
	Delta         string
	ContentText   string
	Meta          map[string]any
}

type AppendChatTurnItemTranslatedContentParams struct {
	TurnID        string
	SessionID     string
	UserMessageID string
	EntryID       string
	Delta         string
	Replace       bool
	Failed        bool
}

type CompleteChatTurnParams struct {
	TurnID                     string
	SessionID                  string
	UserMessageID              string
	AssistantMessageID         string
	ModelInput                 *string
	ContextMode                *string
	ThinkingTranslationApplied bool
	ThinkingTranslationFailed  bool
	TokenIn                    *int64
	TokenOut                   *int64
	CachedInputTokens          *int64
}

type FailChatTurnParams struct {
	TurnID        string
	SessionID     string
	UserMessageID string
}

// StartChatTurn 功能：为用户消息创建或复用一条运行中的聊天 turn 记录。
// 参数/返回：接收 session/user 消息关联与运行时元信息；返回最新 turn 快照。
// 失败场景：缺少关键 ID、session 未命中或写库失败时返回 error。
// 副作用：写入 SQLite `chat_turns`，必要时更新已有 running turn 的元信息。
func (s *Store) StartChatTurn(ctx context.Context, params CreateChatTurnParams) (ChatTurn, error) {
	if s == nil || s.db == nil {
		return ChatTurn{}, fmt.Errorf("store not initialized")
	}
	sessionID := strings.TrimSpace(params.SessionID)
	userMessageID := strings.TrimSpace(params.UserMessageID)
	if sessionID == "" || userMessageID == "" || params.Turn <= 0 {
		return ChatTurn{}, fmt.Errorf("%w: session_id, user_message_id and turn are required", ErrValidation)
	}
	now := time.Now().UnixMilli()
	turn := ChatTurn{
		ID:                         id.New("ct_"),
		SessionID:                  sessionID,
		UserMessageID:              userMessageID,
		Turn:                       params.Turn,
		Status:                     "running",
		ExpertID:                   pointerStringValue(params.ExpertID),
		Provider:                   pointerStringValue(params.Provider),
		Model:                      pointerStringValue(params.Model),
		ModelInput:                 trimOrNil(params.ModelInput),
		ThinkingTranslationApplied: params.ThinkingTranslationApplied,
		ThinkingTranslationFailed:  false,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO chat_turns (
			id, session_id, user_message_id, assistant_message_id, turn, status,
			expert_id, provider, model, model_input, context_mode,
			thinking_translation_applied, thinking_translation_failed,
			token_in, token_out, cached_input_tokens,
			created_at, updated_at, completed_at
		 ) VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, NULL, ?, 0, NULL, NULL, NULL, ?, ?, NULL)
		 ON CONFLICT(user_message_id) DO UPDATE SET
			session_id = excluded.session_id,
			turn = excluded.turn,
			status = 'running',
			expert_id = excluded.expert_id,
			provider = excluded.provider,
			model = excluded.model,
			model_input = COALESCE(excluded.model_input, chat_turns.model_input),
			thinking_translation_applied = excluded.thinking_translation_applied,
			updated_at = excluded.updated_at,
			completed_at = NULL;`,
		turn.ID,
		turn.SessionID,
		turn.UserMessageID,
		turn.Turn,
		turn.Status,
		trimOrNil(pointerStringValueOrNil(turn.ExpertID)),
		trimOrNil(pointerStringValueOrNil(turn.Provider)),
		trimOrNil(pointerStringValueOrNil(turn.Model)),
		turn.ModelInput,
		boolToInt(turn.ThinkingTranslationApplied),
		turn.CreatedAt,
		turn.UpdatedAt,
	); err != nil {
		return ChatTurn{}, fmt.Errorf("insert chat turn: %w", err)
	}
	return s.GetChatTurnByUserMessage(ctx, sessionID, userMessageID)
}

// UpsertChatTurnItem 功能：按 entry_id 原地更新一个可恢复的时间线条目快照。
// 参数/返回：接收 turn 或 session/user 标识与条目更新语义；返回更新后的条目。
// 失败场景：turn 未创建、关键字段缺失或写库失败时返回 error。
// 副作用：写入 SQLite `chat_turn_items` 并刷新所属 turn 的更新时间。
func (s *Store) UpsertChatTurnItem(ctx context.Context, params UpsertChatTurnItemParams) (ChatTurnItem, error) {
	if s == nil || s.db == nil {
		return ChatTurnItem{}, fmt.Errorf("store not initialized")
	}
	entryID := strings.TrimSpace(params.EntryID)
	kind := strings.TrimSpace(params.Kind)
	status := strings.TrimSpace(params.Status)
	if entryID == "" || kind == "" {
		return ChatTurnItem{}, fmt.Errorf("%w: entry_id and kind are required", ErrValidation)
	}
	turn, err := s.lookupChatTurn(ctx, params.TurnID, params.SessionID, params.UserMessageID)
	if err != nil {
		return ChatTurnItem{}, err
	}
	existing, err := s.getChatTurnItem(ctx, turn.ID, entryID)
	if err != nil && !os.IsNotExist(err) {
		return ChatTurnItem{}, err
	}
	hasExisting := err == nil
	if params.Seq <= 0 {
		if hasExisting {
			params.Seq = existing.Seq
		} else {
			switch kind {
			case "thinking":
				params.Seq = 1
			case "answer":
				params.Seq = 2
			default:
				params.Seq = 1
			}
		}
	}
	if status == "" {
		if hasExisting {
			status = existing.Status
		} else {
			status = "created"
		}
	}
	content := strings.TrimSpace(params.ContentText)
	op := strings.ToLower(strings.TrimSpace(params.Op))
	delta := params.Delta
	if content == "" {
		switch op {
		case "append":
			content = existing.ContentText + delta
		case "replace", "complete":
			content = delta
		case "upsert":
			if delta != "" {
				content = existing.ContentText + delta
			} else if hasExisting {
				content = existing.ContentText
			}
		default:
			if hasExisting {
				content = existing.ContentText
			}
		}
	}
	meta := cloneMetaMap(params.Meta)
	if len(meta) == 0 && hasExisting {
		meta = cloneMetaMap(existing.Meta)
	}
	metaJSON, err := marshalTurnMeta(stripTranslationMeta(meta))
	if err != nil {
		return ChatTurnItem{}, err
	}
	now := time.Now().UnixMilli()
	createdAt := now
	if hasExisting {
		createdAt = existing.CreatedAt
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO chat_turn_items (
			turn_id, entry_id, seq, kind, status, content_text,
			translated_content, translation_failed, meta_json, created_at, updated_at
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(turn_id, entry_id) DO UPDATE SET
			kind = excluded.kind,
			status = excluded.status,
			content_text = excluded.content_text,
			translated_content = COALESCE(excluded.translated_content, chat_turn_items.translated_content),
			translation_failed = CASE WHEN excluded.translation_failed != 0 THEN excluded.translation_failed ELSE chat_turn_items.translation_failed END,
			meta_json = COALESCE(excluded.meta_json, chat_turn_items.meta_json),
			updated_at = excluded.updated_at;`,
		turn.ID,
		entryID,
		params.Seq,
		kind,
		status,
		content,
		translatedMetaValue(meta),
		boolToInt(translationFailedValue(meta)),
		metaJSON,
		createdAt,
		now,
	); err != nil {
		return ChatTurnItem{}, fmt.Errorf("upsert chat turn item: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE chat_turns SET updated_at = ? WHERE id = ?;`, now, turn.ID); err != nil {
		return ChatTurnItem{}, fmt.Errorf("touch chat turn after item upsert: %w", err)
	}
	return s.getChatTurnItem(ctx, turn.ID, entryID)
}

// AppendChatTurnItemTranslatedContent 功能：把 thinking 翻译增量或失败态持久化到对应条目。
// 参数/返回：接收 turn 或 session/user 标识与翻译 delta；返回更新后的条目。
// 失败场景：目标 turn/item 不存在或写库失败时返回 error。
// 副作用：更新 SQLite `chat_turn_items` 与所属 turn 的翻译状态字段。
func (s *Store) AppendChatTurnItemTranslatedContent(ctx context.Context, params AppendChatTurnItemTranslatedContentParams) (ChatTurnItem, error) {
	if s == nil || s.db == nil {
		return ChatTurnItem{}, fmt.Errorf("store not initialized")
	}
	entryID := strings.TrimSpace(params.EntryID)
	if entryID == "" {
		return ChatTurnItem{}, fmt.Errorf("%w: entry_id is required", ErrValidation)
	}
	turn, err := s.lookupChatTurn(ctx, params.TurnID, params.SessionID, params.UserMessageID)
	if err != nil {
		return ChatTurnItem{}, err
	}
	now := time.Now().UnixMilli()
	setter := "COALESCE(translated_content, '') || ?"
	if params.Replace {
		setter = "?"
	}
	query := fmt.Sprintf(`UPDATE chat_turn_items
	    SET translated_content = %s,
	        translation_failed = CASE WHEN ? != 0 THEN 1 ELSE translation_failed END,
	        updated_at = ?
	  WHERE turn_id = ? AND entry_id = ?;`, setter)
	if _, err := s.db.ExecContext(
		ctx,
		query,
		strings.TrimSpace(params.Delta),
		boolToInt(params.Failed),
		now,
		turn.ID,
		entryID,
	); err != nil {
		return ChatTurnItem{}, fmt.Errorf("append chat turn item translation: %w", err)
	}
	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE chat_turns
		    SET thinking_translation_applied = 1,
		        thinking_translation_failed = CASE WHEN ? != 0 THEN 1 ELSE thinking_translation_failed END,
		        updated_at = ?
		  WHERE id = ?;`,
		boolToInt(params.Failed),
		now,
		turn.ID,
	); err != nil {
		return ChatTurnItem{}, fmt.Errorf("update chat turn translation state: %w", err)
	}
	return s.getChatTurnItem(ctx, turn.ID, entryID)
}

// CompleteChatTurn 功能：把一轮对话更新为完成态，并保存恢复所需的 turn 级元信息。
// 参数/返回：接收 turn 或 session/user/assistant 关联和统计元信息；返回完成后的 turn。
// 失败场景：目标 turn 不存在、assistant_message_id 为空或写库失败时返回 error。
// 副作用：更新 SQLite `chat_turns` 的完成态与恢复字段。
func (s *Store) CompleteChatTurn(ctx context.Context, params CompleteChatTurnParams) (ChatTurn, error) {
	if s == nil || s.db == nil {
		return ChatTurn{}, fmt.Errorf("store not initialized")
	}
	assistantMessageID := strings.TrimSpace(params.AssistantMessageID)
	if assistantMessageID == "" {
		return ChatTurn{}, fmt.Errorf("%w: assistant_message_id is required", ErrValidation)
	}
	turn, err := s.lookupChatTurn(ctx, params.TurnID, params.SessionID, params.UserMessageID)
	if err != nil {
		return ChatTurn{}, err
	}
	now := time.Now().UnixMilli()
	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE chat_turns
		    SET assistant_message_id = ?,
		        status = 'completed',
		        model_input = COALESCE(?, model_input),
		        context_mode = COALESCE(?, context_mode),
		        thinking_translation_applied = CASE WHEN ? != 0 THEN 1 ELSE 0 END,
		        thinking_translation_failed = CASE WHEN ? != 0 THEN 1 ELSE 0 END,
		        token_in = ?,
		        token_out = ?,
		        cached_input_tokens = ?,
		        updated_at = ?,
		        completed_at = ?
		  WHERE id = ?;`,
		assistantMessageID,
		trimOrNil(params.ModelInput),
		trimOrNil(params.ContextMode),
		boolToInt(params.ThinkingTranslationApplied),
		boolToInt(params.ThinkingTranslationFailed),
		params.TokenIn,
		params.TokenOut,
		params.CachedInputTokens,
		now,
		now,
		turn.ID,
	); err != nil {
		return ChatTurn{}, fmt.Errorf("complete chat turn: %w", err)
	}
	return s.getChatTurn(ctx, turn.ID)
}

// FailChatTurn 功能：把一轮对话标记为失败，保留已持久化的过程条目供刷新恢复。
// 参数/返回：接收 turn 或 session/user 标识；返回失败态 turn。
// 失败场景：目标 turn 不存在或写库失败时返回 error。
// 副作用：更新 SQLite `chat_turns.status/updated_at/completed_at`。
func (s *Store) FailChatTurn(ctx context.Context, params FailChatTurnParams) (ChatTurn, error) {
	if s == nil || s.db == nil {
		return ChatTurn{}, fmt.Errorf("store not initialized")
	}
	turn, err := s.lookupChatTurn(ctx, params.TurnID, params.SessionID, params.UserMessageID)
	if err != nil {
		return ChatTurn{}, err
	}
	now := time.Now().UnixMilli()
	if _, err := s.db.ExecContext(ctx, `UPDATE chat_turns SET status = 'failed', updated_at = ?, completed_at = COALESCE(completed_at, ?) WHERE id = ?;`, now, now, turn.ID); err != nil {
		return ChatTurn{}, fmt.Errorf("fail chat turn: %w", err)
	}
	return s.getChatTurn(ctx, turn.ID)
}

// GetChatTurnByUserMessage 功能：按 session 与 user message 定位单条 chat turn 并附带条目。
// 参数/返回：接收 sessionID 与 userMessageID；返回聚合后的 turn。
// 失败场景：记录不存在或查询失败时返回 error。
// 副作用：无；仅查询 SQLite。
func (s *Store) GetChatTurnByUserMessage(ctx context.Context, sessionID, userMessageID string) (ChatTurn, error) {
	if s == nil || s.db == nil {
		return ChatTurn{}, fmt.Errorf("store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	userMessageID = strings.TrimSpace(userMessageID)
	if sessionID == "" || userMessageID == "" {
		return ChatTurn{}, fmt.Errorf("%w: session_id and user_message_id are required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, session_id, user_message_id, assistant_message_id, turn, status, expert_id, provider, model, model_input, context_mode, thinking_translation_applied, thinking_translation_failed, token_in, token_out, cached_input_tokens, created_at, updated_at, completed_at FROM chat_turns WHERE session_id = ? AND user_message_id = ?;`, sessionID, userMessageID)
	turn, err := scanChatTurn(row)
	if err != nil {
		if isNoRows(err) {
			return ChatTurn{}, os.ErrNotExist
		}
		return ChatTurn{}, fmt.Errorf("query chat turn: %w", err)
	}
	items, err := s.listChatTurnItems(ctx, []string{turn.ID})
	if err != nil {
		return ChatTurn{}, err
	}
	turn.Items = items[turn.ID]
	return turn, nil
}

// ListChatTurns 功能：读取一个会话可恢复的 turn 快照及其有序条目。
// 参数/返回：接收 sessionID 与可选 limit；返回按 turn 升序聚合的结果。
// 失败场景：session_id 非法或查询失败时返回 error。
// 副作用：无；仅查询 SQLite。
func (s *Store) ListChatTurns(ctx context.Context, sessionID string, limit int) ([]ChatTurn, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, session_id, user_message_id, assistant_message_id, turn, status, expert_id, provider, model, model_input, context_mode, thinking_translation_applied, thinking_translation_failed, token_in, token_out, cached_input_tokens, created_at, updated_at, completed_at
		   FROM (
			 SELECT id, session_id, user_message_id, assistant_message_id, turn, status, expert_id, provider, model, model_input, context_mode, thinking_translation_applied, thinking_translation_failed, token_in, token_out, cached_input_tokens, created_at, updated_at, completed_at
			   FROM chat_turns
			  WHERE session_id = ?
			  ORDER BY turn DESC, created_at DESC
			  LIMIT ?
		 )
		  ORDER BY turn ASC, created_at ASC;`,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query chat turns: %w", err)
	}
	defer rows.Close()
	turns := make([]ChatTurn, 0)
	turnIDs := make([]string, 0)
	for rows.Next() {
		turn, err := scanChatTurn(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chat turn: %w", err)
		}
		turns = append(turns, turn)
		turnIDs = append(turnIDs, turn.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat turns: %w", err)
	}
	itemsByTurn, err := s.listChatTurnItems(ctx, turnIDs)
	if err != nil {
		return nil, err
	}
	filtered := make([]ChatTurn, 0, len(turns))
	for i := range turns {
		turns[i].Items = itemsByTurn[turns[i].ID]
		if len(turns[i].Items) == 0 && turns[i].AssistantMessageID == nil {
			continue
		}
		filtered = append(filtered, turns[i])
	}
	return filtered, nil
}

func (s *Store) lookupChatTurn(ctx context.Context, turnID, sessionID, userMessageID string) (ChatTurn, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID != "" {
		return s.getChatTurn(ctx, turnID)
	}
	return s.GetChatTurnByUserMessage(ctx, sessionID, userMessageID)
}

func (s *Store) getChatTurn(ctx context.Context, turnID string) (ChatTurn, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return ChatTurn{}, fmt.Errorf("%w: turn_id is required", ErrValidation)
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, session_id, user_message_id, assistant_message_id, turn, status, expert_id, provider, model, model_input, context_mode, thinking_translation_applied, thinking_translation_failed, token_in, token_out, cached_input_tokens, created_at, updated_at, completed_at FROM chat_turns WHERE id = ?;`, turnID)
	turn, err := scanChatTurn(row)
	if err != nil {
		if isNoRows(err) {
			return ChatTurn{}, os.ErrNotExist
		}
		return ChatTurn{}, fmt.Errorf("query chat turn by id: %w", err)
	}
	items, err := s.listChatTurnItems(ctx, []string{turn.ID})
	if err != nil {
		return ChatTurn{}, err
	}
	turn.Items = items[turn.ID]
	return turn, nil
}

func (s *Store) getChatTurnItem(ctx context.Context, turnID, entryID string) (ChatTurnItem, error) {
	row := s.db.QueryRowContext(ctx, `SELECT entry_id, seq, kind, status, content_text, translated_content, translation_failed, meta_json, created_at, updated_at FROM chat_turn_items WHERE turn_id = ? AND entry_id = ?;`, turnID, entryID)
	item, err := scanChatTurnItem(row)
	if err != nil {
		if isNoRows(err) {
			return ChatTurnItem{}, os.ErrNotExist
		}
		return ChatTurnItem{}, fmt.Errorf("query chat turn item: %w", err)
	}
	return item, nil
}

func (s *Store) listChatTurnItems(ctx context.Context, turnIDs []string) (map[string][]ChatTurnItem, error) {
	out := make(map[string][]ChatTurnItem)
	if len(turnIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(turnIDs))
	args := make([]any, 0, len(turnIDs))
	for _, turnID := range turnIDs {
		turnID = strings.TrimSpace(turnID)
		if turnID == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, turnID)
	}
	if len(placeholders) == 0 {
		return out, nil
	}
	query := fmt.Sprintf(`SELECT turn_id, entry_id, seq, kind, status, content_text, translated_content, translation_failed, meta_json, created_at, updated_at FROM chat_turn_items WHERE turn_id IN (%s) ORDER BY seq ASC, created_at ASC;`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query chat turn items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var turnID string
		item, err := scanChatTurnItemWithTurnID(rows, &turnID)
		if err != nil {
			return nil, fmt.Errorf("scan chat turn item: %w", err)
		}
		out[turnID] = append(out[turnID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat turn items: %w", err)
	}
	for turnID := range out {
		sort.SliceStable(out[turnID], func(i, j int) bool { return out[turnID][i].Seq < out[turnID][j].Seq })
	}
	return out, nil
}

func scanChatTurn(s scanner) (ChatTurn, error) {
	var turn ChatTurn
	var expertID sql.NullString
	var provider sql.NullString
	var model sql.NullString
	var applied int
	var failed int
	if err := s.Scan(
		&turn.ID,
		&turn.SessionID,
		&turn.UserMessageID,
		&turn.AssistantMessageID,
		&turn.Turn,
		&turn.Status,
		&expertID,
		&provider,
		&model,
		&turn.ModelInput,
		&turn.ContextMode,
		&applied,
		&failed,
		&turn.TokenIn,
		&turn.TokenOut,
		&turn.CachedInputTokens,
		&turn.CreatedAt,
		&turn.UpdatedAt,
		&turn.CompletedAt,
	); err != nil {
		return ChatTurn{}, err
	}
	turn.ExpertID = strings.TrimSpace(expertID.String)
	turn.Provider = strings.TrimSpace(provider.String)
	turn.Model = strings.TrimSpace(model.String)
	turn.ThinkingTranslationApplied = applied != 0
	turn.ThinkingTranslationFailed = failed != 0
	return turn, nil
}

func scanChatTurnItem(s scanner) (ChatTurnItem, error) {
	return scanChatTurnItemWithTurnID(s, nil)
}

func scanChatTurnItemWithTurnID(s scanner, turnID *string) (ChatTurnItem, error) {
	var item ChatTurnItem
	var translated sql.NullString
	var failed int
	var metaJSON sql.NullString
	if turnID != nil {
		if err := s.Scan(turnID, &item.EntryID, &item.Seq, &item.Kind, &item.Status, &item.ContentText, &translated, &failed, &metaJSON, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return ChatTurnItem{}, err
		}
	} else {
		if err := s.Scan(&item.EntryID, &item.Seq, &item.Kind, &item.Status, &item.ContentText, &translated, &failed, &metaJSON, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return ChatTurnItem{}, err
		}
	}
	meta, err := unmarshalTurnMeta(metaJSON.String)
	if err != nil {
		return ChatTurnItem{}, err
	}
	if strings.TrimSpace(translated.String) != "" {
		if meta == nil {
			meta = map[string]any{}
		}
		meta["translated_content"] = strings.TrimSpace(translated.String)
	}
	if failed != 0 {
		if meta == nil {
			meta = map[string]any{}
		}
		meta["translation_failed"] = true
	}
	item.Meta = meta
	return item, nil
}

func marshalTurnMeta(meta map[string]any) (*string, error) {
	if len(meta) == 0 {
		return nil, nil
	}
	buf, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal chat turn item meta: %w", err)
	}
	text := string(buf)
	return &text, nil
}

func unmarshalTurnMeta(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("unmarshal chat turn item meta: %w", err)
	}
	if len(meta) == 0 {
		return nil, nil
	}
	return meta, nil
}

func cloneMetaMap(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]any, len(meta))
	for key, value := range meta {
		out[key] = value
	}
	return out
}

func stripTranslationMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := cloneMetaMap(meta)
	delete(out, "translated_content")
	delete(out, "translation_failed")
	if len(out) == 0 {
		return nil
	}
	return out
}

func translatedMetaValue(meta map[string]any) *string {
	if len(meta) == 0 {
		return nil
	}
	value, _ := meta["translated_content"].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func translationFailedValue(meta map[string]any) bool {
	if len(meta) == 0 {
		return false
	}
	value, _ := meta["translation_failed"].(bool)
	return value
}

func isNoRows(err error) bool {
	return err == sql.ErrNoRows || strings.Contains(strings.ToLower(err.Error()), "no rows in result set")
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func pointerStringValueOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
