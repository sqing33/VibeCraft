## Why

当前 Chat 对话页在加载历史或对话变长时，会把大量历史内容一次性渲染到 DOM 中（包含长文本/Markdown），导致页面严重卡顿甚至卡死。该问题在导入历史（例如 Codex history）或长时间运行的会话中尤为明显，直接影响可用性与稳定性。

## What Changes

- 前端 Chat 消息列表改为虚拟列表渲染：仅渲染视口附近的消息，其余消息不挂载到 DOM，避免超长对话导致 UI 卡死。
- 支持“无限上拉加载更早消息”：用户滚动到顶部时按需加载更早消息并 prepend，保持滚动位置稳定。
- 保持现有体验：在底部时自动跟随流式输出；用户上滑后不强制回到底部，并提供“回到底部”快捷入口。
- 后端 `GET /api/v1/chat/sessions/:id/messages` 增加分页游标参数（`before_turn`），支持按 turn 拉取更早消息。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `chat-session-memory`: `GET /api/v1/chat/sessions/:id/messages` 增加 `before_turn` 分页能力，明确分页排序与返回语义。
- `chat-page-immersive-layout`: Transcript 区域在超长历史下必须保持可交互与流畅滚动，并支持向上滚动加载更早消息。

## Impact

- Backend:
  - API：`GET /api/v1/chat/sessions/:id/messages` 新增 query 参数 `before_turn`（向后兼容）。
  - Store：新增按 `turn < before_turn` 查询消息的路径，附件聚合逻辑复用现有实现。
- UI:
  - Chat transcript 改用虚拟列表（引入 `react-virtuoso` 依赖）。
  - ChatStore 的消息加载从“覆盖”调整为“合并/去重”，避免上拉加载的旧消息被刷新覆盖。
  - 长对话下的滚动/自动跟随逻辑需重做并回归验证。

