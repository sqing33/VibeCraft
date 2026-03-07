## 1. OpenSpec artifacts

- [x] 1.1 创建 `improve-codex-chat-streaming` change 目录
- [x] 1.2 编写 proposal / design / tasks
- [x] 1.3 编写 delta specs（`cli-chat-streaming` / `cli-chat-thinking` / `chat-cli-session-resume` / `cli-runtime`）

## 2. Backend implementation

- [x] 2.1 新增 Codex app-server JSON-RPC client（stdio 握手、请求、通知分发、关闭）
- [x] 2.2 在 chat manager 中为 Codex 路径接入 app-server 优先逻辑
- [x] 2.3 保留 legacy wrapper 回退，并限制为 early failure 才回退
- [x] 2.4 将细粒度 delta 映射到现有 `chat.turn.delta` / `chat.turn.thinking.delta`
- [x] 2.5 补齐 token usage、thread id 持久化与工件写入

## 3. Tests and docs

- [x] 3.1 新增 parser / notification 单测
- [x] 3.2 运行与 Codex chat 相关的 targeted tests
- [x] 3.3 更新 `PROJECT_STRUCTURE.md` 的关键文件索引

## 4. Finalization

- [x] 4.1 `openspec validate improve-codex-chat-streaming`
- [x] 4.2 完成后 archive change 并同步基线 specs
