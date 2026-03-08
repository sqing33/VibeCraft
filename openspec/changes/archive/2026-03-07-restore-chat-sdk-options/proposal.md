## Why

当前 `#/chat` 页面把可选对话入口收敛成了两个 CLI 工具。对于只想做简单问答的场景，CLI runtime 会天然带入更重的工作流语义与项目上下文读取，交互成本偏高，也和此前“普通 SDK 对话”体验不一致。

需要把 Chat 页的 SDK 入口加回来，让用户在同一处既能继续使用 `Codex CLI` / `Claude Code`，也能直接选择 `OpenAI SDK` / `Anthropic SDK` 发起普通对话。

## What Changes

- Chat 页的新建会话与消息发送选择器改为“混合运行时”列表：保留两个 CLI 工具，并增加两个 SDK 选项。
- 当用户选择 SDK 选项时，模型下拉只展示对应 provider 的模型，并通过 SDK chat 路径发送，不再走 CLI 会话续聊语义。
- Chat create-session / turn API 放开对 chat-capable SDK helper expert 的校验，允许 UI 用 `expert_id + model_id` 启动/继续 SDK 对话。
- 会话切换、消息身份展示与流式状态继续正确回填当前运行时与模型信息。
- 为该行为补充 OpenSpec delta 与后端集成测试，锁住“CLI + SDK 混合选择”能力。

## Capabilities

### New Capabilities

<!-- None -->

### Modified Capabilities

- `chat-session-memory`: Chat 会话创建/发送能力从 CLI-first 扩展为 CLI + SDK 并存，并允许 chat-capable SDK experts 作为直接对话运行时。
- `ui`: Chat 页的运行时/模型选择器从仅 CLI 工具扩展为 CLI 工具 + SDK provider 混合选择。

## Impact

- 前端：`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/stores/chatStore.ts`、`ui/src/lib/daemon.ts`
- 后端：`backend/internal/api/chat.go`
- 测试：`backend/internal/api/chat_integration_test.go`
- 规范：`openspec/specs/chat-session-memory/spec.md`、`openspec/specs/ui/spec.md`
