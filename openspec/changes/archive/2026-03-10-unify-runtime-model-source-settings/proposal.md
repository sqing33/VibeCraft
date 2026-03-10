## Why

当前应用同时暴露了“模型设置”和“CLI 工具设置”，但 CLI 运行时并不会一致地消费这些设置：`codex`/`claude`/`opencode` 主要依赖各自本地配置或启动参数，`iflow` 又使用单独的官方认证字段，导致“页面能配置、实际不一定生效”。随着项目已经同时支持 4 个 CLI runtime 和 2 个 SDK runtime，这种共享模型池 + 工具特例的结构已经让聊天、设置和运行时解析都变得难以理解和维护。

## What Changes

- 新增独立的 `API Sources` 配置能力，用于统一管理 OpenAI / Anthropic / iFlow 来源，不再把模型配置混在来源卡片里。
- 新增统一的 `Runtime Models` 配置能力，用于分别维护 2 个 SDK runtime 与 4 个 CLI runtime 可用的模型列表、默认模型，以及每个模型绑定的 API 来源。
- 调整 `CLI 工具` 设置职责，只保留 enable / command path / health / iFlow 登录操作等工具级配置，不再承担共享模型池的默认模型配置。
- 重构 CLI 运行时解析链：聊天、仓库分析与其他 CLI 执行面改为从 runtime-model 绑定解析来源与模型，再为每个 CLI 物化受管配置根目录或启动参数，避免直接修改项目内配置或用户全局默认配置。
- 将设置 UI 改为“API 来源 / 模型设置 / CLI 工具”三层职责分离，并更新聊天页的 runtime-first 模型选择逻辑，使运行时仅展示本 runtime 自己声明的模型。
- 增加旧配置到新配置结构的迁移与兼容镜像，保证已有 `llm` / `cli_tools` 数据可以自动导入新的来源与 runtime 模型配置。

## Capabilities

### New Capabilities
- `api-source-settings`: 管理可复用的 API 来源、密钥与 iFlow 官方认证来源配置。
- `runtime-model-settings`: 为 SDK 与 CLI runtime 分别维护模型列表、默认模型以及模型到来源的绑定关系。

### Modified Capabilities
- `cli-tool-settings`: CLI 工具设置改为只负责工具级启停、命令路径、健康检查与 iFlow 登录，不再承载共享模型池默认模型。
- `cli-runtime`: CLI 运行时改为使用应用受管的外部配置根目录或启动参数来消费模型/来源绑定，而不是直接依赖项目配置或用户全局默认配置。
- `ui`: 设置页与聊天页改为围绕 `API 来源 + Runtime 模型 + CLI 工具` 的新结构组织与展示。

## Impact

- 后端配置模型与迁移逻辑：`backend/internal/config/*`
- 设置与聊天 API：`backend/internal/api/*`
- expert / runtime 解析链：`backend/internal/expert/*`, `backend/internal/chat/*`
- CLI wrapper 与受管配置物化：`scripts/agent-runtimes/*`, `backend/internal/cliruntime/*`, `backend/internal/paths/*`
- 前端设置页与聊天页：`ui/src/app/components/*`, `ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/lib/daemon.ts`
- 配置与运行时测试：相关 Go / UI 单测与集成测试
