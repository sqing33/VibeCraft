## Why

当前附件预览已经支持图片和 PDF，但文本/代码附件仍然只能看到文件名，无法直接阅读内容。为了让聊天附件在代码审阅、配置检查和文档阅读场景下真正可用，需要补上文本/代码附件的富预览能力。

## What Changes

- 为文本类附件增加内容预览能力。
- Markdown 附件在预览弹窗中按 Markdown 渲染。
- 代码与配置类附件在预览弹窗中启用语法高亮与行号。
- 引入前端高亮库，并根据附件后缀推断语言。

## Capabilities

### New Capabilities

### Modified Capabilities
- `chat-attachments`: 附件预览从图片/PDF 扩展到文本、Markdown 和代码文件。
- `ui`: 聊天页附件预览弹窗增加 Markdown 渲染与代码语法高亮。

## Impact

- 前端：`ChatSessionsPage`、新增附件预览组件/工具、`ui/package.json` 新依赖。
- 规范：更新 `chat-attachments` 与 `ui` 基线规范，并归档本次 change。
