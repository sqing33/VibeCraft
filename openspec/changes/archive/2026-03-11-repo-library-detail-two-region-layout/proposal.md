## Why

当前仓库详情页虽然已经把信息分层，但主区仍然像三个并列面板，信息流被切得太碎。用户更希望左边稳定地承载仓库上下文，右边集中承载“选卡 + 阅读”，让视线只在两大区域之间切换。

## What Changes

- 把仓库详情主区从三栏改成 `2:3` 的双区域布局。
- 左侧区域改为上下堆叠：顶部放快照/分析运行选择，底部放仓库上下文摘要。
- 右侧区域改为上下堆叠：顶部放知识卡片选择带，底部放卡片详情与 evidence。
- 将知识卡片列表改成横向滚动的轻量选择带，每张卡片只展示标题和类型标签，不再重复展示详情摘要。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `repo-library-detail-workspace`: 仓库详情工作台的主布局、卡片导航形态和滚动职责发生调整。

## Impact

- 前端：主要改动 `ui/src/app/pages/RepoLibraryRepositoryDetailPage.tsx` 的布局与卡片选择带样式。
- Specs：更新 `repo-library-detail-workspace` 对主区域结构和滚动职责的描述。
