## Context

- 现状：UI 已迁移到 HeroUI（`@heroui/react`），LLM 设置页的 Model Profile 使用 HeroUI `Select` 选择 Source。
- 问题：当前 Source `Select` 允许被清空，导致 `source_id` 为空字符串；用户点击保存/测试时会失败（或触发后端校验错误），体验不佳。
- 约束：不改动后端 API；保持页面信息架构与主要交互不变；继续使用 HeroUI 组件与现有 `toast()` 适配层。

## Goals / Non-Goals

**Goals:**

- Model Profile 的 Source 选择在 UI 层面不可为空：禁用空选择，并在数据异常时自动回退到第一个可用 Source。
- 保存/测试前对缺失 Source 的配置给出明确的错误 toast，避免“保存失败”这类不透明错误。
- 清理 `ui/tailwind.config.cjs` 中未使用的 Radix accordion 动画配置，降低迁移后的维护噪音。

**Non-Goals:**

- 不重构/重设计 LLM 设置页布局与表单字段。
- 不引入新的表单校验框架，不修改后端校验逻辑。

## Decisions

1. **Source Select 禁止空选择**
   - 使用 HeroUI `Select` 的 `disallowEmptySelection`，避免用户交互层面清空已选项。
   - 当当前 `source_id` 为空或不在 `sourceOptions` 中时，渲染时回退到 `sourceOptions[0]`，并在交互回调中避免写入空字符串。

2. **避免创建无效 Model**
   - 当没有任何 Source 时，阻止“新增 Model”并给出提示（toast），避免进入无法保存的状态。

3. **保存前最小校验**
   - 在 `onSave()` 前检查是否存在 `source_id` 为空的模型；若存在则 toast 错误并中止保存。

4. **Tailwind 配置清理**
   - 在确认项目中无 `animate-accordion-*` 等引用后，移除 `accordion-*` keyframes/animation 配置，避免残留 Radix 变量引用。

## Risks / Trade-offs

- [历史配置不一致导致 source 缺失] → UI 回退到第一个可用 Source；若没有 Source，则提示用户先创建 Source。
- [更严格的校验阻止保存] → 仅阻止明显无效的 `source_id` 为空情况，并提供明确提示文案。

