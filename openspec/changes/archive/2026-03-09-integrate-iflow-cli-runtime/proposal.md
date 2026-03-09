## Why

`vibe-tree` 当前把主执行型 CLI 工具限定在 `Codex CLI` 与 `Claude Code` 两条路径上，缺少一个基于 OpenAI 兼容协议、适合国内模型生态的第三选择。`iFlow CLI` 已经具备非交互执行、模型切换、对话恢复与项目级配置能力，适合作为第三个主执行 CLI runtime 接入。

现有实现还存在一个结构性缺口：CLI runtime 的模型选择虽然能切换 `model_id`，但不会把所选模型 source 的 `base_url/api_key` 注入到 CLI wrapper。这个缺口对 Codex / Claude 影响不大，但会直接限制 `iFlow CLI` 这类 OpenAI-compatible CLI 的可用性。

## What Changes

- 新增 `iFlow CLI` 作为第三个主执行 CLI tool，保持其在 Settings / Chat / Repo Library 中可选，但不抢占 `codex` / `claude` 的默认优先级。
- 新增 `iflow` CLI wrapper，使用官方非交互参数 `--prompt`、`--resume`、`--output-file`、`--yolo` 执行，并落标准 artifact。
- 扩展 CLI runtime 的模型运行时补丁能力：当 CLI tool 依赖所选模型 source 的连接信息时，自动把 `api_key/base_url/model` 注入 wrapper 环境。
- 为仓库新增最小 `.iflow/settings.json` 项目配置，使 iFlow 优先读取 `AGENTS.md` 作为上下文文件。
- 为聊天会话补齐 iFlow 的 native session resume：保存 `session-id`，下个 turn 优先 `--resume`，失败后回退到本地 reconstructed prompt。
- 为 iFlow 增加 best-effort 的纯文本 streaming 兼容路径，并保证最终仍以标准 artifact 作为落盘真相源。
- 更新相关 UI / OpenSpec / 项目结构文档，保证三 CLI tool 共存语义清晰。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `cli-runtime`: 允许 OpenAI-compatible CLI wrapper 绑定所选模型 source 的运行时连接信息，并为 iFlow 增加标准 wrapper contract。
- `cli-tool-settings`: CLI tools 基线从 `codex` / `claude` 扩展为支持 `iflow` 作为第三个主执行 tool。
- `chat-cli-session-resume`: 聊天 session reference 的持久化与恢复逻辑扩展到 iFlow。
- `cli-chat-streaming`: 允许 iFlow 这种纯文本 CLI 通过 line-based 方式提供增量输出，同时保留 artifact contract。
- `ui`: 设置页与聊天 runtime 选择器支持第三个 CLI tool，并正确恢复其选择状态。

## Impact

- 后端：`backend/internal/config/*`、`backend/internal/expert/*`、`backend/internal/api/chat.go`、`backend/internal/chat/*`。
- CLI wrapper：`scripts/agent-runtimes/iflow_exec.sh`。
- 前端：CLI tools / chat runtime 选择相关 UI 与文案。
- 配置：新增 `.iflow/settings.json`。
- 规范：更新 `openspec/` 与 `PROJECT_STRUCTURE.md`。
