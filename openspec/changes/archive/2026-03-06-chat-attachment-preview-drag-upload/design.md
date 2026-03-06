## Context

现有聊天附件实现已经具备上传、持久化与 provider-native 多模态输入能力，但 UI 仍是“点击选择文件 + 纯元数据展示”的最小版本。仓库里已有 HeroUI Modal 可复用，因此预览可以在现有页面内实现，不必引入新依赖。后端当前存储了 `storage_rel_path`，但没有公开读取接口，前端也无法直接访问 data dir 下文件。

## Goals / Non-Goals

**Goals:**
- 在聊天 composer 实现拖拽上传，和现有点击上传共用同一待发送附件状态。
- 为历史图片/PDF 附件提供预览入口。
- 为后端增加最小可用的附件内容读取接口，并返回正确的 `Content-Type` / `Content-Disposition`。

**Non-Goals:**
- 不做文本/代码附件在线编辑器预览。
- 不做附件下载统计、权限系统或签名 URL。
- 不做多文件拖拽排序和拖拽覆盖交互。

## Decisions

1) **预览接口走 daemon 原始文件流，而不是把 base64 塞进消息 API**
- 方案：新增 `GET /api/v1/chat/sessions/:id/attachments/:attachmentID/content`，直接返回附件二进制内容。
- 备选：在消息查询接口内附带 data URL/base64。
- 取舍：文件流接口更节省响应体，也适合 PDF iframe 预览。

2) **预览范围限制为图片 + PDF**
- 方案：图片使用 `<img>`，PDF 使用 `<iframe>`，文本类先不做内容预览。
- 备选：文本类也在 modal 中读取并展示。
- 取舍：先覆盖最直接的多模态场景，避免引入大文本渲染/编码问题。

3) **拖拽区域直接覆盖 composer 外层容器**
- 方案：在发送框外层增加 drag enter/leave/over/drop 事件和可见高亮态。
- 备选：单独新增一个显式 dropzone 区块。
- 取舍：覆盖 composer 更自然，且不会增加额外页面高度。

## Risks / Trade-offs

- **[浏览器预览大 PDF 可能较重]** → 仅做 iframe 嵌入，不做预渲染缩略图。
- **[拖拽事件易受子节点冒泡影响]** → 使用 drag counter 或统一外层事件处理，避免闪烁。
- **[附件内容接口可能被误用于下载]** → v1 接受该能力，响应头默认 `inline`，路径仍受 session/attachment 约束。

## Migration Plan

- 无 schema 变更。
- 新接口向后兼容，旧前端不受影响。
- 完成后同步更新 `PROJECT_STRUCTURE.md` 与基线 specs，并归档 change。

## Open Questions

- 是否后续要为文本附件加只读预览页；本次不做。
