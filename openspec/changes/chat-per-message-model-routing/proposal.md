## Why

当前 `#/chat` 的对话能力基于 SDK（OpenAI/Anthropic）已具备会话持久化、流式渲染与基础压缩，但“每条消息可选模型/Expert”和“高质量长期记忆”仍缺少端到端规范与实现支撑。需要在不引入 CLI 的前提下，把多模型切换与记忆能力产品化，保证可续聊、可排障、可演进。

## What Changes

- 对话 turn API 支持每条消息可选 `expert_id`，并把“本条使用的 expert/provider/model”写入消息记录，确保历史可追溯。
- 多模型切换时对 provider anchor（OpenAI `previous_response_id` / Anthropic `container`）做安全策略：避免跨模型/跨 provider 串线，必要时自动回退到本地 summary + recent 重建上下文。
- 自动/手动 compaction 升级为 LLM 总结：压缩生成结构化中文 summary，并在失败时回退到本地规则总结，避免 turn 失败。
- UI 在发送框提供“本条 Expert”选择；消息与流式状态展示本条使用的 provider/model；刷新后仍保持正确标注。
- 伴随实现需要升级本地 SQLite schema（chat_messages 增加 metadata 字段）。

## Capabilities

### New Capabilities

- `chat-turn-model-routing`: 支持“每条消息可选 Expert/模型”，并在消息级别持久化 metadata（expert/provider/model）与 anchor 安全策略。
- `chat-compaction-llm`: 基于 LLM 的对话压缩总结（自动触发 + 手动触发），输出可长期维护的结构化 summary。

### Modified Capabilities

- `chat-session-memory`: turn API、消息数据模型、anchor 回退路径与 compaction 行为升级。
- `ui`: `#/chat` 交互与展示能力扩展（每条消息选 expert、显示本条模型、流式状态标注）。

## Impact

- 后端：`/api/v1/chat/*` API、chat manager、store schema migration、WebSocket 事件 payload。
- 前端：ChatSessionsPage、chatStore、daemon.ts 类型与请求体。
- 数据：SQLite schema 从 v2 升级（新增列兼容旧数据）。
