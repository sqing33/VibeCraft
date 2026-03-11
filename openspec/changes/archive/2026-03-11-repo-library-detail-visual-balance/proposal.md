## Why

当前仓库详情页虽然已经具备三栏工作台结构，但三栏视觉权重过于接近，导致上下文、卡片导航和详情阅读像三个等权大面板并列，信息主次不清，页面显得拥挤而别扭。

这次变更不再扩展数据能力，只收敛布局层级和视觉节奏，让页面更像“知识阅读界面”而不是“数据库控制台”。

## What Changes

- 将快照和分析运行选择从左栏移到顶部上下文工具条，减少左栏负担。
- 将左栏缩成窄上下文栏，只保留技术栈摘要和分析会话信息。
- 将中栏改造成更轻的卡片导航列表，弱化大卡片堆叠感。
- 将右栏强化为主详情区，让卡片详情成为唯一重内容焦点。
- 调整 Evidence 区的层级和滚动容器，使其更像详情的支撑区而不是并列主面板。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `repo-library-detail-workspace`: 调整仓库详情工作台的视觉层级、栏位职责和组件摆放方式。

## Impact

- 前端：主要改动 `RepoLibraryRepositoryDetailPage.tsx`，少量样式层级调整即可，不涉及新的后端字段。
- Specs：更新 Repo Library detail workspace 对布局职责的描述。
