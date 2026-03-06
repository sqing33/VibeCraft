## Context

当前设置页使用“Sources + Models”两个独立区块编辑 LLM 配置，而持久化结构仍然是平铺的 `sources[]` 与 `models[]`。用户这次想优化的是配置心智模型：一个 API 源通常只对应一种 SDK，因此 SDK 不应在每个模型上重复选择；模型应直接归属于某个 Source。与此同时，后端当前对模型 ID/模型名只做 trim，不做大小写规范化，导致用户输入大写模型名时，后续调用使用的标识不可控。

## Goals / Non-Goals

**Goals:**

- 在 UI 上移除独立模型区块，改为在每个 Source 卡片内部维护模型列表。
- 将 SDK 选择收敛到 Source 级别，并让该 Source 下所有模型继承同一个 provider。
- 允许用户在 UI 中输入带大写的模型名，但保存、专家注册和测试调用都使用小写模型标识。
- 尽量保持现有 API 路径与持久化格式兼容，避免引入额外迁移成本。

**Non-Goals:**

- 不改动聊天页、工作流页等专家选择交互。
- 不引入新的后端设置 API，也不把磁盘配置格式整体改成嵌套结构。
- 不处理非 OpenAI / Anthropic 之外的新 SDK 类型。

## Decisions

### 1. 保持后端 settings payload 扁平，前端负责按 Source 分组

保留 `GET/PUT /api/v1/settings/llm` 现有 `sources[] + models[]` 结构，前端加载时按 `source_id` 分组，保存时再展开。这样可以最小化后端接口变更和配置文件兼容风险，同时满足用户想要的编辑体验。

备选方案是直接把后端配置结构改成 `source.models[]` 嵌套，但这会影响配置校验、写盘格式、专家镜像与潜在兼容性，超出这次 UI/交互优化的必要范围。

### 2. 用单个模型输入同时生成显示标签与规范化标识

Source 内的每个模型只暴露一个“模型 ID”输入框。保存时：

- `label` 使用用户原始输入（保留大小写显示）
- `id` 与 `model` 统一使用 `strings.ToLower(trim(input))`

这样可以兼顾“我想看大写名字”和“请求必须小写”两个目标，并保持专家列表/调用链上的真实标识稳定。

### 3. 后端在保存、镜像、测试三条路径统一归一化

除了 UI 保存前主动转小写，后端也会在 `NormalizeLLMSettings`、`MirrorLLMToExperts` 和 `/settings/llm/test` 中再做一次小写归一化。这样即使未来有其他调用方直接写 API，也能得到一致行为。

## Risks / Trade-offs

- [旧配置的 label 可能不是模型名] → 加载时优先展示与 `model` 大小写等价的 `label`，否则回退展示 `model`，避免把任意自定义标签误当作真实模型 ID。
- [大小写归一化后可能出现重复 ID] → 在前端保存前与后端校验时都按小写后的值检查重复，尽早提示冲突。
- [Source provider 以前可为空] → 前端对新编辑流默认给 Source 设置 provider，并在旧数据加载时回退到关联模型的 provider 或 `openai`，减少空值影响。

## Migration Plan

1. 保持磁盘上的 `llm.sources[] / llm.models[]` 结构不变。
2. 旧配置加载到 UI 时，按 `source_id` 聚合为 Source 内模型列表。
3. 用户保存后，模型 `id/model` 被写成小写，`label` 保留原始显示文本。
4. 如需回滚，只需恢复旧版 UI；配置文件结构本身无需迁移。

## Open Questions

- 暂无。本次需求已明确：以 Source 为单位配置 SDK，并对模型标识统一小写化。
