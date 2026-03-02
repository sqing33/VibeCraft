## Why

HeroUI 迁移完成后，LLM 设置页的 Model Profile 的 Source 下拉允许被清空，可能导致 `source_id` 为空，从而出现保存/测试失败的用户体验问题。同时 Tailwind 配置中仍残留未使用的 Radix accordion 动画配置，增加维护噪音。

## What Changes

- 修复 LLM 设置页：Source Select 禁止空选择，并在保存/测试前对缺失 Source 的配置给出明确错误提示。
- 清理 Tailwind 配置：移除未使用的 `accordion-*` keyframes/animation（历史 Radix 依赖残留）。

## Capabilities

### New Capabilities

- （无）

### Modified Capabilities

- `ui`: LLM Model Profiles 在 UI 中必须始终绑定一个有效的 Source；UI 必须阻止保存/测试 `source_id` 为空的配置。

## Impact

- 影响范围：仅前端 `ui/`（设置页交互与 Tailwind 配置）。
- 不影响：后端 daemon API、desktop 壳逻辑、工作流执行链路。

