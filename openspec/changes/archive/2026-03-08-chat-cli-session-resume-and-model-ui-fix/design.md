## Context

当前 `vibecraft` 已支持工具优先的 `Codex CLI / Claude Code` 选择，但 Chat 仍未充分利用 CLI 自身的会话能力：

- `chat.Manager.runCLITurn()` 仍会把 `summary + recent history + current input` 重建成 prompt，再每次启动一次新的 CLI oneshot。
- wrapper 目前只写 `final_message.md/summary.json/artifacts.json`，没有稳定产出 CLI session/thread 引用。
- Chat UI 的模型选择器在当前工具优先改造后仍有受控状态与选项列表不同步的问题，导致下拉中勾选了某项，但框里显示空白。

参考工件显示：
- Codex 路径会输出 `thread.started.thread_id` 并支持 `codex exec resume <session_id>`。
- Claude Code 路径会输出 `session_id` 并支持 `-r/--resume <session_id>`。
- 两者在恢复后续对话时，应用层只发送“当前输入”，不再重放完整历史。

## Goals / Non-Goals

**Goals:**
- 在本地持久化 CLI session/thread 引用，并让后续 turn 优先使用 CLI resume。
- 保留本地 summary/历史重建作为 fallback，而不是作为每次 turn 的主路径。
- 修复 Chat 页面模型选择器的空白显示问题。
- 不破坏现有 chat WS 事件、附件、manual compact、thinking translation 兼容性。

**Non-Goals:**
- 不在本次引入真正的长连接 app-server lane。
- 不重写 workflow/orchestration 的会话恢复方式。
- 不把 `Expert Builder` 一起重构到 session resume。

## Decisions

### 1. `chat_sessions` 新增 CLI session 引用字段

增加至少：
- `cli_tool_id`
- `model_id`
- `cli_session_id`

其中：
- Codex 的 `thread_id` 存入 `cli_session_id`
- Claude Code 的 `session_id` 存入 `cli_session_id`

### 2. wrapper 统一产出 `session.json`

两个 wrapper 都需要在 artifact 目录写出：
- `session.json`

结构最小包含：
- `tool_id`
- `session_id`
- `model`
- `resumed`

### 3. Chat turn 优先 resume

当 session 已有 `cli_session_id` 时：
- Codex wrapper 调 `codex exec resume <session_id>`
- Claude wrapper 调 `claude -p -r <session_id>`

应用层只传当前 turn 的输入与新附件路径，不再主动拼完整历史。

当 resume 失败时：
- fallback 到当前的本地重建 prompt 方案
- 并允许 wrapper 返回新的 session id 覆盖旧值

### 4. UI 模型选择器显式渲染 label

为两个模型 `Select` 增加显式 render/label 映射，确保：
- 即使 session 存的是实际模型名，仍能映射回当前 model pool 的条目
- 即使工具切换后模型池刷新，当前默认模型也不会显示空白

## Risks / Trade-offs

- [CLI session 失效] → 回退到本地重建 prompt，并在下一次成功运行后刷新 `cli_session_id`
- [Codex / Claude 输出格式变化] → wrapper 解析 `session_id` 失败时仍可落 `final_message.md`，不会阻断基本回答
- [旧会话无 `cli_session_id`] → 按首次 turn 路径执行，兼容升级
- [附件与 resume 组合复杂] → 继续只发送本轮新增附件；历史附件由 CLI session 自己维护，上层不再重复重放

## Migration Plan

1. 增加 chat session schema 字段并兼容旧数据。
2. 扩展 wrapper 产出 `session.json` 并支持 resume。
3. 更新 chat manager：优先 resume，失败回退到重建 prompt。
4. 修复 Chat 页面模型选择器显示问题。
5. 跑 backend test + ui build 验证。
