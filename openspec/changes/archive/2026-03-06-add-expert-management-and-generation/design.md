## Context

当前 `cfg.Experts` 同时承担运行时 expert 列表与模型镜像产物两种职责，导致 UI 无法区分“系统生成 expert”和“用户定义 expert”。同时 chat / workflow 都直接消费 `expert.Resolve()` 产物，若要支持主副模型回退，必须在 expert 解析与执行链路上增加 fallback 元数据。

设置页目前只有 `连接与诊断` 与 `模型` 两个标签。模型页负责 source/model 的基础能力管理，而用户新提出的“专家”其实是对模型、skill、prompt 和回退策略的组合资产，更适合作为单独标签页。

## Goals / Non-Goals

**Goals:**
- 将专家管理从模型页中解耦，提供独立的展示与发布入口。
- 允许用户通过 AI 对话生成专家草稿，而不是手工填写大表单。
- 让生成出的专家立即可用于 chat / workflow 运行，并支持 SDK 主副模型回退。
- 在不暴露 API Key 的前提下，向 UI 返回足够完整的专家元数据。

**Non-Goals:**
- 本次不实现 skill 的真实执行/装载运行时，`enabled_skills` 先作为专家元数据与生成约束保存。
- 本次不做复杂的版本树、审批流或多人协作发布。
- 本次不重做聊天主页面，AI 创建专家仅在设置页内提供轻量对话面板。

## Decisions

### 1. 保持 `cfg.Experts` 为单一存储源，但区分 `managed_source`

沿用现有 `cfg.Experts`，新增 `managed_source` 区分：
- `builtin`：默认内建 expert（如 `master`/`bash`/`demo`）
- `llm-model`：由模型设置自动镜像生成的系统 expert
- `expert-profile`：专家标签页里管理的自定义专家

这样可以复用现有 registry / scheduler / chat 代码路径，同时允许 `PUT /api/v1/settings/experts` 只覆盖 `expert-profile`，保留 builtin 与 llm-model 条目。

**备选方案：**新增 `expert_profiles` 顶层配置。优点是持久化更干净；缺点是需要全链路新增编译层和迁移逻辑。本次优先最小可落地。

### 2. 自定义专家以“模型引用 + 运行时派生字段”保存

自定义专家新增 `primary_model_id` / `secondary_model_id`，保存时根据 `cfg.LLM` 解析出 provider/model/base_url/env，并写回 expert 条目。这样 UI 只需要处理模型引用，运行链路仍可直接使用 expert registry。

**备选方案：**只保存模型引用，在 Resolve 时实时查找 `cfg.LLM`。缺点是 registry 需要持有更多上下文；当前方案更贴近既有实现。

### 3. SDK fallback 由 expert 解析后写入 `RunSpec`，chat 与 execution 共用

在 `runner.RunSpec` 中新增 SDK fallback 列表。`expert.Resolve()` 根据 expert 的 secondary model 与 fallback 条件填充。`SDKRunner` 在主模型失败时按 fallback 顺序重试；`chat.Manager` 也复用同一 fallback 列表进行 provider 调用重试，并记录最终 provider/model。

**备选方案：**只在 chat 或 workflow 某一处单独实现 fallback。缺点是行为不一致。

### 4. AI 创建专家使用专门的生成 API，而不是复用 chat sessions

新增 `POST /api/v1/settings/experts/generate`，请求体携带对话消息、builder expert id 与当前模型/skill catalog。服务端拼接 `expert-creator` skill 指令，调用 builder expert 的 provider，并要求输出 `expert_builder_v1` 结构化 JSON。

生成 API 设计为无状态，前端在设置页本地维护消息历史与当前草稿，避免引入新的数据库表与迁移。

### 5. 专家标签页采用“三栏信息密度”布局

标签页内部分为：
- 顶部摘要区：专家数量、可用生成模型、可用 skills 数量、创建按钮
- 左侧列表区：专家卡片（支持筛选状态/来源）
- 右侧详情区：职责、模型策略、fallback、skills、提示词摘要、生成信息

AI 创建使用 modal/drawer，左侧对话，右侧草稿预览与发布操作，降低主标签页复杂度。

## Risks / Trade-offs

- [技能仅做元数据保存] → 先保证专家配置流闭环，后续再接入实际 skill runtime。
- [将派生字段写回 cfg.Experts] → 配置文件会包含部分运行时冗余字段；通过 `managed_source` 控制重建逻辑避免失真。
- [Anthropic / OpenAI 输出结构化 JSON 的兼容性差异] → 统一使用 `expert_builder_v1` schema；demo provider 提供离线回退；生成结果仍在后端做二次校验。
- [fallback 重试可能导致重复计费] → 仅在请求失败时触发一次副模型重试，并在 UI 中显式展示 fallback 策略。

## Migration Plan

1. 扩展 `ExpertConfig` 与 `RunSpec` 数据结构。
2. 在 config 保存/加载链路中加入 expert 编译与 llm-model 重建逻辑。
3. 新增专家设置 API 与 expert builder 生成服务。
4. 前端新增专家标签页与生成对话 modal。
5. 运行后通过 API / UI / 测试验证 fallback 与发布流程。

回滚策略：移除专家标签页入口，并保留旧版 `GET /api/v1/experts` 与模型页行为；由于 builtin experts 不被覆盖，最坏情况下仅丢失自定义 expert-profile 条目。

## Open Questions

- 当前版本的 `enabled_skills` 是否需要在 workflow/chat 执行时下传到 prompt 中，还是先只展示与保存。
- 未来是否需要把 `expert-creator` 生成历史写入数据库，支持继续编辑旧草稿。
