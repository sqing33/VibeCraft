## Context

当前 Repo Library 的“发起分析”入口集中在 `#/repo-library/repositories` 页面右侧表单。仓库详情页虽提供了“打开分析 Chat / 同步最新回复”，但缺少“复用当前分析参数再跑一次”的入口，导致用户对比不同 CLI 工具或模型时需要手动复制参数，且容易遗漏 `features/depth/language/analyzer_mode` 等字段。

约束：
- Repo Library 采用 hash 路由，当前路由解析不支持 querystring 形式的预填参数。
- 仓库分析创建参数已经在 `repo_analysis_runs` 与关联快照中落盘（features/depth/language/agent_mode/cli_tool_id/model_id 与 snapshot ref）。

## Goals / Non-Goals

**Goals:**
- 在仓库详情页提供“用其他模型分析”入口，复用当前分析参数并跳转到添加仓库页。
- 在添加仓库页支持一次性预填草稿：进入页面时将草稿映射到表单 state，并在应用后清理草稿。
- 预填能力可复用到仓库列表侧栏的“快捷预填”操作（不强依赖后端新增字段）。

**Non-Goals:**
- 不新增新的后端分析创建 API（复用现有 `createRepoLibraryAnalysis`）。
- 不在本次引入 URL query 形式的预填参数（避免改动路由解析与深链接语义）。
- 不改变“同步最新回复”的语义与后端校验流程。

## Decisions

1. **跨页面预填采用全局 store 草稿，而非 URL 参数**
   - 方案 A（选用）：Zustand store 保存 `analysisDraft`，详情页写入草稿并跳转；添加仓库页读取并清理。
   - 方案 B：hash route 增加 querystring，并在路由解析中携带预填参数。
   - 选择理由：方案 A 改动面更小，不改变路由协议，也避免把潜在敏感信息写到 URL；同时更容易做“一次性消费”语义。

2. **预填字段以“可创建分析请求”为准**
   - 预填覆盖：`repo_url/ref/features/depth/language/analyzer_mode/cli_tool_id/model_id`。
   - `ref` 选择：优先 `resolved_ref`，否则 `requested_ref`，再回退 `HEAD`。

## Risks / Trade-offs

- [草稿未清理导致后续污染] → 添加仓库页读取草稿后立即 `clearAnalysisDraft()`，并只在非空草稿时触发。
- [侧栏快捷预填缺少完整字段] → 优先使用列表返回的 `latest_analysis`；若其字段不完整，则按需补一次 `fetchRepoLibraryRepository` 获取完整 analyses 再选最新。
- [模型/工具在当前环境不可用] → 预填仅设置 `selectedCliToolId/selectedModelId`，表单最终仍由 `effectiveCliToolId/effectiveModelId` 校验与回退保证可提交。

