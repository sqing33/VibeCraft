## Context

vibecraft 的 Codex CLI 对接采用 `codex app-server --listen stdio://` 的 JSON-RPC 协议：daemon 启动子进程并逐行解析 stdout 的 JSON envelope，然后把 `item/*` / `codex/event/*` 通知映射为 `chat.turn.event` 结构化 feed 与兼容的 `chat.turn.delta` 流式增量。

当前实现存在几类稳定性问题：

- stdout 使用 `bufio.Scanner` 逐行读取并强制每行都是 JSON；一旦出现非 JSON 行（日志/颜色控制码/偶发输出）或超长行超过 token 上限，就会直接终止 read loop，导致 turn 无法收到 `turn/completed`。
- `method:"error"` 通知没有解析上游 `will_retry` 语义，导致“可重试告警”被 UI 展示为失败。
- `initialize/thread/start/thread/resume/turn/start` 缺少请求级 timeout 和明确的过载重试策略；遇到 `-32001` 等临时错误时容易误判为不可恢复失败。
- tool stdout/stderr 直接进入 turn item meta，容易把 WS 与 SQLite 压力推高，也让偶发的超大输出更容易触发链路抖动。
- expert 的 `timeout_ms` 已在 Resolve 阶段生成 `Resolved.Timeout`，但 chat turn handler 目前使用 `context.WithoutCancel` 后未附加 timeout，导致 turn 可能“无限跑”。

## Goals / Non-Goals

**Goals:**

- Codex app-server JSON-RPC 读取与解析必须具备容错能力：非 JSON 行不应致命，超长行不应因为 `Scanner` 上限导致连接中断。
- 正确处理 `error` 通知的 `will_retry`：可重试错误展示为进度/系统提示，不把 turn 标记为失败。
- 针对可预期的临时错误（尤其 `-32001 overloaded`）实现指数退避 + jitter 的重试，减少“波动即失败”。
- turn 必须进入终态：无论成功或失败，前端必须能收到 `chat.turn.completed`（以及可恢复的 timeline 快照），避免 UI 卡住。
- tool 输出在 WS 与持久化侧做尾部截断，完整原始输出写入 artifacts，兼顾稳定性与可排障性。
- chat turn 必须遵守 expert `timeout_ms`（忽略 HTTP 取消，但不忽略超时）。

**Non-Goals:**

- 不重写为全新的 JSON-RPC 框架或引入复杂外部依赖。
- 不对前端协议与渲染结构做大改（以保持变更范围可控）。
- 不在本次变更里引入新的持久化表结构迁移（除非实现过程中被证明是必要的）。

## Decisions

1. **stdout 读取：从 `bufio.Scanner` 切换为无 token 上限的逐行读取，并对非 JSON 行容错**
   - 使用 `bufio.Reader.ReadBytes('\n')`（或等价实现）读取一行，去掉尾部换行后尝试 JSON 解析。
   - 解析失败时不设置 `readErr`，而是把原始行写入 diagnostics artifacts（并做采样/限额），同时继续读取后续行。
   - 仅在底层 I/O 错误或进程退出时结束 read loop，并让 Wait/Close 负责收敛错误原因。

2. **JSON-RPC 调用：引入“可分类”的错误与统一的 call 重试逻辑**
   - 解析 response error 时保留 `code/message`（必要时再保留 `data`），并将 `-32001` 识别为可重试错误。
   - `initialize/thread/*/turn/start` 使用 `callWithRetry`：每次调用附加 `context.WithTimeout`，在遇到可重试错误时按指数退避 + jitter 重试，最大次数与最大等待受限（避免无限等待）。

3. **`method:"error"` 通知：按协议语义降级展示**
   - 解析上游 `ErrorNotification`（至少提取 `message` 与 `will_retry`）。
   - `will_retry=true`：
     - 发出 `kind=system|progress` 的 upsert，内容为“Codex 过载/正在重试”等。
     - 不把 turn 标记为 failed，不触发 fallback。
   - `will_retry=false`：
     - 发出 `kind=error` 的 upsert，并把错误摘要缓存到本轮状态，用于最终收敛时的 fallback assistant 文案。

4. **终态收敛策略：优先“可读完成”，必要时 fallback**
   - 如果 `turn/completed` 正常到达：按现有逻辑完成 assistant message 与 timeline。
   - 如果流异常关闭但已累计 assistant 文本：将累计文本作为 final，并附加一段短的“流异常中断”提示（不泄露内部堆栈），然后仍然完成 turn 与 `chat.turn.completed`。
   - 如果没有任何可用文本且属于“早期失败”（初始化/线程/启动阶段失败）：保持现有一次 fallback 到 legacy CLI wrapper 的策略。

5. **tool 输出策略：截断流式 + 全量 artifacts**
   - turn item 的 `meta.stdout/meta.stderr` 仅保留尾部（例如各 32KB），并在 meta 中增加 `stdout_truncated/stderr_truncated` 与总字节数等信息（便于 UI 展示“已截断”）。
   - 同时把完整 stdout/stderr 写入 artifacts（按 turn + tool call 维度，或至少按 turn 维度追加日志），用于排障与复现。

6. **chat turn timeout：在 handler 的 WithoutCancel 上附加 WithTimeout**
   - turnCtx = `context.WithoutCancel(reqCtx)`，若 `resolved.Timeout > 0`，再包一层 `context.WithTimeout(turnCtx, resolved.Timeout)`。
   - 超时触发后，turn 进入失败或“完成但带错误摘要”的终态收敛路径，确保 UI 可恢复。

## Risks / Trade-offs

- [截断 tool 输出降低可见性] → 通过 artifacts 保留全量输出，并在 meta 标注已截断与总量。
- [吞掉非 JSON 行可能掩盖真实协议错误] → 将非 JSON 行写入 diagnostics artifacts，并设置限额与采样；当出现高频非 JSON 行时仍可升级为失败（避免无限噪声）。
- [重试导致整体 turn 更慢] → 限制重试次数/总等待时间；对 `-32001` 之外的错误不盲目重试。
- [把异常断流收敛为 completed 可能“看起来成功”] → assistant 文案明确标注“结果可能不完整”，并保留错误摘要/诊断工件供定位。

