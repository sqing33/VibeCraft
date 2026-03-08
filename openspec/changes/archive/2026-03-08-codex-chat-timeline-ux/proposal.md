## Why

当前 Codex 对话虽然已经有结构化运行时 feed，但所有 thinking 仍被合并进同一个条目，导致 `thinking → 命令 → thinking` 这类真实过程在 UI 上塌成一个大思考框。与此同时，命令输出默认整段展开，过程区噪音过高，用户很难快速扫读关键步骤。

## What Changes

- 为 Codex 结构化运行时 feed 增加稳定的时间线顺序字段，并把 thinking 拆成可按时间顺序展示的多个片段。
- 约束 thinking 分段规则：连续 reasoning 复用当前思考片段；一旦穿插 tool / plan / question / system / progress，后续 reasoning 必须开启新片段。
- 思考翻译事件增加可选 `entry_id`，确保译文增量附着到对应的 thinking 片段，而不是覆盖到错误条目。
- 前端 reducer 改为按时间线顺序维护 feed，不再把 answer 单独抽到末尾，也不再把所有 thinking 合并成一个展示区。
- 命令卡片默认只展示命令与摘要，`stdout/stderr` 默认折叠，点击后才展开查看。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `cli-chat-streaming`: 结构化运行时 feed 增加稳定时间线排序语义。
- `cli-chat-thinking`: Codex thinking 需要按真实交错过程拆分为多个独立片段。
- `thinking-translation`: 翻译增量需要绑定到具体 thinking 条目。
- `ui`: 聊天过程区改为紧凑时间线，并让命令输出默认折叠。

## Impact

- 后端：`backend/internal/chat/codex_turn_feed.go`、`backend/internal/chat/codex_appserver.go`、`backend/internal/chat/thinking_translation.go`、相关测试。
- 前端：`ui/src/lib/chatTurnFeed.ts`、`ui/src/stores/chatStore.ts`、`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/app/components/chat/ChatTurnFeed.tsx`。
- 协议：`chat.turn.event` 与 `chat.turn.thinking.translation.delta` payload 增强，但保持向后兼容。
