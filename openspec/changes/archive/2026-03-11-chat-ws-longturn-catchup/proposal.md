## Why

长耗时（10 分钟+）的 Codex 聊天 turn 在高频增量输出下容易触发 WebSocket 下行背压，导致前端实时事件断档或卡住。后端仍在持续落库 turn timeline，所以刷新页面后又能恢复最新“思考过程”，但不刷新时用户体验较差且容易误判为已停止。

## What Changes

- 后端 WebSocket 广播在单帧内允许批量发送多个 envelope（JSON 数组），提升吞吐，降低慢客户端堆积导致的丢消息概率。
- 后端提升单客户端发送队列容量，并在写协程内批量 drain，减少 `WriteMessage` 次数。
- 前端 WebSocket 解析兼容“单个 envelope 或 envelope 数组”，逐个派发到现有事件总线。
- Chat 会话页在“turn 仍在进行但 WS 断开或长时间无增量事件”时自动轮询 `GET /api/v1/chat/sessions/:id/turns` 追帧（必要时联动刷新 messages），避免必须手动刷新才能看到进度。

## Capabilities

### New Capabilities

<!-- none -->

### Modified Capabilities

- `ui`: WebSocket 事件订阅支持批量帧；Chat 页在长任务 WS 不稳定时自动追帧，保证“对话中”状态可持续更新。
- `chat-turn-timeline`: 明确 turns API 作为 WS 断线/丢帧时的可恢复真相源，并补充“运行中 turn 可被轮询追帧”的场景。

## Impact

- 后端：`backend/internal/ws/hub.go` 广播写入策略变更（可能输出 JSON 数组帧）；相关 WS 集成测试需兼容。
- 前端：`ui/src/lib/ws.ts`、`ui/src/App.tsx`、`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/stores/chatStore.ts` 行为变更；其他消费 WS 的页面会受益于统一解析层兼容。
- 协议兼容：保留单 envelope 形态，同时新增数组形态，旧客户端若未升级可能无法解析数组帧。

