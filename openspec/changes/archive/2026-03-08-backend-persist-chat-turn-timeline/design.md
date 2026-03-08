## Context

当前 `vibe-tree` 只把 chat session、最终 user/assistant message 和附件元数据持久化到 SQLite，Codex 运行中的 thinking、tool、plan、question、system、answer 时间线主要依赖前端运行态与 `sessionStorage` 维护。这样虽然页面在单次挂载期间可以显示完整过程，但刷新、重连或重新进入会话时，前端只能从后端恢复最终消息，无法稳定恢复本轮过程详情，导致时间线丢失、重复或错位。

本次变更跨越后端存储、聊天运行时、API 读取与前端恢复链，并且需要调整 SQLite schema，因此必须先统一“谁是事实源”的架构边界。

## Goals / Non-Goals

**Goals:**
- 将 Codex chat 过程时间线改为后端 canonical timeline，前端只消费快照与增量事件。
- 为每一轮 chat 持久化 turn 元数据与结构化条目，支持 completed 与 running 两类恢复。
- thinking 翻译、tool 输出、plan/question/progress 等过程内容刷新后仍可恢复。
- 保持现有消息 API 和 WebSocket 事件兼容，尽量减少对既有 UI 结构的破坏。
- 修正 reasoning 归并语义：`summaryTextDelta` 优先于 raw `textDelta`，避免重复字符。

**Non-Goals:**
- 不保存每一个原始 token 级 app-server delta 作为审计日志。
- 不为旧历史会话逆向补建过程时间线；旧会话仍按已有最终消息显示。
- 不改造非 Codex provider 为完整结构化时间线；本次重点是 Codex CLI 路径。

## Decisions

### 1. 新增 turn 与条目表，而不是把过程 JSON 塞进 `chat_messages`
- 选择：新增 `chat_turns` 与 `chat_turn_items` 两张表。
- 原因：`chat_messages` 负责 transcript 最终消息；过程时间线有 running/completed 生命周期、稳定 `entry_id`、`seq` 与聚合 `meta_json`，单独建模更清晰。
- 备选：把完整 feed JSON 塞入 assistant message 扩展字段。放弃原因是 running turn 没有 assistant message，且每次增量更新整块 JSON 容易放大写放大与冲突。

### 2. 后端保存“归并后的条目快照”，而不是保存所有原始 delta
- 选择：每个 `(session_id, user_message_id, entry_id)` 在库中只维护当前条目快照，按 `append/replace/upsert/complete` 增量更新。
- 原因：前端恢复只需要可见 UI 状态，不需要逐 token 回放；这样实现简单、查询稳定、存储压力更小。
- 备选：额外新增 `chat_turn_item_events` 逐条记原始 delta。放弃作为本期方案，后续若需要审计或精确回放再单独扩展。

### 3. API 采用“按 session 读取 turn + items 聚合快照”
- 选择：新增 `GET /api/v1/chat/sessions/:id/turns`，一次返回 session 下最近若干 turn 及其 items。
- 原因：页面进入会话时需要同时恢复 completed 与 running turn，聚合接口比让前端多次请求更稳。
- 备选：拆成 `turns` 和 `turn-items` 两个接口。本期不做，避免前端多次往返和排序拼装。

### 4. WebSocket 保留，但降级为“增量同步通道”
- 选择：`chat.turn.event` / `chat.turn.completed` 继续保留；后端先写库，再广播。
- 原因：兼容现有运行时展示，同时让刷新恢复不依赖浏览器本地状态。
- 备选：彻底废弃 WS，仅靠轮询接口。放弃原因是交互体验明显变差。

### 5. Thinking 翻译结果直接写回 thinking 条目
- 选择：翻译增量继续广播，但同时把 `translated_content` 写入对应 thinking item 的 `meta_json`，turn 级 summary 字段也同步更新。
- 原因：刷新后必须仍能优先显示中文翻译，而不是退化回英文。
- 备选：仍只在前端保存译文。放弃原因是这正是当前刷新丢失的根因之一。

### 6. summary reasoning 优先，raw reasoning 仅作后备
- 选择：对同一 reasoning item，一旦已收到 `summaryTextDelta`，后续 raw `textDelta` 不再进入可见 timeline 条目。
- 原因：官方 app-server 语义中 summary 与 raw 是不同层级；同时展示会造成重复字符和理解噪音。
- 备选：同时保存并让前端决定显示哪一种。放弃原因是会把重复处理责任再次推给前端。

## Risks / Trade-offs

- **SQLite 写入频率上升** → 通过按 entry upsert 与短周期批量 flush 降低每个 chunk 的写放大；仍保留单连接 WAL 模式。
- **数据模型变复杂** → 通过把 turn 和 item 明确分层，保持查询和 UI 投影清晰；同时补足 store/API 测试。
- **新旧会话并存** → 对没有 timeline 数据的旧会话，前端回退为只显示最终消息，避免伪造不存在的过程。
- **前后端同步改动面大** → 先保持现有消息 API 与 WS 事件兼容，再把前端 runtime 缓存降为辅助状态，减少一次性切换风险。

## Migration Plan

1. 为 SQLite 增加 `chat_turns` 与 `chat_turn_items` 表及索引，提升 `user_version`。
2. 后端 store 增加 turn/item upsert 与 list 接口，聊天运行时在广播前写库。
3. 新增 `GET /api/v1/chat/sessions/:id/turns`，前端进入会话时并行加载 `messages` 与 `turns`。
4. 前端改为以 `turns` 快照构建 completed / running timeline，`sessionStorage` 仅保留视图级状态。
5. 发布后新 turn 自动具备可恢复 timeline；旧 turn 继续按历史 transcript 展示。
6. 若出现严重回归，可在前端暂时回退为仅显示 `messages`，但保持后端 schema 不回滚。

## Open Questions

- 本期默认按 session 返回“全部 turn + items”；若后续单会话 turn 数量持续增长，再补分页或按最近 N 轮裁剪。
- 本期不把 tool output 的原始 delta 单独落事件表；若后续需要逐 chunk 回放，再单独提出二期变更。
