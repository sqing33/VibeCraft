## Context

聊天附件预览已经有“图片/PDF + 后端内容接口”的基础设施，因此文本/代码富预览可以完全复用现有内容接口。仓库里已使用 `react-markdown` 渲染聊天回复，新增 Markdown 附件渲染时可复用该能力；语法高亮则通过成熟组件库实现，避免自写高亮逻辑。

## Goals / Non-Goals

**Goals:**
- 文本、Markdown、代码附件都能在预览弹窗中直接查看内容。
- 代码/配置类附件有语法高亮和行号。
- Markdown 附件支持 GFM 渲染，并对其代码块继续高亮。

**Non-Goals:**
- 不做编辑能力。
- 不做全文搜索、复制按钮、diff 视图。
- 不做服务端内容转换或文件摘要。

## Decisions

1) 使用 `react-markdown` 继续负责 Markdown 渲染，保持和聊天回复一致。
2) 新增 `react-syntax-highlighter` 负责代码高亮，按文件后缀推断语言。
3) 文本/代码内容仍通过现有附件内容接口读取，不新增新 API。

## Risks / Trade-offs

- **[前端包体增大]** → 先接受新增高亮依赖；后续如有需要再做懒加载或 light build 优化。
- **[语言推断不完全准确]** → 先覆盖常见代码/配置后缀，无法识别时回退为纯文本展示。

## Migration Plan

- 无后端 schema 变更。
- 新增前端依赖后重跑 build/lint。
- 同步基线 specs 与 `PROJECT_STRUCTURE.md`，完成后归档 change。

## Open Questions

- 未来是否需要按主题切换不同高亮主题；本次先跟随现有 light/dark store 做切换。
