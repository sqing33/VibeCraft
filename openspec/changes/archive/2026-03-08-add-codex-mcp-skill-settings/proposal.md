## Why

当前 `vibe-tree` 已经能用 Codex app-server 运行聊天，但 MCP 与 Skill 仍停留在“本机已有配置/目录自动生效”的隐式模式。这样既无法在前端集中配置，也无法按会话精确控制 MCP 暴露范围，容易让无关 MCP 增加对话上下文与工具噪声。

## What Changes

- 新增独立的 `MCP` 设置页，支持维护 MCP 注册表、按 CLI tool 设置默认启用状态。
- 新增独立的 `Skill` 设置页，支持发现/查看 skills，并按 CLI tool 维护启用绑定；新增 skill 默认对所有 CLI tools 启用。
- 为聊天会话新增“当前会话启用哪些 MCP”的选择，并以默认配置初始化，不回写全局默认值。
- 为 Codex app-server 线程启动/恢复新增 `config.mcp_servers` 注入，只把当前会话选中的 MCP 暴露给 Codex。
- 为 Codex 基础指令新增 skill allowlist/path index 注入，让 Codex 只按需读取当前 tool/expert 有效的 `SKILL.md`。
- 将 expert `enabled_skills` 从纯元数据升级为运行时限制条件，与 tool 级 skill 启用集取交集。

## Capabilities

### New Capabilities
- `mcp-skill-settings`: 管理 MCP/Skill 设置、默认启用状态、聊天会话的 MCP 选择，以及对 Codex 运行时的注入策略。

### Modified Capabilities
- `cli-runtime`: Codex chat runtime 需要支持按线程注入 MCP 配置与 Skill 指令。
- `experts`: `enabled_skills` 从仅展示/生成约束升级为运行时生效的 skill 限制。

## Impact

- 后端：`backend/internal/config`、`backend/internal/api`、`backend/internal/chat`、`backend/internal/store`
- 前端：`ui/src/app/components/SettingsDialog.tsx`、新增 MCP/Skill 设置 tab、`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/lib/daemon.ts`、`ui/src/stores/chatStore.ts`
- 数据：`chat_sessions` 需要保存当前会话的 MCP 选择
- 运行时：Codex `thread/start` / `thread/resume` 请求体新增 `config.mcp_servers`，`baseInstructions` 增加 skill index/allowlist
