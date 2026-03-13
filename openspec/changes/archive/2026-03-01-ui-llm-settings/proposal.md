## Why

目前模型（Codex/ClaudeCode）的 API Key 与节点 URL 主要依赖环境变量（含 dotenv 注入），对多数用户不友好：需要会配置 `.env`/shell 环境，并且难以在 UI 内直观看到当前使用的节点与密钥状态。由于很多用户会使用非官方 API 节点（自建/转发/第三方），需要一个可在前端直接配置与管理的方式。

## What Changes

- UI「系统设置」升级为 Tabs 结构：
  - 保留现有「连接与诊断」内容。
  - 新增「模型」设置页，用于配置 API 源与模型档案。
- 新增“模型设置”数据模型：
  - **API 源（Source）**：仅包含 `base_url` + `api_key`（不绑定模型）。
  - **模型档案（Model Profile）**：包含 `model`、选择要使用的 Source、并选择使用 `codex(openai)` 或 `claudecode(anthropic)` SDK。
- daemon 新增设置读写 API，用于：
  - 读取当前 Sources / Model Profiles（**不回传明文 key**，仅返回 masked/has_key 状态）。
  - 保存变更到 `~/.config/vibecraft/config.json`（确保文件权限与原子写入）。
  - 将 Model Profiles 映射为可执行的 Experts，并在运行时热更新 expert registry（无需重启）。
- 兼容性策略：
  - 继续支持旧的 `${ENV_VAR}` 注入方式；但对常规用户，模型配置不再要求 env。

## Capabilities

### New Capabilities

- `llm-settings`: 在 daemon 侧提供可持久化的 LLM Sources / Model Profiles 配置，并暴露安全的读写 API；运行时将其映射为 Experts 供 workflow/node 选择与执行。

### Modified Capabilities

- `ui`: 系统设置支持 Tabs，并新增「模型」设置页完成 Sources / Model Profiles 的编辑与保存。
- `experts`: expert registry 支持运行时刷新（热更新），并允许由 LLM Model Profiles 派生出可选 expert 列表。

## Impact

- 后端：
  - `backend/internal/config/`：扩展 config schema（LLM settings）并实现安全写回。
  - `backend/internal/expert/`：增加 registry 热更新能力。
  - `backend/internal/api/`：新增 LLM settings API（GET/PUT）。
- 前端：
  - `ui/src/app/components/SettingsDialog.tsx`：Tabs 化并新增模型设置 Tab。
  - `ui/src/lib/daemon.ts`：新增 settings API 调用。
  - 新增若干 UI 组件（表单、列表、Tabs）。
- 测试：新增 Go 单测覆盖 config 写回与 key masking；必要时补充 API handler 单测。
