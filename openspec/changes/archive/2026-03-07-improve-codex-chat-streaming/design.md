## Context

当前 Codex 聊天链路为：`expert.Resolve(provider=cli)` 产出 `bash codex_exec.sh` 规格，`chat.Manager.runCLITurn` 使用 PTY runner 启动 shell wrapper，逐行扫描 `codex exec --json` 输出，并在 `parseCodexCLIStreamEvents()` 中仅消费 `thread.started` / `sessionConfigured` / `item.completed`。这导致：

- `agent_message` 只有在 item 完成后才整体下发；
- `reasoning` 只有在 item 完成后才整体下发；
- 中间执行过程只有粗粒度 `command_execution` 文本；
- 前端虽然支持增量追加，但上游事件本身不够细。

同时，官方 Codex app-server 已经提供更细粒度的 JSON-RPC 事件模型：

- `item/agentMessage/delta`
- `item/reasoning/summaryTextDelta`
- `item/reasoning/textDelta`
- `item/plan/delta`
- `thread/tokenUsage/updated`

这意味着问题不需要通过重写前端解决，而是需要把 Codex 聊天的 transport 从“高层 exec JSON”提升到“app-server delta 流”。

## Goals / Non-Goals

**Goals**

- Codex 聊天答案按细粒度 delta 推给现有 `chat.turn.delta`。
- Codex 思考/计划按细粒度 delta 推给现有 `chat.turn.thinking.delta`。
- 维持现有 `POST /api/v1/chat/sessions/:id/turns` 与前端 store/WS 接口不变。
- 保留 `cli_session_id` / 本地重建回退语义。
- 在 app-server 路径下补充 token usage 与 CLI 工件写入。
- 当 app-server 不可用时安全回退到现有 wrapper 实现。

**Non-Goals**

- 不改造 Claude CLI 路径；本次只优化 Codex。
- 不改动 workflow / orchestration 的 Codex runtime；本次只改 chat turn。
- 不引入全新前端页面结构；保持现有 chat 页面和 WS 事件模型。
- 不实现官方全部 app-server item 类型的完整 UI，只聚焦用户感知最强的消息/思考/计划/usage。

## Decisions

1) **Codex 聊天单独走 app-server，其他 CLI 保持原样**

- 方案：在 `chat.Manager` 中检测 `VIBE_TREE_CLI_FAMILY=codex`，优先走新的 app-server client；其它 CLI 仍使用 legacy `runCLITurn` wrapper。
- 取舍：变更面最小，避免把 workflow/orchestration 与 Claude 路径一起卷进来。

2) **保留现有 WS 事件类型，不新增前端协议版本**

- 方案：`item/agentMessage/delta` 仍映射到 `chat.turn.delta`；`reasoning/plan/tool-progress` 继续映射到 `chat.turn.thinking.delta`。
- 取舍：前端无需大改，只靠更高频率事件就能明显改善体验。

3) **Codex 会话恢复改为 thread/resume，本地重建仍保留**

- 方案：若 `session.cli_session_id` 存在，先用 `thread/resume`；失败时重新启动 app-server 并走本地重建 prompt 的 `thread/start`。
- 取舍：既利用官方 thread 持久化，又兼容旧会话与异常场景。

4) **app-server 失败只在“早期失败”时回退 legacy wrapper**

- 方案：初始化 / 建线程阶段失败时回退 legacy `codex exec --json`；一旦 turn 已开始并发出细粒度 delta，就不再切换实现，避免重复输出。
- 取舍：降低重复流式输出风险，保持失败语义可预测。

5) **命令输出 delta 不直接原样灌入 thinking 区**

- 方案：优先流 reasoning / plan；对 `commandExecution` 只在 `item.started/completed` 提供简短进度提示，避免把大量命令 stdout 混进思考区。
- 取舍：聊天 UI 关注“模型在想什么/正在做什么”，不是复刻完整终端。

6) **补齐工件契约**

- 方案：即使 Codex 聊天绕过 shell wrapper，也仍在 `artifactDir` 下写 `final_message.md`、`session.json`、`summary.json`、`artifacts.json`。
- 取舍：维持 CLI runtime 契约的一致性，便于后续排障与回归。

## Implementation Plan

### 1. OpenSpec / docs

- 新增本 change 的 proposal/design/tasks。
- 更新 delta specs：`cli-chat-streaming`、`cli-chat-thinking`、`chat-cli-session-resume`、`cli-runtime`。

### 2. Codex app-server client

- 新增 `backend/internal/chat/codex_appserver.go`：
  - 启动 `codex app-server` stdio 进程；
  - 完成 `initialize` / `initialized` 握手；
  - 发送 `thread/start` 或 `thread/resume`；
  - 发送 `turn/start`；
  - 读取 JSON-RPC notifications 并分发；
  - 管理响应等待、stderr 捕获、优雅关闭。

### 3. Chat manager routing

- `RunTurn()` 调 `runCLITurn(..., thinkingTranslationSpec)`。
- `runCLITurn()` 变成 dispatcher：Codex 走 app-server 优先；失败后回退 legacy。
- 现有 wrapper 逻辑下沉为 `runLegacyCLITurn()`。

### 4. Delta mapping

- `item/agentMessage/delta` -> `chat.turn.delta`
- `item/reasoning/summaryTextDelta` / `item/reasoning/textDelta` -> `chat.turn.thinking.delta`
- `item/plan/delta` -> `chat.turn.thinking.delta`
- `item/completed(CommandExecution)` -> 简短 progress 文案
- `thread/tokenUsage/updated` -> 维护本 turn 的 token usage 快照
- `turn/completed` -> 生成最终 assistant message，广播 `chat.turn.completed`

### 5. Artifact + session persistence

- app-server 路径完成后写 `final_message.md`
- 写 `session.json`（`tool_id=codex`, `session_id=<thread_id>`）
- 写 `summary.json` / `artifacts.json`
- `chat_sessions.cli_session_id` 更新为 thread id

### 6. Tests

- 新增 app-server notification parser 单测：消息 delta / reasoning delta / token usage / item completed fallback。
- 维持 legacy parser 测试不变。

## Risks / Mitigations

### Risk: `codex app-server` 为 experimental，可能在用户机器上不可用

- 缓解：仅在 early failure 时自动回退 legacy wrapper。

### Risk: 细粒度事件过多导致前端噪音

- 缓解：限制命令输出仅转简短状态，不把完整 stdout 原样灌进 thinking。

### Risk: JSON-RPC server request 未处理导致阻塞

- 缓解：默认 `approval_policy=never` + `sandbox=danger-full-access`；若仍收到 server request，返回标准 JSON-RPC method-not-supported error，避免悬挂。

### Risk: 旧 `cli_session_id` 与 app-server thread 不兼容

- 缓解：`thread/resume` 失败时自动本地重建，并写入新的 thread id 覆盖旧值。
