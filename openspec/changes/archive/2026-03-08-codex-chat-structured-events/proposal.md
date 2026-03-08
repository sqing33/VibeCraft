## Why

当前 `#/chat` 已经能把 Codex CLI 的回答与部分 thinking 通过 WebSocket 推到前端，但运行时内容仍被压扁成 `thinking + answer` 两段字符串：命令执行、计划更新、提问、系统进度与真实 thinking 混在一起，前端只能把它们塞进一个 pending assistant 气泡里。结果是：

- thinking 看起来像“卡住后突然冒一大段”；
- 命令/计划/审批没有独立样式，用户难以判断模型当前在做什么；
- 当前后端只偏向解析文档里的 `item/*` 事件名，对真实环境中的 `codex/event/*` 兼容不足。

需要把 Codex 聊天链路升级为“结构化 turn feed”：后端区分不同事件类型并增量广播，前端按事件种类分层渲染，用不同样式展示 thinking、command、plan、question、progress 与 answer。

## What Changes

- 后端新增统一的 `chat.turn.event` 结构化事件流，兼容 `item/*` 与 `codex/event/*` 两套 Codex app-server 通知。
- Codex 运行时把 answer、thinking、tool command、plan、question、system progress、error 拆成独立 entry，并维持稳定 `entry_id` 进行增量更新。
- 前端 chat store 新增 turn feed 状态模型，按 `append/upsert/complete` 应用事件，不再只依赖 `streamingBySession` / `thinkingBySession` 两个字符串。
- Chat 页面改为分层展示活动 turn feed：system/progress、thinking、tool、plan、question、answer 各自独立样式；turn 完成后把本轮 feed 关联到最终 assistant message。
- 保留现有 `chat.turn.delta` / `chat.turn.thinking.delta` 兼容广播，避免现有链路立即失效。

## Capabilities

### Modified Capabilities

- `cli-chat-streaming`: 从“仅流式回答文本”升级为“结构化 turn feed + 兼容旧 delta”。
- `cli-chat-thinking`: thinking、plan、tool progress、question 进入不同事件层，不再统一混为 thinking 文本。
- `thinking-translation`: 仅翻译 thinking 条目，不污染 tool/progress/answer。
- `ui`: `#/chat` 支持结构化运行中条目与完成后过程详情展示。

## Impact

- 后端：`backend/internal/chat/codex_appserver.go`、`backend/internal/chat/manager.go`、thinking translation 广播路径与相应测试。
- 前端：`ui/src/stores/chatStore.ts`、`ui/src/app/pages/ChatSessionsPage.tsx`，并新增 turn feed 渲染组件。
- 规范：CLI streaming / thinking / translation / UI specs 需要同步更新。
