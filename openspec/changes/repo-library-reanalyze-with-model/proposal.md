## Why

Repo Library 需要支持对同一仓库用不同 CLI 工具与模型重复分析，以便对比结果差异与回归质量。当前只能在“添加仓库”页手动重新填写参数，仓库详情页缺少一键复用既有分析参数的入口，导致操作成本高且容易填错。

## What Changes

- 在仓库详情页的“分析/同步最新回复”区域新增“用其他模型分析”入口，一键跳转到“添加仓库/发起分析”页面。
- 跳转时自动预填当前分析的创建参数：`repo_url`、`ref`、`features`、`depth`、`language`、`analyzer_mode(agent_mode)`、`cli_tool_id`、`model_id`。
- “添加仓库/发起分析”页面支持从全局草稿读取一次性预填，并在填充后自动清理草稿，避免污染后续手动操作。

## Capabilities

### New Capabilities

- `repo-library-analysis-prefill`: 支持从仓库详情页一键复用并预填分析参数，快速发起对比分析。

### Modified Capabilities

- `repo-library-ui`: 增加跨页面的“复用分析参数并跳转”交互与状态管理。

## Impact

- UI：新增一个全局分析草稿状态（Zustand store），仓库详情页新增按钮并写入草稿；添加仓库页读取草稿并预填表单。
- API：不新增后端接口（复用既有创建分析接口），仅复用已有分析记录字段。

