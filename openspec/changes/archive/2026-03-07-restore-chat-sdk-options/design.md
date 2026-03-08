## Context

当前 Chat 页已经具备一套完整的 CLI-first 选择与恢复链路：

- UI 通过 `GET /api/v1/settings/cli-tools` 获取 `Codex CLI` / `Claude Code` 工具与模型池。
- 新建会话与消息发送都基于 `cli_tool_id + model_id` 发请求。
- 后端 chat API 在创建 session / 发 turn 时优先把 `cli_tool_id` 解析为 CLI expert。

与此同时，系统内部仍保留了 SDK 所需的数据源与执行能力：

- `GET /api/v1/experts` 仍会暴露 OpenAI / Anthropic 的 helper experts。
- `chat.Manager.RunTurn` 已经同时支持 CLI 路径和 SDK 路径。
- 当前真正阻断 SDK 直聊的点，主要是 Chat UI 不再提供 SDK 选项，以及 chat API 仍把 `helper_only` expert 一概判定为“不可 chat”。

## Goals / Non-Goals

**Goals**

- 在 Chat 页同一组选择器中同时提供 `Codex CLI`、`Claude Code`、`OpenAI SDK`、`Anthropic SDK`。
- 选择 SDK 时只展示对应 provider 的模型，并通过 SDK provider 直接对话。
- 会话切换、刷新历史、流式过程和消息身份展示继续正确工作。
- 后端仅放行“chat-capable SDK helper expert”，不误放开 `process` 或其他非 chat runtime。

**Non-Goals**

- 不改动 Settings 页的 CLI 工具管理逻辑。
- 不重新设计 expert 管理体系，也不新增 provider 配置入口。
- 不改变 CLI 会话续聊、CLI session metadata、thinking translation 等既有行为。

## Decisions

1) **用前端“混合运行时选项”承载 CLI + SDK，而不是新增后端 runtime 枚举**

- 方案：Chat 页本地把现有 `cliTools` 和可用模型 provider 组合成统一 option 列表。
- CLI 选项保留原来的 `tool/model` 组合；SDK 选项使用固定 key（如 `sdk:openai` / `sdk:anthropic`）。
- 好处：不改 API 结构，只改 UI 如何构造请求。

2) **SDK 选项按 provider 聚合，但真正发送时仍落到具体 model expert**

- 方案：当用户选中 `OpenAI SDK` 或 `Anthropic SDK` 时，模型下拉展示该 provider 的模型列表；发送时使用所选 `model_id` 作为 `expert_id`，并同时带上 `model_id`。
- 原因：运行时 expert registry 已经为每个 LLM model 生成对应 helper expert（ID 即模型 ID），这样无需再新增 provider 专用 chat expert。

3) **后端只对 `process` / 非 SDK helper 保持拒绝**

- 方案：Chat create-session / turn API 改为：如果 `resolved.Spec.SDK != nil`，则允许 helper-only expert 进入 chat；仍继续拒绝 `process` provider。
- 原因：`helper_only` 在这里代表“不能作为工作流节点主执行 expert”，但对于 Chat 的 SDK 直聊是安全且已被 manager 支持的。

4) **会话回填优先依据 session metadata 推断当前运行时类型**

- CLI 会话：优先使用 `cli_tool_id`，其次根据 `expert_id` / `provider` 推断 CLI 家族。
- SDK 会话：当 `cli_tool_id` 为空且 `provider` 为 `openai` / `anthropic` 时，回填到对应 SDK 选项。

## Risks / Trade-offs

- **运行时列表与实际模型池不一致**：SDK 选项仅在对应 provider 存在模型时显示，避免空列表选项。
- **helper expert 放开范围过大**：通过 `resolved.Spec.SDK != nil` 精确限制，避免误放 `process` / 非 chat helper。
- **旧会话回填异常**：保留现有 CLI 推断逻辑，并在 SDK 场景下增加 provider fallback。

## Validation Plan

- 补后端 integration test：使用 SDK helper model expert 创建 session 并成功发 turn。
- 运行 UI build / typecheck，确保混合运行时 selector 与请求结构通过编译。

