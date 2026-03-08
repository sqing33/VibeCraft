## Context

当前 Chat 页已经在同一会话内支持运行时/模型切换，也已经为 Codex CLI 打通了 app-server 的 thread start / resume 机制与 MCP runtime config 注入。现状仍有两个缺口：

1. Codex 虽然底层支持 reasoning effort，但 UI / API / store / app-server 之间没有参数链路。
2. 输入区仍是偏高的纵向堆叠布局，右侧控制列宽度过大，左侧输入框没有完全承担主编辑区域。

本次设计在不改变整体会话模型的前提下，把 reasoning effort 视为“与运行时/模型同级的 session 默认值 + 本条 turn 覆盖值”，并同步压缩输入区布局。

## Goals / Non-Goals

**Goals**
- 为 Codex CLI turn 提供 low / medium / high / xhigh 四档思考程度选择。
- 将 `reasoning_effort` 存入 `chat_sessions`，并在成功 turn 后更新为 last-used 默认值。
- Codex thread 配置使用 `model_reasoning_effort`，turn 请求使用 `effort`，覆盖当前消息。
- 输入区右侧控制栏缩窄约三分之一，并调整为三行：运行时、模型、思考程度+上传/发送。
- 非 Codex 运行时下仍保留思考程度控件但禁用，以保证版面稳定。

**Non-Goals**
- 不为非 Codex 运行时引入新的 reasoning 参数语义。
- 不新增单独的“保存默认思考程度”按钮；仍沿用发送成功后更新 session 默认值的策略。
- 不在左侧“新建会话”卡片中新增独立思考程度 UI，仅为 Codex 会话使用默认 `medium` 初始化。

## Decisions

1. **Session 持久化字段使用 nullable `reasoning_effort`**
   - 这样可以兼容既有数据，也能让未来从 session 直接恢复上次使用的 Codex 配置。

2. **采用“线程默认 + 本条覆盖”的双层传递策略**
   - `thread/start` / `thread/resume` 的 config 中注入 `model_reasoning_effort`。
   - `turn/start` 中再传 `effort`，确保本条消息的选择优先生效。

3. **非 Codex 运行时保留但禁用控件**
   - 这样不会在切换运行时时导致按钮位置跳动，也明确表达这是 Codex 专属能力。

4. **右侧控制栏收窄到约 156px**
   - 该宽度相较原有约 220px 明显收窄，同时仍能容纳一个紧凑下拉框加两个圆形按钮。

5. **左侧输入框由右侧控制栏决定高度**
   - 输入框容器改为拉伸填满整个 composer 高度，整体高度由右侧两组选择器和底部控制行自然决定。

## Risks / Trade-offs

- `reasoning_effort` 属于 Codex 专属参数，若未来其它 CLI 支持类似能力，需要再抽象为通用 runtime option。
- 右侧控制栏过窄会压缩下拉框可读性，因此选项文案保持简短英文值。
- 如果外部 API 写入了 UI 不支持的 effort 值（例如 `minimal`），前端会回退显示为 `medium`，但后端仍允许兼容 Codex schema 的完整枚举。

## Migration Plan

1. SQLite schema 升级到 v11，为 `chat_sessions` 新增 `reasoning_effort`。
2. Store 的 session 读写、fork、默认值更新链路接入新字段。
3. API 与 chat manager 把 `reasoning_effort` 贯穿到 Codex app-server。
4. 前端在 Chat 输入区新增思考程度选择器，并缩窄右侧控制栏。

## Verification

- 后端：store 迁移 / session 默认值 / Codex runtime settings 测试通过。
- 前端：`ui` 构建通过，确认类型与布局无回归。
- 手工：在 `#/chat` 中验证 Codex 运行时可选 effort，非 Codex 显示为禁用。
