## Why

当前 Chat 的 CLI 对话虽然已经切到了 `Codex CLI / Claude Code` 主路径，但还存在两个明显问题：

1. Chat 页面工具/模型选择器的模型下拉存在“项被勾选但输入框显示为空白”的 UI 问题。
2. 后端每次 turn 仍然重建并发送完整历史上下文给 CLI，而不是利用 CLI 自身的 `session_id / thread_id` 继续同一会话。

参考 `.github-feature-analyzer/` 中的 `fengshao1227-ccg-workflow` 与 `BloopAI-vibe-kanban`，更合理的做法是：首次运行记录 Codex `thread_id` 或 Claude `session_id`，后续 turn 只发送当前输入，并通过 CLI 自己的 resume/fork 机制恢复上下文。

## What Changes

- 修复 Chat 页面两个模型选择器的显示问题，确保已选模型在选择框中可见。
- 为 chat session 增加 CLI 会话引用（如 `session_id/thread_id`）的持久化与读写能力。
- 更新 `codex_exec.sh` / `claude_exec.sh`，支持：
  - 首次运行时从 CLI 输出中提取并写出 `session.json`
  - 后续 turn 通过 `resume <session_id>` / `--resume <session_id>` 继续会话
- 更新 chat manager：优先使用已保存的 CLI session 继续会话，只在首次运行或恢复失败时回退到本地重建上下文。
- 保持 thinking translation、附件、fork、manual compact 与当前 WS 事件契约兼容。

## Capabilities

### New Capabilities
- `chat-cli-session-resume`: 持久化 CLI session/thread 标识，并在后续 turn 中复用 CLI 原生上下文恢复能力。

### Modified Capabilities
- `chat-session-memory`: chat 会话需要保存 CLI session 引用，并在后续 turn 中优先走 CLI resume。
- `ui`: Chat 页面需要正确显示工具优先的模型选择结果。
- `cli-runtime`: wrapper 需要写出 `session.json` 并支持 resume 模式。

## Impact

- 后端：`backend/internal/store/chat.go`、`backend/internal/store/migrate.go`、`backend/internal/api/chat.go`、`backend/internal/chat/manager.go`、`backend/internal/cliruntime/artifacts.go`
- wrapper：`scripts/agent-runtimes/codex_exec.sh`、`scripts/agent-runtimes/claude_exec.sh`
- 前端：`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/stores/chatStore.ts`、`ui/src/lib/daemon.ts`
- 数据：chat session schema 需要新增 CLI session 引用字段，并兼容旧数据读取
