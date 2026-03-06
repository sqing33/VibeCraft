## Why

当前专家生成流程虽然支持多轮消息输入，但本质上仍是“单次请求生成单次草稿”，不会保存会话历史，也无法在后续继续基于旧上下文微调专家。为了把专家生成功能升级为真正可持续设计的工作台，需要引入持久化的 builder session、消息历史和草稿快照。

## What Changes

- 新增专家生成会话能力：支持创建、读取、继续和归档专家 builder session。
- 新增专家生成历史持久化：保存用户/AI 消息记录以及每轮生成后的专家草稿快照。
- 新增从会话快照发布专家的流程，并将发布专家与其来源 session / snapshot 关联。
- 修改专家设置页，让 AI 创建专家升级为长对话工作台，支持继续优化已有专家。
- 修改 expert builder API，从无状态生成升级为“会话驱动生成”。

## Capabilities

### New Capabilities
- `expert-builder-sessions`: 专家生成会话、消息历史、草稿快照和发布回溯。

### Modified Capabilities
- `expert-builder`: 专家生成从单次请求升级为有状态的长对话会话。
- `experts`: 专家配置新增来源 session / snapshot 元数据，并支持继续优化已有专家。
- `ui`: 专家设置页新增历史会话、快照预览、继续对话和从快照发布。
- `store`: SQLite schema 新增专家 builder 相关表与索引。

## Impact

- 后端：`backend/internal/store`, `backend/internal/api`, `backend/internal/expertbuilder`, `backend/internal/config`
- 前端：`ui/src/app/components/ExpertSettingsTab.tsx`, `ui/src/lib/daemon.ts`
- 规范/文档：`openspec/specs/{expert-builder,experts,ui,store}`, `PROJECT_STRUCTURE.md`
