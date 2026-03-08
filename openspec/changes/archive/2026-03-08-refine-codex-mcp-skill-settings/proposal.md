## Why

当前这版 MCP / Skill 设置把 Codex 的真实接入模型做偏了：MCP 被拆成“名称/ID/启用开关”的表单，而 Skill 被建成“全局启用 + 按工具启用”的双层开关。它们都偏离了 Codex CLI 的实际使用方式，导致前端概念冗余、运行时约束错误，也会误导后续功能继续沿着错误模型扩展。

## What Changes

- 将 MCP 设置改为 JSON 原生配置模型：用户直接录入或编辑 MCP JSON，兼容 `{"mcpServers": {...}}` 与 `{...}` 两种形态。
- MCP 设置页移除“按工具启用”这一层，只保留“默认启用”用于决定新建会话时每个 CLI tool 的默认选中集合。
- MCP 注册项不再要求单独填写显示名/ID/启用开关，服务端从 JSON key 解析稳定 `id`，并保留原始 JSON 供后续编辑。
- Skill 设置页改为“发现与来源状态”视图：展示从项目目录与用户目录发现到的 skills，不再提供逐 skill 启用/停用或按工具绑定开关。
- Codex 运行时的 skill 注入改为“默认使用全部已发现 skills”，仅由 expert `enabled_skills` 做运行时收窄，不再受 tool-level skill 绑定影响。
- 保留聊天会话级 MCP 选择与 `config.mcp_servers` 注入机制，但其候选集合改为所有已保存 MCP 注册项。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `mcp-skill-settings`: MCP 设置改为 JSON 原生导入/编辑；Skill 设置改为纯发现视图；移除冗余启用模型。
- `cli-runtime`: Codex 线程的 MCP 与 Skill 注入规则需要对齐新的配置/发现模型。
- `experts`: `enabled_skills` 的运行时约束语义需要从“与 tool 绑定集求交”改为“与已发现 skill 集求交”。

## Impact

- 后端：`backend/internal/config`、`backend/internal/api`、`backend/internal/chat`
- 前端：`ui/src/app/components/MCPSettingsTab.tsx`、`ui/src/app/components/SkillSettingsTab.tsx`、`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/lib/daemon.ts`
- 测试：MCP/Skill 设置 API、运行时注入、配置归一化测试
- 文档与规范：`PROJECT_STRUCTURE.md`、`openspec/specs/*`
