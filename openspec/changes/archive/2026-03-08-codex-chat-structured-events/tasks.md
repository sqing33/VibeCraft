## 1. OpenSpec & Parser Foundation

- [x] 1.1 补充 Codex 结构化 turn feed proposal / design / delta specs
- [x] 1.2 扩展 Codex app-server parser，兼容 `item/*` 与 `codex/event/*` 两套通知
- [x] 1.3 为 parser 与事件映射补充单元测试

## 2. Backend Structured Event Feed

- [x] 2.1 为 Codex turn 新增 entry 级状态聚合与 `chat.turn.event` 广播
- [x] 2.2 保留旧 `chat.turn.delta` / `chat.turn.thinking.delta` 兼容广播
- [x] 2.3 把 thinking translation 接入结构化 feed 的 thinking entry

## 3. Frontend State & Rendering

- [x] 3.1 为 chatStore 新增活动 turn feed / 已完成 feed 状态与 reducer
- [x] 3.2 让 ChatSessionsPage 订阅 `chat.turn.event` 并按 kind 分层渲染
- [x] 3.3 新增结构化条目组件，分别展示 progress / thinking / tool / plan / question / answer

## 4. Validation & Spec Sync

- [x] 4.1 运行后端 focused tests 与前端类型检查
- [x] 4.2 更新 `PROJECT_STRUCTURE.md` 中新增关键文件职责
- [x] 4.3 同步基线 specs 并归档 change
