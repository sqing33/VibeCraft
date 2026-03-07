## Why

`vibe-tree` 当前的主要 AI 执行链路仍以 SDK provider/model 为中心，这让 chat、workflow、orchestration 等需要文件操作、工具调用、长上下文连续性的场景被迫围绕“流式文本调用”设计。现在最需要解决的不是再加更多模型，而是把后端主执行路径切换为 CLI runtime，让 SDK 只保留在 thinking translation、模型测试等超轻量辅助任务里。

## What Changes

- **BREAKING**: 将后端默认 AI 执行路径从 SDK-first 切换为 CLI-first；chat、workflow、project orchestration 的主 AI 运行均默认通过 CLI runtime 执行。
- 新增统一的 CLI runtime contract，约束 CLI wrapper、结构化输出、artifact 目录、session 元数据与共享 execution 生命周期的衔接方式。
- 调整 expert 注册表与解析逻辑，使其显式区分 `cli`、`process` 与 `sdk_helper` 类 expert，并让主 AI surface 默认过滤掉 helper-only SDK expert。
- 将 chat session/turn 语义从 provider-anchor 模式迁移到 CLI-backed session 模式，保留流式事件、附件、fork、manual compact 与 thinking translation。
- 将 workflow 与 orchestration 的规划/执行/综合统一到 CLI runtime，同时复用现有 execution/workspace/orchestration 基座。
- 明确本阶段**不做** Repo Library / `github-feature-analyzer` 集成，也不要求一并完成交互式 `app-server` 会话。

## Capabilities

### New Capabilities
- `cli-runtime`: 定义 CLI 作为默认 AI runtime 时的执行合同、artifact/session 结构化输出，以及 SDK helper 的保留边界。

### Modified Capabilities
- `experts`: 专家配置、解析与列表接口改为 runtime-aware，支持 CLI-first 与 helper-only SDK。
- `workflow`: workflow 的 AI 规划与 AI worker 执行默认走 CLI runtime。
- `project-orchestration`: orchestration 的 master/agent/synthesis 默认走 CLI runtime，并持久化 runtime 元数据。
- `agent-workspace-flow`: agent run 产出的 worktree/workspace 元数据扩展为 CLI artifact/session contract。
- `chat-session-memory`: chat session/turn 从 SDK/provider-anchor 模式迁移到 CLI-backed 模式，同时保留流式与 compaction 体验。

## Impact

- 后端模块：`backend/internal/config/*`、`backend/internal/expert/*`、`backend/internal/runner/*`、`backend/internal/chat/*`、`backend/internal/api/chat.go`、`backend/internal/api/workflow_start.go`、`backend/internal/orchestration/*`、`backend/internal/scheduler/*`、`backend/internal/store/*`。
- 配置与数据：expert config 需要新增 runtime 元数据；chat/orchestration/workflow 相关持久化需要保存 CLI session/artifact/runtime 相关字段；旧 provider anchor 数据保留兼容读但不再参与新 turn 路由。
- API / WS：chat turn、workflow execution、orchestration detail 需要增加 runtime/artifact 相关字段，但尽量保持现有 endpoint 结构与执行日志复用方式不变。
- 保留的 SDK 辅助能力：thinking translation、`/api/v1/settings/llm/test`，以及经明确批准的单次辅助生成能力。
- 依赖风险：与未归档的 `chat-per-message-model-routing` change 存在 chat runtime 假设重叠，本 change 在 runtime 方向上应视为 supersede/整合来源。
