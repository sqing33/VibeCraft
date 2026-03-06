## Why

部分模型返回的思考过程是英文，当前 Chat UI 会直接展示原文，影响中文用户阅读与排障效率。随着系统设置已经承载模型与专家管理，现在需要在同一入口补上一个最小可用的“思考过程翻译”能力，并允许用户只对指定模型启用。

## What Changes

- 在系统设置中新增位于最左侧的 `基本设置` 标签页，先只承载“思考过程翻译”配置。
- daemon 新增 `GET/PUT /api/v1/settings/basic`，用于持久化翻译 Source、翻译模型，以及“需要翻译的 AI 模型”名单。
- Chat 运行时为命中的模型增加 reasoning 翻译链路：按缓冲分段调用翻译模型，将思考过程翻成中文后再展示。
- 前端为 reasoning 增加“翻译中 / 翻译完成 / 翻译失败回退原文”的状态处理，成功时只展示中文，失败时回退显示英文原文。
- 保存 LLM settings 时自动裁剪或清空失效的翻译配置，避免基础设置引用已删除的 Source/模型。

## Capabilities

### New Capabilities
- `thinking-translation`: 提供思考过程翻译配置、运行时翻译策略，以及聊天展示与失败回退行为。

### Modified Capabilities
- `ui`: 系统设置标签页从三个扩展为四个，并新增 `基本设置` 入口与 reasoning 中文展示行为。
- `chat-session-memory`: Chat turn 流式事件增加 reasoning 翻译事件，并在完成事件中补充翻译结果与状态字段。

## Impact

- 后端：`backend/internal/config/` 扩展基础设置 schema 与校验；`backend/internal/api/` 新增 basic settings API；`backend/internal/chat/` 增加 reasoning 翻译缓冲与事件广播；`backend/internal/expert/` 扩展 resolved metadata 以命中 model id。
- 前端：`ui/src/app/components/SettingsDialog.tsx` 增加 `基本设置` Tab；新增 `BasicSettingsTab`；`ui/src/lib/daemon.ts` 扩展 basic settings 接口；`ui/src/stores/chatStore.ts` 与 `ui/src/app/pages/ChatSessionsPage.tsx` 增加 reasoning 翻译状态与渲染逻辑。
- OpenSpec：新增 `thinking-translation` capability；更新 `ui` 与 `chat-session-memory` delta specs。
- 测试：新增 config/api/chat 相关单测，覆盖配置校验、LLM 设置联动清理、翻译触发与失败回退。
