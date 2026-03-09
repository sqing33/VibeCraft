## Context

当前 Codex turn 的控制流是：

1. `runCodexAppServerTurn()` 每次 turn 都调用 `newCodexAppServerClient()`。
2. 新 client 总是重新启动一个 `codex app-server` 子进程。
3. 启动后再做 `Initialize()`。
4. 若 session 已存 `CLISessionID`，则调用 `thread/resume`；否则 `thread/start`。
5. turn 完成后 `defer client.Close()`，导致进程立即销毁。

因此目前“会话复用”只发生在线程 ID 层，而不是进程层。

## Goals / Non-Goals

**Goals**
- 同一 daemon 生命周期内，让同一 chat session 的 Codex app-server 保持热态。
- 保持现有 `thread/resume` 与本地重建 fallback 的兼容性。
- 让配置变化（模型、workspace、skills/MCP/base instructions）能够安全触发重建。
- 控制资源占用，避免暖进程永久泄漏。

**Non-Goals**
- 不改 Claude Code 路径。
- 不改变 chat 数据库 schema。
- 不把 app-server 进程跨 daemon 重启持久化。

## Decisions

### 1. 引入 session-scoped `codexRuntimePool`

新增内存池，按 `chat_session_id` 保存暖运行时条目。每个条目包含：
- `client`
- `threadID`
- `runtimeSignature`
- `lastUsedAt`
- 串行化访问锁

这样同一 session 的下一轮 turn 可以直接复用已初始化 client。

### 2. 用 `runtimeSignature` 判断是否允许复用

复用不仅取决于 `session_id`，还取决于会影响 Codex thread 行为的运行时配置是否一致：
- `model`
- `cwd`
- `baseInstructions`
- `config`（MCP、reasoning effort 等）

若签名变化，则关闭旧 client，并创建新的 app-server；若 session 已持久化 `thread_id`，新 client 再用 `thread/resume` 恢复上下文。

### 3. 暖 client 只在需要时 `thread/start` / `thread/resume`

- 若池中已有可复用条目且 thread 已就绪，则本轮直接 `turn/start`。
- 若池中 client 是新建的，但 session 已有 `CLISessionID`，先 `thread/resume`。
- 若没有历史 thread，则 `thread/start`。

这样能同时覆盖：
- 热复用路径：不再重复初始化。
- 冷恢复路径：daemon 重启后依然能恢复历史会话。

### 4. 用空闲 TTL + 显式失效控制资源

池条目在以下情况释放：
- 超过空闲 TTL（例如 10 分钟）
- session 被归档
- daemon shutdown
- client 调用失败且判断为不可继续复用

这样既能减少多轮会话延迟，也能避免后台子进程无限增长。

### 5. session 级串行化，避免同一 thread 并发写入

同一 `chat_session_id` 的暖运行时必须串行使用，避免两个 HTTP 请求同时向同一 thread 发 `turn/start`，导致事件流交叉。

## Risks / Trade-offs

- **内存/进程驻留增加**：通过 TTL 与显式失效控制。
- **配置漂移**：通过 `runtimeSignature` 强制重建解决。
- **client 意外退出**：复用失败时移除池条目，并回退到现有冷启动/重建 prompt 流程。
