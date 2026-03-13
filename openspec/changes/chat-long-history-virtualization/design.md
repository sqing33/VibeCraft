## Context

当前 `#/chat` 的对话渲染采用 `messages.map(...)` 直接把历史消息全部挂载到 DOM。对话很长或单条消息很长时（导入历史、长时间运行的 session、超长 Markdown/文本），React 渲染与浏览器布局/绘制成本会指数上升，最终导致 UI 卡顿甚至卡死。

现状后端 `GET /api/v1/chat/sessions/:id/messages` 仅支持 `limit`，无法做“向上分页拉取更早消息”。UI `loadMessages` 也会覆盖 store 中的消息数组，不适合增量 prepend。

该变更同时涉及：
- UI 性能（虚拟列表、滚动跟随、prepend 不跳动）
- API 协议扩展（向后兼容的分页参数）
- Store/状态管理（覆盖 → 合并/去重）
- 新增前端依赖（`react-virtuoso`）

## Goals / Non-Goals

**Goals:**
- Chat transcript 在超长历史场景下保持可交互，不因 DOM 过大而卡死。
- 支持向上无限加载更早消息，且 prepend 后不出现“滚动位置跳动/倒退”。
- 保持底部自动跟随流式输出的体验：在底部时跟随，用户上滑后停止强制跟随，并提供“回到底部”入口。
- 后端分页能力向后兼容：不传新参数时行为不变。

**Non-Goals:**
- 不引入“单条超长消息默认折叠/摘要”策略（按需求选择不折叠）。
- 不改动 chat turn 的持久化结构与 WebSocket 事件协议。
- 不实现服务端基于时间戳/ID 的复杂游标；本次以 `turn` 作为分页锚点。

## Decisions

1) **UI 列表虚拟化采用 `react-virtuoso`**
- Why: 对动态高度内容（Markdown、附件块、details 折叠等）支持成熟，能显著降低长列表 DOM 节点数；内置 `followOutput`、`atBottomStateChange` 适合聊天/流式输出场景。
- Alternatives:
  - 仅用 CSS `content-visibility`：可缓解但 DOM 仍巨大，且对超长列表的内存/布局压力仍明显。
  - 仅做分页（只渲染最后 N 条）：实现简单但阅读/回溯体验较差。

2) **历史分页接口使用 `before_turn`（`turn < before_turn`）**
- Why: `turn` 在会话内单调递增且已在现有数据模型中稳定存在；实现简单，便于 UI 以“当前最早消息的 turn”为锚点向上拉取。
- Behavior:
  - `GET /api/v1/chat/sessions/:id/messages?limit=200`：保持现状，返回最近 N 条（升序）。
  - `...&before_turn=T`：返回 `turn < T` 的最近 N 条（升序）。
- Alternatives:
  - `before_message_id`：需要保证 message_id 的排序语义与索引策略，复杂度更高。
  - `offset`：插入/并发写入下体验较差，且对“稳定分页”不友好。

3) **ChatStore 消息加载由覆盖改为合并/去重**
- Why: 无限上拉需要 prepend；同时发送消息/WS catch-up 时需要 merge，避免把已加载的更早消息丢掉。
- Policy:
  - 基于 `message_id` 去重。
  - 最终数组按 `turn ASC, created_at ASC` 维持稳定顺序。

4) **prepend 不跳动采用 Virtuoso 的稳定索引策略**
- Approach: 使用固定 `BASE_INDEX` + `firstItemIndex = BASE_INDEX - messages.length`，并通过 `computeItemKey` 使用 `message_id` 作为 key，使 prepend 后视口保持稳定。
- Rationale: 避免手写“记录 scrollTop/scrollHeight 差值”在动态高度内容下不可靠的问题。

## Risks / Trade-offs

- [单条消息极长时可见区域仍可能卡顿] → 虚拟列表只保证“总量不炸”，不保证单条超长 Markdown 的解析/高亮成本；后续若仍有极端卡顿，可增加“超长消息折叠/懒解析”作为二期优化。
- [prepend 过程中测量抖动] → 使用 `react-virtuoso` 的 `firstItemIndex` 策略 + 适度 overscan（`increaseViewportBy`）降低跳动概率，并在手动验收中重点验证。
- [自动跟随与用户滚动冲突] → 以 `atBottomStateChange` 作为真相源；仅在 `atBottom=true` 时启用 `followOutput`，并在“用户发送消息”时强制回到底部。

