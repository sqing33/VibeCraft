## Context

上一轮实现已经把 MCP 会话级选择和 Codex `config.mcp_servers` 注入打通，但 UI 和配置模型仍保留了过多“人为设计”的控制面：MCP 被建模为可手填 label/id、可按工具启用/默认启用的对象；Skill 被建模为可全局启用且再按工具启用的绑定。结合参考项目（尤其是 `.github-feature-analyzer/iOfficeAI-AionUi` 中的 JSON 导入式 MCP 配置与“skills 默认发现、由 enabledSkills 收窄”的运行时方式），可以确认这两个模型都不符合目标产品心智。

本次调整会触及设置 API、配置归一化、Codex runtime 注入逻辑和前端设置页，但不会改变已存在的“会话级 MCP 选择”主链。

## Goals / Non-Goals

**Goals:**
- 让 MCP 设置页直接面向 Codex MCP JSON，而不是额外的表单概念。
- 保留每个 CLI tool 的 MCP 默认选中能力，但移除冗余的“按工具启用”层。
- 让 Skill 设置页真实反映“项目/用户目录中的已发现技能”，不再伪造额外开关。
- 让 Codex runtime 默认注入全部已发现 skills，并只由 expert `enabled_skills` 做约束。
- 对旧配置保持向后兼容，避免因历史字段导致加载失败。

**Non-Goals:**
- 不改动全局 `~/.codex/config.toml`。
- 不为 Skill 新增安装器、目录管理器或 per-conversation 选择 UI。
- 不改变现有 chat session 对 MCP 选择的持久化方式。
- 不把 `SKILL.md` 全文注入 Codex 指令。

## Decisions

### 1. MCP 以“原始 JSON + 解析后条目”双表示保存
- 设置 API 接收和返回每个 MCP 条目的 `raw_json`，同时返回解析得到的 `id`、`config` 和 `default_enabled_cli_tool_ids`。
- 服务端兼容两种输入：`{"mcpServers": {"server": {...}}}` 与 `{"server": {...}}`；任一输入都归一为按 server key 拆分的条目列表。
- 这样前端编辑时不需要额外输入 ID/显示名，且能保留与用户原始配置最接近的内容。
- 备选方案是仅存解析后的 map，不保留原始 JSON；缺点是编辑回显时会丢失用户写法与注释/结构习惯。

### 2. MCP 默认启用是唯一的 tool-level 开关
- `default_enabled_cli_tool_ids` 保留，用于“新建会话时默认勾选哪些 MCP”。
- 移除 `enabled_cli_tool_ids` 和 `enabled`：一个 MCP 是否被实际注入，由“是否被当前会话选中”决定；是否会在新会话里自动出现，由 `default_enabled_cli_tool_ids` 决定。
- 备选方案是保留 `enabled` 作为全局总开关；但这会与“是否默认选中”“是否当前会话选中”形成三套状态，继续增加理解成本。

### 3. Skill 改为发现驱动，不再持久化绑定
- Skill 来源仍然是项目目录与用户目录中的 `SKILL.md`。
- 设置 API 只返回 discovered skills 列表及其来源/路径，前端作为状态页展示，不再提供保存接口。
- Codex runtime 直接使用 `skillcatalog.Discover()` 的结果构建 allowlist，并在 expert 配置声明 `enabled_skills` 时做交集过滤。
- 备选方案是继续保留 `skill_bindings` 作为高级配置；但当前产品没有真实需求，且会持续制造“默认启用/按工具启用”的假概念。

### 4. 旧配置字段保持可读但不再生效
- 历史 `skill_bindings`、`mcp_servers[].enabled`、`mcp_servers[].enabled_cli_tool_ids` 在加载时继续兼容 JSON 解码，但不再参与新逻辑决策。
- `NormalizeMCPServers` 会把旧字段折叠为新模型，保证旧配置能平滑迁移；Skill 相关旧字段直接忽略。
- 这样可以避免用户现有 `config.json` 因结构调整而失效。

## Risks / Trade-offs

- [风险] 用户输入的 MCP JSON 非法或一次包含多个 server。 → 服务端做结构校验，并在 UI 明确支持单条/多条 JSON 导入。
- [风险] 移除 Skill 保存接口后，旧前端状态或测试可能仍尝试 PUT。 → 同步移除前端保存按钮与后端 PUT 路由/测试。
- [风险] 旧配置中遗留的 skill 绑定仍让维护者误以为生效。 → 在代码中显式忽略该字段，并更新 OpenSpec 与页面文案。
- [风险] MCP 默认启用减少后，用户担心某个 server 被“禁用”。 → 会话页保留显式勾选列表，任何已保存 server 都可被当前会话手动启用。

## Migration Plan

1. 新增新的 MCP API/前端数据形态，并兼容读取旧 `mcp_servers` 字段。
2. 将 Skill 设置 API 改为发现只读视图，同时移除前端保存/开关控件。
3. 更新 Codex runtime 的 effective skill 计算逻辑。
4. 更新测试与规格，确认旧配置仍可加载、新 UI 可保存。
5. 完成后归档本次变更，并同步基线 specs。

## Open Questions

- 无。目标形态已经由用户约束与参考实现共同收敛：MCP 走 JSON 配置，Skill 走目录发现。
