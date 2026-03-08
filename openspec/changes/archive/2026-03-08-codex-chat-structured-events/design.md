## Context

当前 `vibe-tree` 的 Codex 聊天路径已经从 `codex exec --json` 切到优先使用 `codex app-server`，并能在 turn 运行中广播：

- `chat.turn.delta`：最终回答文本增量
- `chat.turn.thinking.delta`：thinking 或 progress 文本增量
- `chat.turn.completed`：最终 assistant message 与 reasoning 汇总

但实现上仍是“按 session 维护两个字符串缓冲区”：

- 后端把 `thinking/progress/tool` 大多映射成 `chat.turn.thinking.delta`；
- 前端把 thinking 和 answer 分别追加到两个字符串，再渲染进同一个 pending assistant 卡片。

这导致运行时语义丢失：用户无法明确区分“模型在想”“模型在执行命令”“模型在更新计划”“模型在请求用户输入”。

## Goals / Non-Goals

**Goals**
- 兼容 Codex app-server 的两类事件命名：文档化 `item/*` 与真实运行时 `codex/event/*`。
- 用统一结构化事件 `chat.turn.event` 表示不同运行时条目，并带稳定 `entry_id` 支持增量更新。
- 前端把活动 turn 的各类条目分层渲染，并在 turn 完成后把过程详情保留在最后一条 assistant message 下。
- 保持向后兼容：旧页面逻辑仍可依赖 `chat.turn.delta` / `chat.turn.thinking.delta`。

**Non-Goals**
- 不伪造 token 级 thinking 动画；thinking 仍保持上游真实 chunk。
- 不把结构化 turn feed 立即持久化到 SQLite；v1 先做运行时展示。
- 不改造非 Codex CLI 会话为完整结构化 feed；非 Codex 路径继续走旧 fallback 展示。

## Decisions

1. **新增 `chat.turn.event`，而不是替换旧事件**
- 方案：新事件负责结构化 feed；旧 `chat.turn.delta` / `chat.turn.thinking.delta` 继续并行广播。
- 原因：降低前端重构风险，兼容现有页面与调试脚本。

2. **统一事件载荷固定字段**
- 载荷固定为：`session_id`、`user_message_id`、`entry_id`、`kind`、`op`、`status`、`delta?`、`content?`、`meta?`。
- `kind` 取值：`progress | thinking | answer | tool | plan | question | system | error`。
- `op` 取值：`append | replace | upsert | complete`。
- 原因：让前端只实现一种 reducer，不为不同工具家族写多套逻辑。

3. **按 entry 聚合运行时状态**
- `thinking` 与 `answer` 各有一个活动 entry。
- `tool` 以 `call_id` 聚合 stdout/stderr/status。
- `plan`、`question`、`progress`、`system` 可独立存在。
- 原因：避免再次退化为“大字符串覆盖”。

4. **thinking translation 仅更新 thinking entry 的翻译视图**
- 方案：翻译增量广播为 `chat.turn.event(kind=thinking, op=upsert, meta.translated_content=...)`，旧 `chat.turn.thinking.translation.delta` 保留。
- 原因：避免把翻译结果混进 answer/tool 文本。

5. **完成后把 feed 挂到 assistant message，而不是直接清空**
- 方案：前端在 `chat.turn.completed` 时，把活动 feed 关联到 `assistant_message_id`，用于“查看本轮过程详情”。
- 原因：保留过程可回看，同时不影响 transcript 主结构。

## Data Flow

1. 用户发送消息，后端发 `chat.turn.started(session_id, user_message_id, ...)`。
2. Codex app-server 事件进入统一 parser，输出一个或多个结构化 turn entry 事件。
3. 后端广播：
   - 新：`chat.turn.event`
   - 旧：必要时仍发 `chat.turn.delta` / `chat.turn.thinking.delta`
4. 前端 store 基于 `session_id + user_message_id + entry_id` 更新活动 feed。
5. 页面按 `kind` 选择不同组件渲染。
6. turn 完成后，assistant message 持久化入库；前端把活动 feed 关联到该 message 并标记为完成。

## Risks / Trade-offs

- **真实 Codex 事件仍可能继续演化**：通过同时兼容 `item/*` 与 `codex/event/*` 降低风险，并在 parser 中保持未知事件可忽略。
- **前端状态复杂度上升**：通过 reducer 化的 `applyTurnEvent` 收敛状态更新，避免页面里散落字符串拼接逻辑。
- **历史消息无过程详情**：v1 只对本次页面生命周期内的新 turn 提供完整 feed；历史数据仍展示最终 assistant message。

## Migration Plan

1. 扩展后端 parser 与广播路径，先补全测试。
2. 前端新增 turn feed store 和渲染组件，保留旧字符串 fallback。
3. 页面切换到“优先 feed、fallback 字符串”的展示逻辑。
4. 验证通过后，将 OpenSpec delta specs 合并进基线并归档 change。
