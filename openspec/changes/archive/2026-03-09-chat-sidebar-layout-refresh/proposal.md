## Why

当前 `#/chat` 页面把全局导航放在顶部，把会话列表放在一个独立的圆角卡片里，导致聊天工作区被切成两层结构，和目标中的一体化左侧栏体验不一致。需要把聊天页调整成更稳定的左右工作台布局，让左栏承担导航与会话切换，右侧专注对话本身。

## What Changes

- 将聊天页改为左右布局：左侧为一体化侧栏，右侧为保留边框的对话区域。
- 将当前顶部栏中的三个页面导航迁移到聊天页左侧栏顶部，并按三行纵向排列。
- 将会话列表保留在左侧栏中部，移除其外层独立圆角描边容器，让左栏与页面背景融为一体。
- 将顶部栏中的健康状态、连接状态、主题切换、开发工具、系统设置迁移到左侧栏底部。
- 重排右侧对话头部：左侧显示会话参数信息，中间显示大标题，右侧显示状态与分叉按钮。
- 保留现有消息流可读宽度与底部输入区锚定行为。

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `chat-page-immersive-layout`: 调整聊天页的沉浸式布局，定义一体化左侧栏与三段式对话头部。

## Impact

- 前端壳层：`ui/src/App.tsx`
- 聊天页布局：`ui/src/app/pages/ChatSessionsPage.tsx`
- 复用现有状态/设置能力：`ui/src/app/components/Topbar.tsx`、`ui/src/app/components/DevToolsDialog.tsx`、`ui/src/app/components/SettingsDialog.tsx`
