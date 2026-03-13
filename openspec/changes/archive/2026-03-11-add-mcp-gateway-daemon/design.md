## Context

`vibecraft` 当前已具备 MCP 注册表、默认启用集合和 chat session 级 MCP 选择，但运行时形态仍是“把 MCP 配置直接塞给某些 CLI”。这导致几个问题：一是不同 CLI 的注入方式不一致；二是对话启动时经常找不到已经运行的 MCP；三是 session 在对话中修改 MCP 选择后，暖运行时和已建立线程不一定能立即使用最新集合。仓库现有 daemon 已是常驻进程、具备 chat runtime 池化和 settings API，因此最合适的落点是在 daemon 内提供统一的 MCP Gateway。

## Goals / Non-Goals

**Goals:**
- 提供 daemon 常驻的单一 MCP 入口，供所有 CLI runtime 复用。
- 让下游 MCP 支持按需唤醒、连接复用和 idle TTL 回收。
- 让 session 级 MCP 选择在 gateway 层动态生效，并为支持的客户端提供工具列表刷新信号。
- 把 gateway 配置、状态与基础可观测性纳入现有 settings / UI。

**Non-Goals:**
- 不在本次改动中重做现有 MCP JSON 编辑模型。
- 不实现复杂的多租户鉴权体系或跨用户共享的远程部署。
- 不把所有下游 MCP 永久常驻，也不在本次改动中支持 resources/prompts 的完整高级管理界面。

## Decisions

### 1. 在 Go daemon 内实现原生 MCP Gateway
- 选择在 `vibecraft-daemon` 内新增 gateway 模块，而不是额外引入 Node sidecar。
- 原因：现有 settings、chat、workspace 与生命周期都在 Go daemon 内，原生实现更容易复用配置、session 上下文和关闭流程。
- 备选方案：
  - 外置 sidecar：实现快，但会引入额外守护、端口和状态同步复杂度。
  - 继续按 CLI 直接注入多个 MCP：改动小，但无法解决统一生命周期和动态更新问题。

### 2. CLI 统一只连接 gateway，一个 session 对应一份 access token
- Codex / Claude / iFlow / OpenCode 运行时统一只注入一个 gateway MCP 条目。
- daemon 在发起 turn 前生成或刷新 session 级 gateway token，token 绑定 `session_id`、`workspace_path` 与当前允许的 `mcp_server_ids`。
- 原因：这样 CLI 初始化只面对一个 MCP server；session 选择变化时只需更新 gateway access policy，不必把下游全量配置重新塞进每个 CLI。
- 备选方案：
  - 仅按 workspace 共用固定 token：实现简单，但无法表达不同会话不同 MCP 允许集合。
  - 每次 turn 临时写一份多 MCP 配置：仍会把复杂度留给各 CLI wrapper。

### 3. 下游采用“按需启动 + 能力缓存 + idle TTL 回收”
- gateway 按 `(workspace_path, server_id)` 维护运行时槽位。
- 当 `tools/call` 命中冷 server 时按配置启动/连接，并在就绪后转发请求。
- gateway 缓存聚合后的工具目录，用于快速响应 `tools/list`；冷 server 可在后台刷新工具元数据。
- 原因：兼顾启动速度和内存占用，避免“添加五个 MCP 就五个都常驻”。
- 备选方案：
  - 全部常驻：首次调用快，但空闲内存与进程数不可控。
  - 每次 `tools/list` 都唤醒全部下游：工具最准确，但对话启动会重新退化为慢路径。

### 4. 工具命名采用 namespaced 公开名，避免聚合冲突
- gateway 对外暴露工具时使用稳定的 namespaced 名称（如 `<server_id>.<tool_name>`）。
- 内部保存 `publicToolName -> {server_id, original_tool_name}` 路由。
- 原因：不同下游可能存在同名 `search`、`read_file` 等工具，必须在聚合层彻底消除冲突。
- 备选方案：
  - 保留原名并在冲突时报错：用户体验差，且对现有 registry 约束过高。
  - 依赖 `_meta` 指明 server：需要上游 CLI 配合，兼容性较差。

### 5. 对话中修改 MCP 选择后，下一轮强制使用最新 access policy
- session MCP 选择仍保存在 `chat_sessions.mcp_server_ids_json`。
- 每次 turn 开始前都基于最新 session 选择刷新 gateway token / access policy。
- gateway 在支持的 transport 上发送工具列表变更通知；对不主动刷新的客户端，下一轮至少保证 `tools/call` 按最新策略放行或拒绝。
- 对 Codex 暖运行时，如果当前 access policy 与已缓存 thread 上下文不兼容，允许丢弃暖 runtime 后重建线程。
- 原因：这是保证“会话中增删 MCP”可用的最小正确性策略。

## Risks / Trade-offs

- [冷启动首次调用仍可能变慢] → 用固定安装路径替代裸 `npx`，并允许缓存工具目录与预热常用 MCP。
- [不同 CLI 对工具列表热更新支持不一致] → 统一保证“下一轮生效”，支持通知的客户端再做到无感刷新。
- [gateway token / access policy 状态丢失] → token 状态由 daemon 内存维护，daemon 重启后在下一轮重新签发。
- [下游 MCP 工具名变化造成缓存陈旧] → 冷启动后重新刷新该 server 的工具目录，并在状态接口暴露最后刷新时间与错误。
- [聚合层错误更难排查] → 增加 `/api/v1/mcp-gateway/status` 与按 server 的最近错误信息，便于 UI 与日志排障。

## Migration Plan

- 先在配置层引入 `mcp_gateway` 设置，默认 `enabled=false`，避免影响现有用户。
- gateway 启用后，逐步让四类 CLI runtime 改为注入单一 gateway；旧的多 MCP 直注入路径保留为回退逻辑。
- 当 gateway 路径验证稳定后，再将 Codex / iFlow 等默认切到 gateway 注入。
- 回滚时只需关闭 `mcp_gateway.enabled`，runtime 恢复旧的直注入逻辑。

## Open Questions

- 无；本次实现采用：workspace 级下游隔离、session 级 access token、默认 TTL 600 秒、工具名使用 `<server_id>.<tool_name>`。
