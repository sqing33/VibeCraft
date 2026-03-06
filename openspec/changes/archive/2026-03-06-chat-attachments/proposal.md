## Why

当前 `#/chat` 只支持纯文本多轮对话，无法把图片、PDF、文本代码文件作为当前 turn 的上下文一起发给 OpenAI / Anthropic SDK。随着聊天页逐渐承担“问代码、问文档、问截图”的真实使用场景，需要在保持 SDK-first 架构的前提下补齐附件上传与多模态输入能力。

## What Changes

- 为聊天 turn API 增加附件上传能力，支持 `multipart/form-data` 发送文本 + 多文件，且允许“仅附件、无文字”发送。
- 新增会话级附件持久化：附件文件写入本地数据目录，附件元数据与消息关联并随消息查询返回。
- OpenAI / Anthropic SDK 调用升级为 provider-native 多模态输入构造：图片、PDF、文本/代码文件按各自 SDK 支持的 block/input 发送，而不是仅拼文本路径。
- 带附件历史的会话在 anchor 丢失时支持从本地消息 + 附件重建上下文；自动 compaction 遇到附件会话时采用安全跳过策略，避免错误压缩多模态上下文。
- UI 聊天发送框增加“上传附件”入口、已选附件标签与消息内附件列表展示。

## Capabilities

### New Capabilities
- `chat-attachments`: 支持聊天消息上传、持久化并发送附件给模型读取，覆盖图片、PDF、文本/代码文件。

### Modified Capabilities
- `chat-session-memory`: turn API、上下文重建与 compaction 策略需要适配带附件的多模态会话。
- `ui`: `#/chat` 发送框与消息渲染需要支持附件选择、发送与展示。
- `store`: 本地存储需要新增附件元数据表与聊天附件文件目录布局。

## Impact

- 后端：`/api/v1/chat/*` API、chat manager、SQLite migration、文件存储 helper、provider 输入构造。
- 前端：`ChatSessionsPage`、`chatStore`、`daemon.ts` 请求与类型。
- 数据：SQLite 新增 `chat_attachments` 表；本地数据目录新增聊天附件存储子目录。
