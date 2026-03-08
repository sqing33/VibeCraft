## Why

当前聊天系统只把会话和最终 user/assistant 消息持久化到后端，运行中的 thinking、命令、计划、提问、翻译结果和过程时间线仍主要保存在前端运行态。浏览器刷新、WebSocket 重连或页面重新挂载时，最终回答虽然还能从后端恢复，但本轮过程详情会丢失、重复或错位，导致对话记录不稳定且难以排障。

现在需要把 Codex 对话过程改成“后端为唯一真相源”的持久化模型，让历史消息与运行中时间线都能从后端恢复，彻底消除刷新丢过程、过程错乱和翻译状态丢失的问题。

## What Changes

- 后端新增聊天 turn 与时间线条目的持久化模型，按 turn 保存结构化过程条目及其聚合状态。
- Codex 运行时结构化事件在广播给前端前先写入后端，thinking 翻译结果也回写到对应条目，而不再只存在前端内存。
- 后端新增读取会话时间线快照的 API，前端进入会话时先拉历史消息与 turn 时间线，再订阅 WebSocket 增量。
- 前端移除以 `sessionStorage` 作为过程真相源的职责，只保留少量视图辅助状态，历史过程与运行中过程统一从后端快照派生。
- 修正 reasoning 归并规则：优先使用可读 `summaryTextDelta`，仅在没有 summary 的情况下才回退 raw reasoning，避免重复字符。

## Capabilities

### New Capabilities
- `chat-turn-timeline`: 持久化会话内每一轮对话的结构化过程条目，并提供可恢复的时间线读取能力。

### Modified Capabilities
- `cli-chat-streaming`: 结构化运行时 feed 从仅广播升级为“先持久化、再广播、可恢复重建”。
- `cli-chat-thinking`: thinking 分段与 reasoning 归并规则升级为后端持久化语义，避免 summary/raw 重复显示。
- `thinking-translation`: thinking 翻译结果需要与具体时间线条目一起持久化和恢复。
- `ui`: `#/chat` 刷新后必须从后端恢复完整过程时间线，运行中 turn 也必须稳定续看。
- `store`: SQLite schema 与 store 接口需要新增 chat turn / timeline item 持久化能力。

## Impact

- 后端：`backend/internal/store/*`、`backend/internal/chat/*`、`backend/internal/api/chat.go` 与迁移/测试。
- 前端：`ui/src/stores/chatStore.ts`、`ui/src/app/pages/ChatSessionsPage.tsx`、`ui/src/lib/chatTurnFeed.ts`、`ui/src/lib/daemon.ts`。
- API：新增 chat timeline 读取接口，现有 `messages` 与 WebSocket 事件保持兼容。
- 数据库：`state.db` 需要新增 chat turn / timeline item 相关表与索引。
