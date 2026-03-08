## Context

`vibe-tree` 当前已经使用 `codex app-server` 承载 Codex 聊天，但 MCP 与 Skill 仍主要依赖用户本机已有的 Codex 配置与技能目录自动发现。前端缺少集中设置入口，聊天会话也无法按当前任务精确选择要暴露给 Codex 的 MCP 集合。

本次变更同时涉及配置模型、设置 API、聊天会话数据模型、Codex app-server 请求构造、以及前端设置/聊天交互，属于跨模块运行时变更。约束上需要避免修改用户全局 `~/.codex/config.toml`，并保持现有 CLI tool / model 选择链路兼容。

## Goals / Non-Goals

**Goals:**
- 在前端提供独立的 MCP 与 Skill 设置页。
- 支持为每个 MCP / Skill 维护按 CLI tool 的默认启用关系。
- 新建聊天会话时按默认配置带出 MCP 选择，并允许仅对当前会话调整。
- 让 Codex 线程启动/恢复时仅看到当前会话选中的 MCP。
- 让 Skill 在运行时通过 allowlist/path index 生效，并支持 expert `enabled_skills` 做进一步限制。

**Non-Goals:**
- 不修改用户全局 `~/.codex/config.toml`。
- 不为 Skill 增加“每条消息动态切换”的 UI。
- 不在 v1 中把 Skill 内容全文注入 prompt。
- 不在 v1 中为 Repo Library 分析入口补充单独的 MCP 选择 UI。

## Decisions

### 1. MCP 采用“配置持久化 + 线程级注入”而不是写全局 Codex 配置
- 持久层把 MCP 注册表保存在 `vibe-tree` 自己的配置文件中。
- 运行时在 `thread/start` / `thread/resume` 的 `config` 字段中注入当前会话选中的 `mcp_servers`。
- 这样可以按会话隔离 MCP，避免全局配置污染，并与 Codex app-server 的原生覆盖层兼容。
- 备选方案是直接写 `~/.codex/config.toml` 或为每个会话创建临时 `CODEX_HOME/config.toml`；前者会污染用户环境，后者要管理额外目录生命周期，首版复杂度更高。

### 2. Skill 采用“默认按 tool 启用 + baseInstructions allowlist/path index 注入”
- Skill 发现仍沿用本项目已有的扫描逻辑。
- 设置页保存的是 skill 绑定关系，而不是复制/管理 skill 文件内容。
- 运行时将当前 tool 启用、并且未被 expert `enabled_skills` 排除的 skills，拼成一个稳定格式的说明块追加到 `baseInstructions`。
- 备选方案是为每个会话创建 shadow skill roots，只暴露被选中的 skill；这会引入目录镜像、路径解析与 resume 生命周期复杂度，首版不采用。

### 3. MCP 会话选择保存到 chat session
- `chat_sessions` 新增 MCP 选择字段，用于保证后续 turn resume 时仍能得到相同的 MCP 集合。
- 新建会话时，MCP 选择由“每个 MCP 在当前 CLI tool 下的默认启用状态”初始化。
- 发送 turn 时不再要求每次重复选择 MCP；只在修改会话运行设置时更新 session 持久字段。

### 4. Skill 默认全部启用，但仍保留 per-tool 绑定
- 新增 skill 时默认绑定到当前所有 CLI tools。
- 这样满足“添加后默认全部启用”的产品预期，同时保留将来按 tool 缩小范围的能力。

## Risks / Trade-offs

- [风险] MCP 会话选择新增后，旧会话没有该字段。 → 通过 store migration 增加可空字段，读取时回退到按 tool 默认启用集合。
- [风险] Skill 说明块过长，影响上下文。 → 只注入 `id`、简短描述、路径和使用规则，不注入 `SKILL.md` 正文。
- [风险] Skill 目录扫描结果与实际 Codex 可读取路径不一致。 → 设置页展示 path；运行时仅注入存在且可读的 skill，并对失效 path 给出告警。
- [风险] MCP 选择变更后与已存 session 默认值不一致。 → 区分“默认配置”和“当前会话覆盖”，前者不自动回写后者。

## Migration Plan

1. 新增配置字段与 settings API，保持默认值向后兼容。
2. 为 `chat_sessions` 增加 MCP 选择字段，并在旧数据上允许为空。
3. 前端增加 MCP/Skill tab 与新建会话 MCP 选择器。
4. Codex runtime 启用 `config.mcp_servers` 与 skill allowlist 注入。
5. 补充配置/API/store/runtime 的单元测试与前端类型同步。

## Open Questions

- 无。首版实现采用：MCP 会话级选择；Skill 默认全部启用但支持后续按 tool 收缩。
