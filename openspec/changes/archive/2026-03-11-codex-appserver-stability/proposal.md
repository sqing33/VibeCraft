## Why

当前 Codex CLI（`codex app-server --listen stdio://`）接入链路在高频交互时容易出现波动：stdout 混入非 JSON 行或超长 JSON 行会触发解析失败，导致 turn 流提前断开；前端只看到一条 `Codex CLI error` 的 runtime feed 条目，但整轮没有 `chat.turn.completed` 收敛，用户体验表现为“卡死/报错但无结果”。

此外，Codex `method:"error"` 通知包含 `will_retry` 语义，当前实现把其一律当作致命失败，进一步放大了误报与不稳定感。

## What Changes

- 加固 Codex app-server stdout 读取与 JSON-RPC 解析：移除 `bufio.Scanner` token 上限约束，对非 JSON 行做容错并落盘诊断工件，而不是直接终止连接。
- 为 `initialize/thread/start/thread/resume/turn/start` 增加请求级超时与可控重试：
  - 特别处理 `-32001 Server overloaded; retry later` 等可重试错误，采用指数退避 + jitter。
- 正确解释 `method:"error"` 通知：
  - `will_retry=true` 作为进度/系统提示，不把 turn 标记为失败。
  - `will_retry=false` 生成可读错误摘要，并保证 turn 能进入终态。
- 强化 turn 终态收敛：即使 app-server 流异常断开，也要尽最大努力产出可读 assistant 结果（或错误摘要）并广播 `chat.turn.completed`，避免 UI 卡在 running 状态。
- 控制 tool 输出体积：WebSocket/持久化的 `chat.turn.event` 仅保留 stdout/stderr 尾部片段（例如 32KB），完整原始输出写入 artifacts 以便排障。
- 将 expert `timeout_ms` 真正应用到 chat turn 的执行上下文（当前 handler 使用 `context.WithoutCancel` 导致 timeout 未生效）。

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `cli-chat-streaming`: Codex app-server 连接与流式事件解析的稳定性与终态收敛语义增强。
- `cli-chat-thinking`: Codex `error` 通知（含 `will_retry`）与 tool 输出展示的归一化规则调整。
- `experts`: chat turn 执行必须遵循 expert 的 `timeout_ms`。

## Impact

- 后端：`backend/internal/chat/codex_appserver.go`、`backend/internal/chat/codex_turn_feed.go`、`backend/internal/api/chat.go` 以及相关测试。
- 前端：无需改协议结构，但会受益于更稳定的 completed 收敛与更少的误报 error 条目。
- 运行时与存储：turn timeline 的 tool meta 体积受控，减少 WS/SQLite 压力；新增 app-server 诊断 artifacts 便于定位偶发问题。

