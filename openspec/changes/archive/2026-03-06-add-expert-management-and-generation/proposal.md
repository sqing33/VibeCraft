## Why

当前系统里的 expert 主要由模型设置镜像生成，用户只能在诊断页看到只读列表，无法把“专家”作为独立资产来管理。随着专家需要具备领域说明、技能组合、主副模型回退和 AI 生成能力，现有模型页已不足以承载该工作流。

## What Changes

- 新增独立的 `专家` 设置标签页，展示系统专家与用户自定义专家的完整信息。
- 新增可编辑的自定义专家配置模型，支持描述、分类、头像、启用技能、主模型/副模型、失败回退策略等字段。
- 新增 AI 专家生成流程：用户在设置页与 AI 对话，系统通过 `expert-creator` skill 产出结构化专家草稿，并允许发布到专家列表。
- 新增专家设置 API，用于读取、保存、生成和校验专家配置。
- 修改 expert 运行链路，使 SDK expert 支持主模型失败后自动切换副模型重试。

## Capabilities

### New Capabilities
- `expert-builder`: 通过 AI 对话与 expert-creator skill 生成结构化专家草稿，并返回可直接发布的数据。

### Modified Capabilities
- `experts`: expert 配置从“仅运行时条目”扩展为“可展示、可配置、可回退”的资产，并新增设置 API 与回退执行语义。
- `ui`: 系统设置从两个标签页扩展为三个标签页，并新增专家列表、详情与 AI 创建交互。

## Impact

- 后端：`backend/internal/config`, `backend/internal/expert`, `backend/internal/chat`, `backend/internal/runner`, `backend/internal/api`
- 前端：`ui/src/app/components`, `ui/src/lib/daemon.ts`, `ui/src/stores/daemonStore.ts`
- 文档/规范：`openspec/specs/*`, `PROJECT_STRUCTURE.md`, `.codex/skills/expert-creator/SKILL.md`
