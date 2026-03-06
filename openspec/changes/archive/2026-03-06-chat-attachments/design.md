## Context

`vibe-tree` 当前聊天链路是 SDK-first：前端通过 `POST /api/v1/chat/sessions/:id/turns` 提交 JSON 文本，后端先把 user/assistant message 写入 SQLite，再分别调用 OpenAI Responses 和 Anthropic Messages SDK 进行流式生成。现有实现依赖 provider anchor（OpenAI `previous_response_id` / Anthropic `container`）与本地 summary + recent 的重建路径，但整个链路默认“输入一定是纯文本”，消息模型与 store 也没有附件关系。

本次变更是跨 UI、HTTP API、chat manager、store schema 与磁盘存储的交叉改动，同时要求保持现有纯文本会话兼容、WebSocket 事件兼容，以及对 OpenAI / Anthropic 两条 SDK 路径都提供 provider-native 的多模态输入。

## Goals / Non-Goals

**Goals:**
- 在同一 turn 中支持文本 + 多附件上传，并允许仅附件发送。
- 附件在本地可持久化、可随消息查询返回、页面刷新后仍可展示。
- OpenAI / Anthropic 以各自 SDK 支持的图片/PDF/文本输入结构发送附件，不依赖 CLI 或 `@path` 路径约定。
- 带附件历史的会话在 anchor 不可用时仍能从本地完整重建多模态上下文。
- 避免自动 compaction 错误压缩附件上下文，优先保证正确性。

**Non-Goals:**
- 不实现拖拽上传、附件预览、下载、彻底删除或后台清理任务。
- 不在设置页新增“模型是否支持多模态”的配置编辑 UI。
- 不支持 Office 文档、音视频等超出首批范围的附件类型。

## Decisions

1) **沿用现有 turn endpoint，扩展为 JSON + multipart 双协议兼容**

- 方案：`POST /api/v1/chat/sessions/:id/turns` 同时支持原有 JSON 与新的 `multipart/form-data`。无附件时前端继续发送 JSON，有附件时发送 FormData。
- 备选：新增独立 upload endpoint，先上传后引用 attachment id。
- 取舍：保留最小 API 面积与现有前端/后端路由结构，避免先上传再发送的两阶段状态复杂度。

2) **附件文件统一持久化到 data dir，而不是依赖原始本地路径**

- 方案：附件落到 `<dataDir>/chat-attachments/<session_id>/<message_id>/`，数据库只保存相对路径与元数据。
- 备选：只在内存中临时持有附件字节，或者直接引用用户本地绝对路径。
- 取舍：本地持久化可保证刷新后历史可见、后续重建上下文可读、路径不会受 UI 进程工作目录影响，也避免把用户绝对路径暴露给 UI。

3) **数据库增加独立 `chat_attachments` 表，而不是把附件 JSON 塞进 `chat_messages`**

- 方案：新增附件表，以 `message_id` 关联消息；`ListChatMessages` 批量回填附件数组。
- 备选：为 `chat_messages` 增加一个 JSON 字段。
- 取舍：独立表更利于迁移、索引、批量查询与后续扩展（预览、删除、审计）。

4) **provider-native 多模态输入按类型分别构造，而不是一律提取文本内容**

- 方案：
  - OpenAI：图片用 `input_image`，PDF 用 `input_file`，文本/代码文件读内容后转 `input_text`。
  - Anthropic：图片用 `ImageBlock`，PDF 用 `DocumentBlock`，文本/代码文件读内容后转 `TextBlock`。
- 备选：把所有附件都转成本地文本摘要后并入 prompt。
- 取舍：provider-native 输入能最大化保留图片/PDF 能力；文本/代码文件则直接读取内容，避免额外文件上传 API 或 provider 文件管理复杂度。

5) **带附件会话禁用自动 compaction，优先保证上下文正确性**

- 方案：若 session 历史存在附件，则 `ensureCompaction` 直接跳过自动 compaction；手动 compaction API 保持可用但也采用同样的安全跳过/拒绝策略。
- 备选：为附件生成文本摘要再参与 compaction。
- 取舍：现有 compaction 模型只面向纯文本摘要，贸然压缩多模态上下文容易丢失关键信息。v1 先牺牲超长会话容量，保证正确性。

6) **仅附件发送由后端补固定占位提示词，前端显示“仅附件”**

- 方案：当用户未输入文字但上传了附件时，后端把模型输入前缀设为固定说明，UI 的用户消息正文显示 `（仅附件）`。
- 备选：强制用户填写文字，或由前端偷偷补一段默认文案。
- 取舍：后端补位更统一，避免不同客户端行为不一致；UI 仍能清晰表达这条消息没有文字正文。

## Risks / Trade-offs

- **[兼容网关不一定支持多模态]** → 直接透传 provider 错误，不按模型名猜测能力，保持行为可诊断。
- **[附件历史会更快触发上下文上限]** → v1 明确跳过自动 compaction，并在超限时返回清晰错误，建议用户新建/分叉会话。
- **[附件字节进 DB 前后的内存占用上升]** → 对单文件与总大小做严格限制，API 层在解析阶段尽早拒绝超限请求。
- **[SQLite 列表查询可能引入 N+1]** → `ListChatMessages` 使用“先查 messages、再按 message_id 批量查 attachments”回填。
- **[多 provider 输入构造逻辑分叉]** → 抽出独立 provider input builder，避免在 `RunTurn` 中堆叠大量条件分支。

## Migration Plan

- 通过新的 schema migration 创建 `chat_attachments` 表，不修改现有 `chat_messages` 主体字段。
- 原有纯文本消息无需回填附件；旧数据读取时 `attachments` 返回空数组或省略。
- API 向后兼容：旧版前端继续使用 JSON turn 请求不受影响。
- 如需回滚，代码回滚后旧表可暂时保留；新版本产生的附件文件不会影响纯文本会话读取。

## Open Questions

- v1 不引入附件预览/下载接口，因此 `storage_rel_path` 先作为内部字段存储，不直接暴露给 UI。
- 若后续要支持更大 PDF 或更多文件，可能需要引入流式保存与 provider file upload；本次不做。
