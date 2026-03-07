## Why

最近的 Chat CLI-first 改造虽然已经引入了 `cli_session_id` 和 wrapper resume 参数，但当前体验仍不稳定：
- Chat 页面两个模型选择器存在“选中了但输入框显示空白”的问题。
- 用户观察到后续提问仍像是在发送完整上下文，而不是稳定依赖 CLI 自身的 resume/session 机制。
- 线上报错还暴露出旧数据库可能未及时升级到包含 `cli_tool_id` 的 schema，需要确保迁移与读取兼容足够稳健。

因此需要对 Chat CLI resume 这条链做一次稳定性修正，而不是再扩新能力。

## What Changes

- 修复 Chat 页面工具优先模型选择器的显示逻辑，确保已选模型标签在选择框中稳定显示。
- 稳定 Chat CLI resume：优先使用已保存的 `cli_session_id`，仅在 resume 失败或不存在时回退到本地重建 prompt。
- 强化 wrapper 对 `session_id/thread_id` 的提取和 `session.json` 写出。
- 保持 store schema 兼容旧数据库，确保 `cli_tool_id/model_id/cli_session_id` 列在旧库上也能被安全迁移。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `chat-cli-session-resume`: 稳定 Codex/Claude CLI 的原生续聊恢复。
- `chat-session-memory`: 会话元数据与 schema 迁移兼容性增强。
- `ui`: 修复 Chat 模型选择器显示。
- `cli-runtime`: wrapper 的 session 解析与 resume 契约增强。

## Impact

- 后端：`backend/internal/store/migrate.go`, `backend/internal/store/chat.go`, `backend/internal/chat/manager.go`, `backend/internal/cliruntime/artifacts.go`
- wrapper：`scripts/agent-runtimes/codex_exec.sh`, `scripts/agent-runtimes/claude_exec.sh`
- 前端：`ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/lib/daemon.ts`
