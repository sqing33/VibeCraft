## Why

`vibecraft` 目前只持久化 MCP 配置，并在少数运行时按 turn 注入配置；它没有常驻的 MCP 运行面，因此对话启动时容易找不到可用 MCP，多个 CLI 的接入方式也不一致。现在需要把 MCP 从“设置项”升级为“daemon 托管能力”，让 CLI 始终只连接一个稳定入口，并支持按需唤醒、热更新选择和统一观测。

## What Changes

- 在 daemon 内新增常驻 `MCP Gateway`，对外提供单一 `/mcp` Streamable HTTP 入口。
- 将已配置的 MCP server 从“静态配置注入”升级为“gateway 托管的下游 registry”，支持 stdio/remote 两类下游按需启动、连接复用与 idle TTL 回收。
- 为 gateway 增加设置、token、状态与基础可观测性接口，并在 MCP 设置页提供开关、TTL 与运行状态展示。
- 调整 Codex / Claude / iFlow / OpenCode 的运行时注入方式：CLI 侧只接入 gateway，而不是各自直接持有完整 MCP 列表。
- 让 chat session 的 MCP 选择在 gateway 层动态生效；会话内增删 MCP 后，下一轮调用必须使用更新后的允许集合，并在支持的客户端上触发工具列表刷新。

## Capabilities

### New Capabilities
- `mcp-gateway`: daemon 托管的 MCP 聚合网关，负责统一入口、下游唤醒/回收、工具路由、鉴权与状态查询。

### Modified Capabilities
- `mcp-skill-settings`: MCP 设置从纯注册表扩展为“注册表 + gateway 配置 + 运行状态”，并继续支持 session 级 MCP 选择。
- `cli-runtime`: 所有 CLI runtime 改为连接单一 gateway，并在对话过程中遵循最新的会话 MCP 允许集合。
- `chat-cli-session-resume`: 暖运行时/原生 resume 在 session MCP 选择变化后，仍必须保证下一轮使用最新 MCP 允许集合。

## Impact

- 影响后端配置、settings API、chat runtime、CLI wrapper、daemon server 路由与 MCP 设置 UI。
- 新增 daemon 内部 gateway 模块、token/状态管理与测试桩。
- 需要引入/使用 Go MCP SDK 的 HTTP server 与下游 client transport 能力。
