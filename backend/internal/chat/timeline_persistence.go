package chat

import (
	"context"
	"fmt"
	"strings"

	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/store"
)

// startTurnTimeline 功能：在一轮 chat 正式执行前创建后端 canonical timeline。
// 参数/返回：接收 session、user message 与本轮模型元信息；返回创建后的 ChatTurn。
// 失败场景：store 未配置、参数非法或 SQLite 写入失败时返回 error。
// 副作用：写入 `chat_turns` 表，作为后续事件与完成态的归属记录。
func (m *Manager) startTurnTimeline(ctx context.Context, sess store.ChatSession, userMsg store.ChatMessage, expertID, provider, model, modelInput string) (store.ChatTurn, error) {
	if m == nil || m.store == nil {
		return store.ChatTurn{}, fmt.Errorf("chat manager not configured")
	}
	return m.store.StartChatTurn(ctx, store.CreateChatTurnParams{
		SessionID:                  sess.ID,
		UserMessageID:              userMsg.ID,
		Turn:                       userMsg.Turn,
		ExpertID:                   pointerOrNilString(expertID),
		Provider:                   pointerOrNilString(provider),
		Model:                      pointerOrNilString(model),
		ModelInput:                 pointerOrNilString(modelInput),
		ThinkingTranslationApplied: false,
	})
}

// persistTurnEvent 功能：把结构化 chat.turn.event 归并写入后端时间线快照。
// 参数/返回：接收 turnID 与事件载荷；持久化成功返回 nil。
// 失败场景：turnID/事件身份缺失或 store 写入失败时返回 error。
// 副作用：更新 `chat_turn_items` 与 `chat_turns.updated_at`。
func (m *Manager) persistTurnEvent(ctx context.Context, turnID string, payload chatTurnEventPayload) error {
	if m == nil || m.store == nil {
		return nil
	}
	params := store.UpsertChatTurnItemParams{
		TurnID:      strings.TrimSpace(turnID),
		EntryID:     strings.TrimSpace(payload.EntryID),
		Seq:         payload.Seq,
		Kind:        strings.TrimSpace(payload.Kind),
		Status:      strings.TrimSpace(payload.Status),
		Op:          strings.TrimSpace(payload.Op),
		Delta:       payload.Delta,
		ContentText: "",
		Meta:        payload.Meta,
	}
	if params.Op != "append" {
		params.ContentText = payload.Content
	}
	_, err := m.store.UpsertChatTurnItem(ctx, params)
	return err
}

// persistTurnTranslationDelta 功能：把 thinking 译文增量写入对应条目的 meta，并保持刷新可恢复。
// 参数/返回：接收 turnID、entryID、译文与替换模式；成功返回 nil。
// 失败场景：目标条目不存在或写库失败时返回 error。
// 副作用：更新 `chat_turn_items.meta_json`。
func (m *Manager) persistTurnTranslationDelta(ctx context.Context, turnID, entryID, delta string, replace bool) error {
	if m == nil || m.store == nil {
		return nil
	}
	_, err := m.store.AppendChatTurnItemTranslatedContent(ctx, store.AppendChatTurnItemTranslatedContentParams{
		TurnID:  strings.TrimSpace(turnID),
		EntryID: strings.TrimSpace(entryID),
		Delta:   delta,
		Replace: replace,
	})
	return err
}

// persistTurnTranslationFailed 功能：把 thinking 翻译失败标记写回对应条目，避免刷新后误判已翻译成功。
// 参数/返回：接收 turnID 与 entryID；成功返回 nil。
// 失败场景：目标条目不存在或写库失败时返回 error。
// 副作用：更新 `translation_failed` 元数据。
func (m *Manager) persistTurnTranslationFailed(ctx context.Context, turnID, entryID string) error {
	if m == nil || m.store == nil {
		return nil
	}
	failed := true
	_, err := m.store.AppendChatTurnItemTranslatedContent(ctx, store.AppendChatTurnItemTranslatedContentParams{
		TurnID:  strings.TrimSpace(turnID),
		EntryID: strings.TrimSpace(entryID),
		Failed:  failed,
	})
	return err
}

// completeTurnTimeline 功能：在 assistant message 落库后收敛 turn 元信息与最终状态。
// 参数/返回：接收 turn、assistant message 与上下文/usage；成功返回 nil。
// 失败场景：store 未配置、turn 不存在或写库失败时返回 error。
// 副作用：更新 `chat_turns` 完成态字段。
func (m *Manager) completeTurnTimeline(ctx context.Context, turn store.ChatTurn, assistantMsg store.ChatMessage, contextMode string, callMeta providerCallMeta, translationApplied, translationFailed bool) error {
	if m == nil || m.store == nil || strings.TrimSpace(turn.ID) == "" {
		return nil
	}
	_, err := m.store.CompleteChatTurn(ctx, store.CompleteChatTurnParams{
		TurnID:                     turn.ID,
		SessionID:                  turn.SessionID,
		UserMessageID:              turn.UserMessageID,
		AssistantMessageID:         assistantMsg.ID,
		ModelInput:                 pointerOrNilString(callMeta.ModelInput),
		ContextMode:                pointerOrNilString(contextMode),
		ThinkingTranslationApplied: translationApplied,
		ThinkingTranslationFailed:  translationFailed,
		TokenIn:                    callMeta.TokenIn,
		TokenOut:                   callMeta.TokenOut,
		CachedInputTokens:          callMeta.CachedInputTokens,
	})
	return err
}

// failTurnTimeline 功能：把运行中 turn 标记为失败，并记录错误文案以便刷新后恢复失败态。
// 参数/返回：接收 turn 与 error；成功返回 nil。
// 失败场景：store 写入失败时返回 error。
// 副作用：更新 `chat_turns.status/error_message/completed_at`。
func (m *Manager) failTurnTimeline(ctx context.Context, turn store.ChatTurn, err error) error {
	if m == nil || m.store == nil || strings.TrimSpace(turn.ID) == "" || err == nil {
		return nil
	}
	_, updateErr := m.store.FailChatTurn(ctx, store.FailChatTurnParams{
		TurnID:        turn.ID,
		SessionID:     turn.SessionID,
		UserMessageID: turn.UserMessageID,
	})
	return updateErr
}

func (m *Manager) persistTurnEventWarn(ctx context.Context, turnID string, payload chatTurnEventPayload) {
	if err := m.persistTurnEvent(ctx, turnID, payload); err != nil {
		logx.Warn("chat", "persist-turn-event", "chat turn event 落库失败", "err", err, "turn_id", turnID, "session_id", payload.SessionID, "user_message_id", payload.UserMessageID, "entry_id", payload.EntryID)
	}
}

func (m *Manager) persistTurnTranslationWarn(ctx context.Context, turnID, entryID, delta string, replace bool) {
	if err := m.persistTurnTranslationDelta(ctx, turnID, entryID, delta, replace); err != nil {
		logx.Warn("chat", "persist-turn-translation", "thinking 译文落库失败", "err", err, "turn_id", turnID, "entry_id", entryID)
	}
}

// completeTurnEntry 功能：把某个时间线条目以完成态写入后端快照，确保刷新后内容与最终 UI 一致。
// 参数/返回：接收 turnID、entryID、kind、content 与可选 meta；成功返回 nil。
// 失败场景：关键参数缺失或 store 写入失败时返回 error。
// 副作用：更新/创建 `chat_turn_items` 当前快照。
func (m *Manager) completeTurnEntry(ctx context.Context, turnID, entryID, kind, content string, meta map[string]any) error {
	if m == nil || m.store == nil {
		return nil
	}
	_, err := m.store.UpsertChatTurnItem(ctx, store.UpsertChatTurnItemParams{
		TurnID:      strings.TrimSpace(turnID),
		EntryID:     strings.TrimSpace(entryID),
		Kind:        strings.TrimSpace(kind),
		Status:      "done",
		Op:          "complete",
		ContentText: content,
		Meta:        meta,
	})
	return err
}

// replaceTurnTranslation 功能：用最终译文覆盖条目上的译文快照，避免 raw->summary 切换后残留旧译文。
// 参数/返回：接收 turnID、entryID 与完整译文；成功返回 nil。
// 失败场景：目标条目不存在或写库失败时返回 error。
// 副作用：覆盖 `translated_content` 元数据。
func (m *Manager) replaceTurnTranslation(ctx context.Context, turnID, entryID, translated string, failed bool) error {
	if m == nil || m.store == nil || strings.TrimSpace(entryID) == "" {
		return nil
	}
	_, err := m.store.AppendChatTurnItemTranslatedContent(ctx, store.AppendChatTurnItemTranslatedContentParams{
		TurnID:  strings.TrimSpace(turnID),
		EntryID: strings.TrimSpace(entryID),
		Delta:   translated,
		Replace: true,
		Failed:  failed,
	})
	return err
}

func (m *Manager) persistTurnTranslationFailedWarn(ctx context.Context, turnID, entryID string) {
	if err := m.persistTurnTranslationFailed(ctx, turnID, entryID); err != nil {
		logx.Warn("chat", "persist-turn-translation-failed", "thinking 翻译失败标记落库失败", "err", err, "turn_id", turnID, "entry_id", entryID)
	}
}
