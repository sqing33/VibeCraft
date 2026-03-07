## Context

当前 CLI chat 的核心问题不在“能不能回答”，而在“事件没有穿透”：
- `runCLITurn()` 读取子进程输出后，仍然会 `io.ReadAll(output)` 等待结束，再一次性广播 `chat.turn.delta`。
- wrapper 主要产出 `final_message.md`、`summary.json`、`session.json`，中间 JSON/JSONL 事件没有实时转发给上层。
- 前端已经支持 `chat.turn.delta`、`chat.turn.thinking.delta`、`chat.turn.thinking.translation.delta`、`chat.turn.completed` 等事件，但 CLI 路径没有充分利用这套能力。

参考工件显示：
- Claude Code 的 `stream-json` 提供 assistant / user / tool / result / session_id 等结构化事件。
- Codex 的 JSON 事件至少能提供 session/thread 建立事件与 assistant message 相关事件；是否能稳定给出 reasoning 文本要看实际事件结构，但 plan/tool/progress 类事件通常可映射成中间状态。

## Goals / Non-Goals

**Goals:**
- 让 CLI chat 真正变成增量流式，而不是完成后整块回写。
- 在 Claude 路径上优先接出可用的 thinking/reasoning 流。
- 在 Codex 路径上至少接出 assistant 文本增量和可用的 plan/tool 中间事件。
- 保留最终 artifact 契约和 `session.json` 续聊能力。

**Non-Goals:**
- 不要求本次引入 Codex app-server 全量交互会话。
- 不要求所有 CLI 都必须支持 reasoning 文本；对不支持的 CLI，可只展示可用的中间事件。
- 不改 workflow/orchestration 的日志/事件模型。

## Decisions

### 1. `runCLITurn()` 改为边读边广播

当前 `runCLITurn()` 在 goroutine 里 `ReadAll(output)`，这需要改为扫描式读取：
- 对纯文本输出，按 chunk 增量广播 `chat.turn.delta`
- 对 JSON/JSONL 输出，解析事件类型后映射到对应的 `chat.turn.*`

### 2. wrapper 负责把“工具特定事件”转成统一事件流

wrapper 层输出仍然写 `final_message.md/summary.json/session.json`，但同时 stdout 应该稳定输出统一 JSONL 事件，例如：
- `type=assistant_delta`
- `type=thinking_delta`
- `type=tool_status`
- `type=session`
- `type=final`

这样 Go 后端不用深度耦合每个 CLI 的原始协议。

### 3. Claude 优先展示 thinking，Codex 优先展示可用进展

- Claude Code：优先从 stream-json 中提取 assistant 与 thinking/reasoning 相关事件。
- Codex：优先提取 assistant 文本增量；若 reasoning 不稳定，则用 plan/tool/progress 事件填充“思考/进度区”。

### 4. 保持最终完成态双保险

即使流式事件失败或丢失，仍以 `final_message.md` / `summary.json` 作为最终落库来源，确保聊天结果不因流式链路不稳而丢失。

## Risks / Trade-offs

- 不同 CLI 的事件协议差异大 → wrapper 先统一事件，再交给 manager
- Codex reasoning 可能无法稳定抽出 → 允许显示“计划/工具进度”替代真正思考文本
- 前端 UI 可能被高频事件刷爆 → 需要对 thinking/tool 事件做节流或 chunk 合并
- 继续保留 `final_message.md` 双保险会增加少量重复逻辑 → 这是值得的稳定性开销
