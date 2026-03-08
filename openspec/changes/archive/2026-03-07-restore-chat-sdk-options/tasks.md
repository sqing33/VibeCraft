## 1. OpenSpec

- [x] 1.1 为 `chat-session-memory` 补充 CLI + SDK 混合对话的 delta spec
- [x] 1.2 为 `ui` 补充 Chat 运行时混合选择器的 delta spec

## 2. Backend

- [x] 2.1 调整 chat create-session / turn API，允许 chat-capable SDK helper experts
- [x] 2.2 增加 integration test，覆盖 SDK helper expert 创建会话并发送对话

## 3. Frontend

- [x] 3.1 将 Chat 页选择器改为 CLI + SDK 混合运行时列表
- [x] 3.2 让新建会话/发送消息在 CLI 与 SDK 场景下构造不同请求参数
- [x] 3.3 保持会话回填、模型过滤与消息身份展示在 SDK 场景下正常工作

## 4. Validation

- [x] 4.1 运行后端 chat integration test
- [x] 4.2 运行前端 build 或 typecheck 验证 Chat 页改动
