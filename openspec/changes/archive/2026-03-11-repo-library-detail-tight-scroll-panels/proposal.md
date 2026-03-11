## Why

当前仓库详情页虽然已经改成 `2:3` 双区域，但边距、标题层级和滚动语义还不统一。用户需要更紧凑的卡片排布、更明确的局部滚动边界，以及更少的重复标题与说明文本。

## What Changes

- 收紧详情页主卡片区域的左侧与顶部留白。
- 重新引入受控高度，让四个主卡片保持在同一工作区内，由卡片内容区域承担纵向滚动。
- 将右侧知识卡片选择区与卡片详情区合并成一个统一视觉容器。
- 移除多余的说明文字与部分标题，把生成时间移到页头状态位置。

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `repo-library-detail-workspace`: 收紧边距、调整标题信息、合并右侧卡片容器，并修改局部滚动行为。

## Impact

- 前端：主要改动 `ui/src/app/pages/RepoLibraryRepositoryDetailPage.tsx`，少量改动 `ui/src/app/components/RepoLibraryShell.tsx` 以支持详情页定制内容留白。
- Specs：更新 `repo-library-detail-workspace` 对局部滚动和顶部信息展示的描述。
