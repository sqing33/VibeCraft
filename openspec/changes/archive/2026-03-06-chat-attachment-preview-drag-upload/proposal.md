## Why

当前聊天附件已经能上传并发送给模型读取，但用户还不能在发送框里拖拽文件，也不能在历史消息里直接预览图片/PDF。要把附件从“能传”提升到“好用”，需要补上拖拽上传和基础预览闭环。

## What Changes

- 在聊天发送框增加拖拽上传能力，支持把文件拖到 composer 区域加入待发送附件。
- 为已发送附件增加预览入口：图片在 UI 中弹窗预览，PDF 在 UI 中弹窗内嵌预览。
- 后端增加聊天附件内容读取接口，按 `session_id + attachment_id` 提供受限附件内容访问。
- UI 为待发送附件与历史附件统一提供“预览”交互；不支持预览的文本类附件保留元数据展示，不新增编辑器预览。

## Capabilities

### New Capabilities

### Modified Capabilities
- `chat-attachments`: 增加已持久化附件的内容读取与预览支撑能力。
- `ui`: 聊天页 composer 增加拖拽上传，消息附件增加预览交互与预览弹窗。

## Impact

- 后端：`/api/v1/chat/*` 路由、附件读取鉴权/响应头、附件元数据查询复用。
- 前端：`ChatSessionsPage` 的 drag state、预览 modal、待发送/历史附件交互。
- OpenSpec：更新 `chat-attachments` 与 `ui` 基线规范，并归档本次 change。
