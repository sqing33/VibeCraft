package chat

import (
	"context"
	"encoding/json"
	"testing"
)

// TestCodexTurnFeedEmitterSplitsThinkingSegments 功能：校验 thinking→tool→thinking 会拆成独立时间线片段并保持稳定 seq。
// 参数/返回：无外部参数；断言 emitter 收到的 payload 顺序、entry_id 与 seq。
// 失败场景：thinking 未拆段、tool 更新换 seq 或顺序错乱时测试失败。
// 副作用：无；仅构造内存态 emitter 与事件载荷。
func TestCodexTurnFeedEmitterSplitsThinkingSegments(t *testing.T) {
	emitter := newCodexTurnFeedEmitter(nil, "ct_1", "sess_1", "msg_1", nil)
	payloads := make([]chatTurnEventPayload, 0, 6)
	emitter.sink = func(payload chatTurnEventPayload) {
		payloads = append(payloads, payload)
	}
	ctx := context.Background()

	emitter.consume(ctx, "item/reasoning/summaryTextDelta", json.RawMessage(`{"delta":"先思考一下"}`))
	emitter.consume(ctx, "codex/event/exec_command_begin", json.RawMessage(`{"callId":"cmd_1","command":"pwd"}`))
	emitter.consume(ctx, "codex/event/exec_command_output_delta", json.RawMessage(`{"callId":"cmd_1","stream":"stdout","chunk":"/tmp\n"}`))
	emitter.consume(ctx, "codex/event/exec_command_end", json.RawMessage(`{"callId":"cmd_1","exit_code":0,"success":true}`))
	emitter.consume(ctx, "item/reasoning/summaryTextDelta", json.RawMessage(`{"delta":"再继续分析"}`))

	if len(payloads) != 5 {
		t.Fatalf("expected 5 payloads, got %d: %+v", len(payloads), payloads)
	}
	if payloads[0].EntryID != "thinking:1" || payloads[0].Kind != "thinking" || payloads[0].Seq != 1 {
		t.Fatalf("unexpected first payload: %+v", payloads[0])
	}
	if payloads[1].EntryID != "tool:cmd_1" || payloads[1].Kind != "tool" || payloads[1].Seq != 2 {
		t.Fatalf("unexpected second payload: %+v", payloads[1])
	}
	if payloads[2].EntryID != "tool:cmd_1" || payloads[2].Seq != 2 {
		t.Fatalf("tool output should reuse seq=2: %+v", payloads[2])
	}
	if payloads[3].EntryID != "tool:cmd_1" || payloads[3].Seq != 2 || payloads[3].Status != "success" {
		t.Fatalf("tool completion should reuse seq=2: %+v", payloads[3])
	}
	if payloads[4].EntryID != "thinking:2" || payloads[4].Kind != "thinking" || payloads[4].Seq != 3 {
		t.Fatalf("unexpected final payload: %+v", payloads[4])
	}
}

// TestCodexTurnFeedEmitterSplitsSameReasoningItemAfterInterleave 功能：校验同一 itemId 在非 thinking 事件打断后会创建新的 thinking 条目。
// 参数/返回：无外部参数；断言第二段 reasoning 不会复用第一次的 entry_id。
// 失败场景：若同一 itemId 被错误复用为同一个 thinking 条目，则测试失败。
// 副作用：无；仅消费内存事件。
func TestCodexTurnFeedEmitterSplitsSameReasoningItemAfterInterleave(t *testing.T) {
	emitter := newCodexTurnFeedEmitter(nil, "ct_1", "sess_1", "msg_1", nil)
	payloads := make([]chatTurnEventPayload, 0, 4)
	emitter.sink = func(payload chatTurnEventPayload) {
		payloads = append(payloads, payload)
	}
	ctx := context.Background()

	emitter.consume(ctx, "item/reasoning/summaryTextDelta", json.RawMessage(`{"itemId":"rs_1","delta":"第一段"}`))
	emitter.consume(ctx, "codex/event/exec_command_begin", json.RawMessage(`{"callId":"cmd_1","command":"pwd"}`))
	emitter.consume(ctx, "item/reasoning/summaryTextDelta", json.RawMessage(`{"itemId":"rs_1","delta":"第二段"}`))

	if len(payloads) != 3 {
		t.Fatalf("expected 3 payloads, got %d: %+v", len(payloads), payloads)
	}
	if payloads[0].EntryID != "thinking:1" || payloads[0].Kind != "thinking" {
		t.Fatalf("unexpected first payload: %+v", payloads[0])
	}
	if payloads[1].EntryID != "tool:cmd_1" || payloads[1].Kind != "tool" {
		t.Fatalf("unexpected second payload: %+v", payloads[1])
	}
	if payloads[2].EntryID != "thinking:2" || payloads[2].Kind != "thinking" {
		t.Fatalf("expected second reasoning segment to use new entry, got %+v", payloads[2])
	}
}

// TestCodexTurnFeedEmitterSkipsToolPlaceholder 功能：校验没有真实命令内容时不会输出 tool 占位条目。
// 参数/返回：无外部参数；断言空命令 begin 事件不会产生 payload。
// 失败场景：若仍然发出 `command execution` 之类的占位条目，则测试失败。
// 副作用：无；仅在内存中消费单条事件。
func TestCodexTurnFeedEmitterSkipsToolPlaceholder(t *testing.T) {
	emitter := newCodexTurnFeedEmitter(nil, "ct_1", "sess_1", "msg_1", nil)
	payloads := make([]chatTurnEventPayload, 0, 1)
	emitter.sink = func(payload chatTurnEventPayload) {
		payloads = append(payloads, payload)
	}
	emitter.consume(context.Background(), "codex/event/exec_command_begin", json.RawMessage(`{"callId":"cmd_1"}`))
	if len(payloads) != 0 {
		t.Fatalf("expected no payload for placeholder tool entry, got %+v", payloads)
	}
}

// TestCodexTurnFeedEmitterPrefersSummaryReasoning 功能：校验同一 reasoning item 出现 summary 后会覆盖 raw 文本并禁止后续 raw 重复追加。
// 参数/返回：无外部参数；断言 thinking payload 的 op 与内容更新顺序。
// 失败场景：summary 没有 replace raw，或后续 raw 仍追加导致重复时测试失败。
// 副作用：无；仅在内存中消费结构化事件。
func TestCodexTurnFeedEmitterPrefersSummaryReasoning(t *testing.T) {
	emitter := newCodexTurnFeedEmitter(nil, "ct_1", "sess_1", "msg_1", nil)
	payloads := make([]chatTurnEventPayload, 0, 3)
	emitter.sink = func(payload chatTurnEventPayload) {
		payloads = append(payloads, payload)
	}
	ctx := context.Background()

	emitter.consume(ctx, "item/reasoning/textDelta", json.RawMessage(`{"itemId":"rs_1","delta":"raw-1"}`))
	emitter.consume(ctx, "item/reasoning/summaryTextDelta", json.RawMessage(`{"itemId":"rs_1","delta":"summary-1"}`))
	emitter.consume(ctx, "item/reasoning/textDelta", json.RawMessage(`{"itemId":"rs_1","delta":"raw-2"}`))

	if len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got %d: %+v", len(payloads), payloads)
	}
	if payloads[0].EntryID != "thinking:1" || payloads[0].Op != "append" || payloads[0].Delta != "raw-1" {
		t.Fatalf("unexpected raw payload: %+v", payloads[0])
	}
	if payloads[1].EntryID != "thinking:1" || payloads[1].Op != "replace" || payloads[1].Delta != "summary-1" {
		t.Fatalf("summary should replace raw payload: %+v", payloads[1])
	}
}
