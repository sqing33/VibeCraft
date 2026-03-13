package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"vibecraft/backend/internal/id"
)

type ImportedChatMessage struct {
	ID                string
	Turn              int64
	Role              string
	ContentText       string
	ExpertID          *string
	Provider          *string
	Model             *string
	TokenIn           *int64
	TokenOut          *int64
	ProviderMessageID *string
	CreatedAt         int64
}

type ImportedChatTurnItem struct {
	EntryID     string
	Seq         int
	Kind        string
	Status      string
	ContentText string
	Meta        map[string]any
	CreatedAt   int64
	UpdatedAt   int64
}

type ImportedChatTurn struct {
	ID                         string
	UserMessageID              string
	AssistantMessageID         *string
	Turn                       int64
	Status                     string
	ExpertID                   *string
	Provider                   *string
	Model                      *string
	ModelInput                 *string
	ContextMode                *string
	ThinkingTranslationApplied bool
	ThinkingTranslationFailed  bool
	TokenIn                    *int64
	TokenOut                   *int64
	CachedInputTokens          *int64
	CreatedAt                  int64
	UpdatedAt                  int64
	CompletedAt                *int64
	Items                      []ImportedChatTurnItem
}

type ImportChatSessionParams struct {
	Title           string
	ExpertID        string
	CLIToolID       *string
	ModelID         *string
	ReasoningEffort *string
	CLISessionID    *string
	MCPServerIDs    []string
	Provider        string
	Model           string
	WorkspacePath   string
	Status          string
	Summary         *string
	CreatedAt       int64
	UpdatedAt       int64
	LastTurn        int64
	Messages        []ImportedChatMessage
	Turns           []ImportedChatTurn
}

// GetChatSessionByCLIReference 功能：按 CLI tool + 原生 session/thread id 查找已有 chat session。
// 参数/返回：接收 `cli_tool_id` 与 `cli_session_id`；命中时返回 ChatSession。
// 失败场景：未命中返回 `os.ErrNotExist`；查询失败返回 error。
// 副作用：读取 SQLite `chat_sessions`。
func (s *Store) GetChatSessionByCLIReference(ctx context.Context, cliToolID, cliSessionID string) (ChatSession, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, fmt.Errorf("store not initialized")
	}
	toolID := strings.TrimSpace(cliToolID)
	sessionID := strings.TrimSpace(cliSessionID)
	if toolID == "" || sessionID == "" {
		return ChatSession{}, fmt.Errorf("%w: cli_tool_id and cli_session_id are required", ErrValidation)
	}
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, title, expert_id, cli_tool_id, model_id, reasoning_effort, cli_session_id, mcp_server_ids_json, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn
		   FROM chat_sessions
		  WHERE cli_tool_id = ? AND cli_session_id = ?
		  LIMIT 1;`,
		toolID,
		sessionID,
	)
	sess, err := scanChatSession(row)
	if err != nil {
		if isNoRows(err) {
			return ChatSession{}, os.ErrNotExist
		}
		return ChatSession{}, fmt.Errorf("query chat session by cli reference: %w", err)
	}
	return sess, nil
}

// ImportChatSession 功能：按显式 transcript 与 timeline 批量导入一个外部 chat session。
// 参数/返回：接收会话元信息、消息、turn 与条目；返回 session 与是否新建。
// 失败场景：参数非法、外键不满足或 SQLite 写入失败时返回 error。
// 副作用：事务性写入 `chat_sessions/chat_messages/chat_turns/chat_turn_items`。
func (s *Store) ImportChatSession(ctx context.Context, params ImportChatSessionParams) (ChatSession, bool, error) {
	if s == nil || s.db == nil {
		return ChatSession{}, false, fmt.Errorf("store not initialized")
	}
	cliToolID := pointerStringValue(params.CLIToolID)
	cliSessionID := pointerStringValue(params.CLISessionID)
	if cliToolID != "" && cliSessionID != "" {
		existing, err := s.GetChatSessionByCLIReference(ctx, cliToolID, cliSessionID)
		if err == nil {
			return existing, false, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return ChatSession{}, false, err
		}
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "Imported Codex Session"
	}
	expertID := strings.TrimSpace(params.ExpertID)
	if expertID == "" {
		expertID = "codex"
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider != "openai" && provider != "anthropic" && provider != "demo" && provider != "cli" {
		return ChatSession{}, false, fmt.Errorf("%w: unsupported provider %q", ErrValidation, params.Provider)
	}
	model := strings.TrimSpace(params.Model)
	if model == "" {
		return ChatSession{}, false, fmt.Errorf("%w: model is required", ErrValidation)
	}
	workspace := strings.TrimSpace(params.WorkspacePath)
	if workspace == "" {
		workspace = "."
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = "active"
	}
	reasoningEffort, err := normalizeChatSessionReasoningEffort(params.ReasoningEffort)
	if err != nil {
		return ChatSession{}, false, err
	}

	now := time.Now().UnixMilli()
	createdAt := normalizeImportedTime(params.CreatedAt, now)
	updatedAt := normalizeImportedTime(params.UpdatedAt, createdAt)
	if updatedAt < createdAt {
		updatedAt = createdAt
	}
	lastTurn := params.LastTurn
	for _, msg := range params.Messages {
		if msg.Turn > lastTurn {
			lastTurn = msg.Turn
		}
		if ts := normalizeImportedTime(msg.CreatedAt, 0); ts > updatedAt {
			updatedAt = ts
		}
	}
	for _, turn := range params.Turns {
		if turn.Turn > lastTurn {
			lastTurn = turn.Turn
		}
		if ts := normalizeImportedTime(turn.UpdatedAt, 0); ts > updatedAt {
			updatedAt = ts
		}
		if completedAt := normalizeImportedPointerTime(turn.CompletedAt); completedAt != nil && *completedAt > updatedAt {
			updatedAt = *completedAt
		}
		for _, item := range turn.Items {
			if ts := normalizeImportedTime(item.UpdatedAt, 0); ts > updatedAt {
				updatedAt = ts
			}
		}
	}

	session := ChatSession{
		ID:              id.New("cs_"),
		Title:           title,
		ExpertID:        expertID,
		CLIToolID:       trimOrNil(params.CLIToolID),
		ModelID:         trimOrNil(params.ModelID),
		ReasoningEffort: reasoningEffort,
		CLISessionID:    trimOrNil(params.CLISessionID),
		MCPServerIDs:    cloneStringSlice(params.MCPServerIDs),
		Provider:        provider,
		Model:           model,
		WorkspacePath:   workspace,
		Status:          status,
		Summary:         trimOrNil(params.Summary),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		LastTurn:        lastTurn,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ChatSession{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO chat_sessions (id, title, expert_id, cli_tool_id, model_id, reasoning_effort, cli_session_id, mcp_server_ids_json, provider, model, workspace_path, status, summary, created_at, updated_at, last_turn)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		session.ID,
		session.Title,
		session.ExpertID,
		session.CLIToolID,
		session.ModelID,
		session.ReasoningEffort,
		session.CLISessionID,
		encodeStringSliceJSON(session.MCPServerIDs),
		session.Provider,
		session.Model,
		session.WorkspacePath,
		session.Status,
		session.Summary,
		session.CreatedAt,
		session.UpdatedAt,
		session.LastTurn,
	); err != nil {
		return ChatSession{}, false, fmt.Errorf("insert imported chat session: %w", err)
	}

	for index, msg := range params.Messages {
		if strings.TrimSpace(msg.ID) == "" {
			return ChatSession{}, false, fmt.Errorf("%w: imported message id is required", ErrValidation)
		}
		role := strings.TrimSpace(msg.Role)
		if role != "user" && role != "assistant" && role != "system" && role != "tool" {
			return ChatSession{}, false, fmt.Errorf("%w: unsupported role %q", ErrValidation, msg.Role)
		}
		content := msg.ContentText
		if strings.TrimSpace(content) == "" {
			return ChatSession{}, false, fmt.Errorf("%w: imported message content_text is required", ErrValidation)
		}
		createdAt := normalizeImportedTime(msg.CreatedAt, session.CreatedAt+int64(index))
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO chat_messages (id, session_id, turn, role, content_text, expert_id, provider, model, token_in, token_out, provider_message_id, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			msg.ID,
			session.ID,
			msg.Turn,
			role,
			content,
			trimOrNil(msg.ExpertID),
			trimOrNil(msg.Provider),
			trimOrNil(msg.Model),
			msg.TokenIn,
			msg.TokenOut,
			trimOrNil(msg.ProviderMessageID),
			createdAt,
		); err != nil {
			return ChatSession{}, false, fmt.Errorf("insert imported chat message: %w", err)
		}
	}

	for turnIndex, turn := range params.Turns {
		if strings.TrimSpace(turn.UserMessageID) == "" || turn.Turn <= 0 {
			return ChatSession{}, false, fmt.Errorf("%w: imported turn user_message_id and turn are required", ErrValidation)
		}
		turnID := strings.TrimSpace(turn.ID)
		if turnID == "" {
			turnID = id.New("ct_")
		}
		status := strings.TrimSpace(turn.Status)
		if status == "" {
			status = "completed"
		}
		createdAt := normalizeImportedTime(turn.CreatedAt, session.CreatedAt+int64(turnIndex))
		updatedAt := normalizeImportedTime(turn.UpdatedAt, createdAt)
		completedAt := normalizeImportedPointerTime(turn.CompletedAt)
		if completedAt == nil && status == "completed" {
			completedAt = &updatedAt
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO chat_turns (
				id, session_id, user_message_id, assistant_message_id, turn, status,
				expert_id, provider, model, model_input, context_mode,
				thinking_translation_applied, thinking_translation_failed,
				token_in, token_out, cached_input_tokens,
				created_at, updated_at, completed_at
			 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			turnID,
			session.ID,
			turn.UserMessageID,
			trimOrNil(turn.AssistantMessageID),
			turn.Turn,
			status,
			trimOrNil(turn.ExpertID),
			trimOrNil(turn.Provider),
			trimOrNil(turn.Model),
			trimOrNil(turn.ModelInput),
			trimOrNil(turn.ContextMode),
			boolToInt(turn.ThinkingTranslationApplied),
			boolToInt(turn.ThinkingTranslationFailed),
			turn.TokenIn,
			turn.TokenOut,
			turn.CachedInputTokens,
			createdAt,
			updatedAt,
			completedAt,
		); err != nil {
			return ChatSession{}, false, fmt.Errorf("insert imported chat turn: %w", err)
		}
		for itemIndex, item := range turn.Items {
			if strings.TrimSpace(item.EntryID) == "" || strings.TrimSpace(item.Kind) == "" {
				return ChatSession{}, false, fmt.Errorf("%w: imported turn item entry_id and kind are required", ErrValidation)
			}
			seq := item.Seq
			if seq <= 0 {
				seq = itemIndex + 1
			}
			itemStatus := strings.TrimSpace(item.Status)
			if itemStatus == "" {
				itemStatus = "done"
			}
			createdAt := normalizeImportedTime(item.CreatedAt, updatedAt)
			itemUpdatedAt := normalizeImportedTime(item.UpdatedAt, createdAt)
			meta := cloneMetaMap(item.Meta)
			metaJSON, err := marshalTurnMeta(stripTranslationMeta(meta))
			if err != nil {
				return ChatSession{}, false, err
			}
			var translated *string
			if value, ok := metaStringValue(meta, "translated_content"); ok {
				translated = &value
			}
			failed := 0
			if metaBoolValue(meta, "translation_failed") {
				failed = 1
			}
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO chat_turn_items (
					turn_id, entry_id, seq, kind, status, content_text,
					translated_content, translation_failed, meta_json, created_at, updated_at
				 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
				turnID,
				item.EntryID,
				seq,
				strings.TrimSpace(item.Kind),
				itemStatus,
				item.ContentText,
				translated,
				failed,
				metaJSON,
				createdAt,
				itemUpdatedAt,
			); err != nil {
				return ChatSession{}, false, fmt.Errorf("insert imported chat turn item: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return ChatSession{}, false, fmt.Errorf("commit imported chat session: %w", err)
	}
	return session, true, nil
}

func normalizeImportedTime(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	if value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

func normalizeImportedPointerTime(value *int64) *int64 {
	if value == nil {
		return nil
	}
	normalized := normalizeImportedTime(*value, 0)
	if normalized <= 0 {
		return nil
	}
	return &normalized
}

func metaStringValue(meta map[string]any, key string) (string, bool) {
	if len(meta) == 0 {
		return "", false
	}
	value, ok := meta[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}

func metaBoolValue(meta map[string]any, key string) bool {
	if len(meta) == 0 {
		return false
	}
	value, ok := meta[key]
	if !ok {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}
