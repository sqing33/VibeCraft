## Why

当前 Chat 页面虽然已经支持 Codex `thread_id` 持久化与 `thread/resume`，但每一轮 turn 仍会重新启动一次 `codex app-server` 进程、重新做 `initialize` 与线程恢复。

这会带来两个直接问题：

1. 用户在同一会话发送第二条消息时，仍然会感受到一段重复的初始化等待。
2. Codex 的 MCP / runtime config 在每轮都重新冷启动，浪费本可在同一 daemon 生命周期内复用的会话热状态。

现状更像“持久化 thread id，但没有持久化 runtime 进程”。要真正减少多轮对话的初始化开销，需要把 Codex app-server 从“每轮临时进程”升级为“按 chat session 复用的暖运行时”。

## What Changes

- 为 Codex chat 引入后端内存级的 session-scoped app-server 运行时池。
- 同一 chat session 的后续 turn 优先复用已初始化的 app-server 进程，而不是每轮重新 `codex app-server --listen stdio://`。
- 当 daemon 重启、运行时过期、配置变化或线程失效时，仍保留当前 `thread/resume -> reconstructed prompt` 的冷恢复兜底。
- 在 session 归档时主动释放对应的暖运行时，避免后台残留空闲进程。
- 为 daemon shutdown 增加 chat runtime 清理，避免退出时遗留子进程。

## Capabilities

### Modified Capabilities
- `chat-cli-session-resume`: Codex 多轮对话除复用 `thread_id` 外，还需要在 daemon 生命周期内复用已初始化 app-server。
- `cli-runtime`: chat 运行时需要支持按 session 复用、按空闲超时回收、按 shutdown 统一清理。

## Impact

- 后端：`backend/internal/chat/manager.go`、`backend/internal/chat/codex_appserver.go`、`backend/internal/api/chat.go`
- 新增：`backend/internal/chat/codex_runtime_pool.go`
- 测试：`backend/internal/chat/codex_appserver_test.go`、`backend/internal/api/chat_integration_test.go`（或同类 chat backend tests）
- 文档：`PROJECT_STRUCTURE.md`
