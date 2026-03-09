## Why

`vibe-tree` 目前已经支持 `Codex CLI`、`Claude Code` 与 `iFlow CLI` 三条主执行路径，但还缺少一个可以同时承接 OpenAI / Anthropic 模型池、并通过官方 `run` / `serve` 能力执行的通用 CLI runtime。`OpenCode CLI` 已经具备会话续跑、JSON 事件流、agent / permission / server 能力，适合作为新的并列 CLI tool 接入。

当前实现还存在一个结构性限制：CLI tool 只能声明单一 `protocol_family`。这对 `codex` / `claude` / `iflow` 足够，但会直接限制 `opencode` 这类多 provider CLI 的模型选择与 UI 过滤能力，因此需要把 CLI tool 配置扩展成“单协议兼容、多协议优先”。

## What Changes

- 新增 `OpenCode CLI` 作为并列可选的主执行工具，保持与 `codex` / `claude` / `iflow` 共存，不替换默认 `codex` 路径。
- 扩展 `cli_tools` 配置与 API，允许 CLI tool 声明多个兼容协议族，并保持对旧 `protocol_family` 字段的向后兼容。
- 新增 `opencode` wrapper，使用官方 `opencode run` 非交互参数与 `--format json` 流式事件输出，并落标准 artifact（`summary.json` / `artifacts.json` / `session.json` / `final_message.md` / `patch.diff`）。
- 为聊天会话补齐 OpenCode 的 native session resume：保存 `session id`，后续 turn 优先 `--session` 续跑，失败后回退到本地 reconstructed prompt。
- 扩展 CLI runtime 的模型注入能力，使 `opencode` 可以基于所选 `model_id` 同时消费 OpenAI / Anthropic source 的连接配置。
- 更新 Settings / Chat / Repo Library 的 CLI tool 文案与模型过滤逻辑，使多协议工具按兼容 provider 列表展示模型；`reasoning_effort` 继续严格限制为 `Codex CLI` 专属。
- 更新 `PROJECT_STRUCTURE.md` 与 OpenSpec delta specs，记录新 wrapper 与配置能力。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `cli-runtime`: 让多协议 CLI tool 继承所选模型 source 的连接配置，并为 OpenCode 增加标准 wrapper artifact contract。
- `cli-tool-settings`: CLI tool 配置从单协议扩展为兼容多协议，并新增 `opencode` 作为第四个主执行 tool。
- `chat-cli-session-resume`: CLI 原生 session/thread 恢复逻辑扩展到 OpenCode session id。
- `cli-chat-streaming`: CLI 聊天流式输出能力扩展到 OpenCode JSON 事件流。
- `ui`: Settings / Chat / Repo Library 的 CLI tool 与模型过滤逻辑支持 `OpenCode CLI` 和多协议工具。

## Impact

- 后端：`backend/internal/config/*`、`backend/internal/expert/*`、`backend/internal/chat/*`、`backend/internal/api/settings_clitools.go`、`backend/internal/api/chat.go`。
- CLI wrapper：`scripts/agent-runtimes/opencode_exec.sh`。
- 前端：`ui/src/lib/daemon.ts`、`ui/src/app/components/CLIToolSettingsTab.tsx`、`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/app/pages/RepoLibraryRepositoriesPage.tsx`。
- 文档与规范：`PROJECT_STRUCTURE.md`、`openspec/changes/integrate-opencode-cli/specs/**`。
