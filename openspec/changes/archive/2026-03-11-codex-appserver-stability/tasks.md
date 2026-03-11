## 1. App-Server Stream Resilience

- [x] 1.1 在 Codex app-server 客户端增加 turn 级 diagnostics（原始 stdout/stderr、非 JSON 行采样/限额落盘）
- [x] 1.2 将 `stdioCodexAppServerClient.readLoop` 从 `bufio.Scanner` 改为 `bufio.Reader` 逐行读取，支持超长行并对非 JSON 行容错
- [x] 1.3 启动 app-server 时注入非交互/降噪默认环境变量（不覆盖用户显式配置）

## 2. JSON-RPC Timeout & Retry

- [x] 2.1 为 JSON-RPC 响应错误保留 `code`，并对 `-32001 overloaded` 等可重试错误实现指数退避 + jitter（有上限）
- [x] 2.2 为 `initialize/thread/start/thread/resume/turn/start` 增加请求级 timeout（基于常量或配置），超时后进入可收敛的终态路径

## 3. Structured Feed Hardening

- [x] 3.1 在 `codex_turn_feed.go` 解析 `ErrorNotification`（含 `will_retry`）并将 `will_retry=true` 映射为 progress/system，而不是 error
- [x] 3.2 对 tool 条目的 `stdout/stderr` 做尾部截断并在 meta 标注 truncation 信息；同时把全量输出写入 artifacts

## 4. Terminal Convergence & Chat Timeout

- [x] 4.1 `runCodexAppServerTurn` 在 transport 异常断开时：有部分 assistant 文本则 best-effort 完成并发送 `chat.turn.completed`；无文本则按“早期失败”走一次 legacy fallback
- [x] 4.2 `POST /api/v1/chat/sessions/:id/turns` 将 expert `timeout_ms` 应用到 turn 执行 ctx（`WithoutCancel` 之上叠加 `WithTimeout`）

## 5. Tests & Verification

- [x] 5.1 新增单测覆盖：stdout 非 JSON 行容错、超长 envelope 不致命
- [x] 5.2 新增单测覆盖：`-32001` 重试退避（次数/总等待有上限）
- [x] 5.3 新增单测覆盖：`will_retry` 映射与 tool 输出截断 meta
- [x] 5.4 运行 `go test ./...` 并修复失败用例
