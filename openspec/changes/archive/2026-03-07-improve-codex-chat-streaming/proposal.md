## Why

当前 `vibe-tree` 的 Codex 聊天链路仍然依赖 `codex exec --json` 的高层事件输出，后端只在 `item.completed` 时把整段 `agent_message` / `reasoning` 映射成 `chat.turn.delta` 与 `chat.turn.thinking.delta`。这会让前端表现成“长时间静止 → 一次性出现一大段思考/答案”，与用户预期的逐步展开体验不一致。

并行调研结果表明，更顺滑的项目并不是单纯前端做了动画，而是后端消费了更细粒度的事件：官方 Codex app-server 协议提供 `item/agentMessage/delta`、`item/reasoning/summaryTextDelta`、`item/reasoning/textDelta` 等通知；`BloopAI-vibe-kanban` 与 `iOfficeAI-AionUi` 也都是基于细粒度语义事件而不是 `item.completed` 才获得更流畅的体验。

需要把 Codex 聊天链路升级为“优先使用 app-server delta 流、保留 legacy wrapper 回退”的实现，同时保持现有前端 API 和会话恢复行为兼容。

## What Changes

- 为 Codex 聊天 turn 新增 app-server JSON-RPC 客户端，走 `thread/start` / `thread/resume` + `turn/start`，直接消费官方细粒度 delta 通知。
- Codex 聊天从 `item/agentMessage/delta` 流式推送答案，从 `item/reasoning/*Delta` 与 `item/plan/delta` 推送思考/进度，不再仅依赖 `item.completed`。
- 原有 `codex exec --json` 路径保留为回退方案：当 app-server 启动或初始化失败时自动退回 legacy wrapper，避免破坏现有用户环境。
- Chat turn 完成后继续写入 `cli_session_id`，但对 Codex 改为以 app-server `thread_id` 为真值，并继续支持失败时从本地重建 prompt 回退。
- Codex 聊天 turn 在 app-server 路径下补齐 token usage、`final_message.md` / `session.json` / `summary.json` / `artifacts.json` 等关键工件，保持 CLI runtime 产物契约。
- 前端保持现有 WS 消费接口不变，仅通过更细的后端事件频率获得更流畅的思考/答案展示。

## Capabilities

### Modified Capabilities

- `cli-chat-streaming`: Codex 聊天改为优先消费 app-server 细粒度 delta。
- `cli-chat-thinking`: Codex 思考/计划事件改为逐步映射，而不是只在 completed 时一次性出现。
- `chat-cli-session-resume`: Codex 会话恢复改为优先使用 app-server `thread/resume`。
- `cli-runtime`: Codex 聊天允许使用 app-server transport 作为 wrapper 之上的更细粒度实现。

## Impact

- 后端：`backend/internal/chat/*`、可能新增 Codex app-server client 文件；`backend/internal/cliruntime/*` 工件写入辅助；聊天单测。
- 前端：无需协议破坏性调整，但 `#/chat` 将收到更高频率的 `chat.turn.delta` / `chat.turn.thinking.delta`。
- 文档：同步 `PROJECT_STRUCTURE.md` 与 OpenSpec 基线 specs。
