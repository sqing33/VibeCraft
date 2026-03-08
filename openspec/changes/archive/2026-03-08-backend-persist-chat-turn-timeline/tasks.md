## 1. OpenSpec 对齐

- [x] 1.1 完成 proposal、design 与 delta specs，并通过 OpenSpec 校验
- [x] 1.2 核对现有 chat timeline / streaming / thinking / translation 基线要求，避免与新能力冲突

## 2. 后端存储与 API

- [x] 2.1 为 SQLite 增加 `chat_turns` 与 `chat_turn_items` 表、索引和迁移测试
- [x] 2.2 在 store 中实现 turn / item 的创建、upsert、完成态更新与按 session 聚合读取
- [x] 2.3 新增 chat timeline 读取 API，并补充接口测试

## 3. 聊天运行时持久化

- [x] 3.1 在 chat turn 启动、结构化事件更新、翻译更新和 turn 完成时写入后端 timeline
- [x] 3.2 修正 Codex reasoning 归并逻辑，优先 summary，抑制同 item raw reasoning 重复
- [x] 3.3 增加后端回归测试，覆盖 running/completed 恢复与 reasoning 去重

## 4. 前端恢复链重构

- [x] 4.1 增加 timeline API 客户端与 store 数据模型，改为从后端 turns 快照派生 completed/running timeline
- [x] 4.2 重构 ChatSessions 页面，移除以本地 runtime 缓存为真相源的完成态/待处理态判定
- [x] 4.3 保留必要视图状态并验证刷新、重连、翻译显示与命令折叠交互

## 5. 验证与归档

- [x] 5.1 运行 `openspec status --change backend-persist-chat-turn-timeline`、`go test ./...`、`npm run build`、`npm run lint`
- [x] 5.2 更新必要文档索引后归档该 change
