## Why

当前 Chat 虽然已经切到 `Codex CLI / Claude Code` 主路径，但交互体验仍然停留在“命令完成后回一整块文本”：
- CLI 输出没有做真正的流式转发，前端只能在结尾看到整段回答。
- CLI 路径没有把中间的 reasoning / thinking / plan / tool 事件映射到前端，所以无法像 SDK chat 一样展示思考过程或执行进展。
- 参考项目显示：Claude Code 的 stream-json、Codex 的 json 事件、app-server/session 事件都可以作为增量消息源，不必等最终文件落盘。

因此需要新增一条完整能力：让 CLI chat 支持真正的增量流式输出，并尽可能展示 reasoning / thinking / tool / plan 相关事件。

## What Changes

- 为 CLI chat 新增真正的事件流通道：wrapper / manager 不再只在结束后发送一整块 delta，而是按增量事件实时广播 `chat.turn.*`。
- 为 Claude Code 解析 stream-json 的 assistant/thinking/session 事件，映射到 `chat.turn.delta` / `chat.turn.thinking.delta` / `chat.turn.completed`。
- 为 Codex 解析 JSON 事件，至少支持：assistant 文本增量、session/thread 建立、可能的 plan/tool 进度事件；如果没有稳定 reasoning 文本，则至少展示可用的中间进展事件。
- 保留 `final_message.md/summary.json/session.json` 作为最终 artifact 契约，但不再把它当作唯一消息来源。
- Chat 页面新增对 CLI 增量 thinking / plan / tool 事件的展示策略，并继续兼容 helper SDK 的 thinking translation。

## Capabilities

### New Capabilities
- `cli-chat-streaming`: CLI chat 增量输出与中间事件实时转发。
- `cli-chat-thinking`: CLI reasoning / thinking / plan / tool 事件展示策略。

### Modified Capabilities
- `chat-session-memory`: chat turn 的实时输出事件不再只依赖最终完成态。
- `cli-runtime`: wrapper 需要支持流式事件解析与统一输出契约。
- `ui`: Chat 页面需要渲染 CLI 的增量文本、思考过程和中间状态。

## Impact

- 后端：`backend/internal/chat/manager.go`, `backend/internal/cliruntime/artifacts.go`，可能需要新增 CLI 流事件解析 helper
- wrapper：`scripts/agent-runtimes/codex_exec.sh`, `scripts/agent-runtimes/claude_exec.sh`
- 前端：`ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/stores/chatStore.ts`, `ui/src/lib/ws.ts`/相关 envelope 消费逻辑
