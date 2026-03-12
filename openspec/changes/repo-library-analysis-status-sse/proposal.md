## Why

Repo Library 左侧仓库列表需要在分析进行中/失败时给出清晰反馈（转圈/叉），并在分析状态变化时自动更新，避免用户手动刷新与误判。

## What Changes

- 后端新增 SSE 事件流接口 `GET /api/v1/repo-library/stream`，在 repo analysis 状态变更时推送 `repo_library.analysis.updated` 事件。
- 前端订阅 SSE 事件，在 Repo Library 所有页面实时刷新仓库摘要列表。
- 左侧仓库列表每行右侧新增状态图标：
  - `queued/running` 显示转圈
  - `failed` 显示叉
  - `succeeded` 不显示

## Capabilities

### New Capabilities

- `repo-library-analysis-status-stream`: 通过 SSE 推送 Repo Library analysis 状态变更事件，驱动 UI 实时更新。

### Modified Capabilities

- `repo-library-ui`: 左侧仓库列表展示 analysis 状态图标，并通过 SSE 自动刷新列表状态。

## Impact

- Backend：新增一个 SSE broker 与 stream handler，并在 Repo Library analysis 生命周期关键点广播事件。
- UI：新增 EventSource 订阅逻辑与列表项状态图标渲染；仍以 `GET /repo-library/repositories` 为真相源。

