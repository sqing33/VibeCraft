## Why

当前系统把模型配置、expert 配置和 CLI 执行器混在一起：`llm.models` 会被自动镜像成主执行 expert，而 `Codex` / `ClaudeCode` 既像工具名又像 expert 名。这和现有参考项目的主流做法不一致，也会让用户在配置时难以理解“到底是在选工具，还是在选模型”。

现在需要把主抽象收敛成“先选 CLI 工具，再在该工具下选模型”：`Codex CLI` 绑定 OpenAI-compatible，`Claude Code` 绑定 Anthropic-compatible；模型不再直接提升成主执行 expert。

## What Changes

- 新增 `CLI Tools` 设置能力，显式配置 `Codex CLI` 与 `Claude Code` 两个主执行器，并绑定协议族与默认模型。
- **BREAKING**: 停止将 `llm.models` 自动暴露为主执行 expert；模型页改为维护模型池，工具页负责选择哪个 CLI 工具及其默认模型。
- 修改 chat 主流程：创建会话与发送 turn 时支持 `cli_tool_id + model_id`，用户先选工具、再选模型。
- 修改 expert 抽象：expert 回退为 persona / policy 层，不再承担“模型列表”的职责；CLI tool 与模型解析从 expert resolve 中显式分离。
- 修改 workflow / orchestration 的默认路由：主 AI 路径按 CLI tool 执行，工具默认模型来自 CLI tool 配置。
- UI 新增 `CLI 工具` Tab，并重构 Chat composer 的选择方式。

## Capabilities

### New Capabilities
- `cli-tool-settings`: 配置和管理 `Codex CLI` / `Claude Code` 两个主执行器、协议族绑定和默认模型。

### Modified Capabilities
- `experts`: expert 变成 persona / policy 配置，主执行模型不再来自 `llm-model` expert 镜像。
- `llm-settings`: 模型页改为维护模型池，模型不再自动变成主执行 expert。
- `chat-session-memory`: chat 会话/turn 需要记录 `cli_tool_id + model_id`，并支持工具优先的选择流程。
- `workflow`: workflow planning / worker 继续走 CLI runtime，但默认模型来源改为 CLI tool 配置。
- `project-orchestration`: orchestration agent/synthesis 默认模型来源改为 CLI tool 配置，并支持工具优先的元数据。
- `ui`: 设置页与 Chat 页改成“先选 CLI 工具，再选模型”的交互。

## Impact

- 后端：`backend/internal/config/*`、`backend/internal/expert/*`、`backend/internal/api/chat.go`、`backend/internal/api/settings_*`、`backend/internal/chat/*`、`backend/internal/api/workflow_start.go`、`backend/internal/orchestration/*`。
- 前端：`ui/src/app/components/SettingsDialog.tsx`、新增 `CLIToolSettingsTab`、`LLMSettingsTab.tsx`、`ChatSessionsPage.tsx`、`ui/src/lib/daemon.ts`。
- 数据/契约：chat API 和会话元数据增加 `cli_tool_id/model_id`；新增 CLI tool settings API；`/api/v1/experts` 需要明确 helper-only 与主执行 expert 的区别。
