## Why

`#/chat` 已经具备每条消息切换运行时/模型、CLI 会话续聊、MCP 注入与流式渲染能力，但目前 Codex CLI 仍缺少“思考程度”选择，且输入区右侧控制栏偏宽、整体高度偏大，影响连续对话效率。

本次在既有 `chat-per-message-model-routing` 变更上继续扩展：一方面把 Codex app-server 已支持的 reasoning effort 打通到 session 默认值与单次 turn 覆盖；另一方面把 Chat 输入区调整为更紧凑的左右布局，让输入框占据主区域，右侧保留更窄的三行控制栏。

## What Changes

- 为 chat session 新增可持久化的 `reasoning_effort` 默认值，并在 Codex turn 成功后跟随 last-used 更新。
- `POST /api/v1/chat/sessions` 与 `POST /api/v1/chat/sessions/:id/turns` 支持 `reasoning_effort`。
- Codex app-server 在线程配置中注入 `model_reasoning_effort`，并在 `turn/start` 时发送本条 turn 的 `effort` 覆盖值。
- Chat 输入区改为左大右小布局：左侧输入框占满剩余高度，右侧缩窄为三行（CLI、模型、思考程度+上传/发送按钮）。
- 思考程度选择器仅在 Codex CLI 运行时可用；切到非 Codex 运行时仍显示但禁用，避免布局跳动。

## Capabilities

### Modified Capabilities

- `chat-turn-model-routing`: 继续维护“每条消息选择运行时/模型”的交互，同时让 session 默认值同步携带 reasoning effort。
- `chat-session-memory`: session 默认值模型扩展为 `expert_id / cli_tool_id / model_id / reasoning_effort / cli_session_id / mcp_server_ids`。
- `chat-cli-session-resume`: Codex 续聊时恢复 thread id 的同时恢复 reasoning effort 默认配置。
- `cli-chat-thinking`: Codex turn 支持 low / medium / high / xhigh 的可选思考程度。
- `ui`: `#/chat` 输入区收窄右侧控制栏，并在发送区加入紧凑的思考程度选择器。

## Impact

- 后端：`backend/internal/api/chat.go`、`backend/internal/store/{chat,migrate}.go`、`backend/internal/chat/{manager,codex_runtime_settings,codex_appserver}.go`
- 前端：`ui/src/lib/daemon.ts`、`ui/src/stores/chatStore.ts`、`ui/src/app/pages/ChatSessionsPage.tsx`
- 数据：SQLite `chat_sessions` 新增 nullable `reasoning_effort` 字段（向前兼容）
- 规范：更新 `ui` delta spec，并新增 `chat-cli-session-resume` / `cli-chat-thinking` delta spec
