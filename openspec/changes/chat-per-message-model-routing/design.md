## Context

`vibe-tree` 当前的 chat 形态（`#/chat`）是 SDK-first：会话与消息写入 SQLite；后端通过 OpenAI/Anthropic SDK 流式推送 `chat.turn.*` WebSocket 事件；并使用 provider anchor（OpenAI `previous_response_id`、Anthropic `container`）尽可能复用上下文，同时在 anchor 失效时回退到本地 summary + recent 的重建上下文路径。

现状限制：
- Session 固定绑定 `expert_id/provider/model`，无法在同一会话内按消息切换模型。
- 自动 compaction 采用本地规则摘要（截断拼接），对长期记忆与语义一致性不足。
- 消息记录不包含“本条使用的模型信息”，历史可追溯性不完整。

约束：
- 本次保持 SDK-first，不引入 CLI 会话类型；未来可在同一 UI 下增加 CLI 会话，但不作为本变更交付目标。
- 变更跨后端 store、api、chat manager 与前端页面/状态管理，且涉及 DB schema migration，需要先做设计对齐。

## Goals / Non-Goals

**Goals:**
- 支持“每条消息可选 Expert/模型”：turn API 接受可选 `expert_id`，并把 expert/provider/model 写入该条 user/assistant message。
- 安全的 anchor 策略：模型切换时不复用旧 anchor，强制走本地 summary + recent 重建上下文，避免跨模型/跨 provider 串线。
- compaction 升级为 LLM 总结：自动触发与手动触发都生成结构化中文 summary；总结失败时回退到本地规则摘要，保证 turn 不被 compaction 阻断。
- UI 提供“本条 Expert”选择，并在消息与流式状态中展示本条使用的 provider/model；刷新后保持一致。
- 通过 schema 兼容升级（新增列）实现平滑迁移，旧数据可读、新数据可写。

**Non-Goals:**
- 不实现 CLI-first/CLI 会话恢复/工具调用等能力（未来按需接入）。
- 不引入复杂的多 anchor 并存（例如同一 session 同时维护多条 provider/model anchor map）。
- 不做“工具级文件预览/Diff/审计”聚合（本次只为后续能力预留数据与事件基础）。

## Decisions

1) **SDK-first，坚持 Expert Registry 作为模型配置真相源**

- 方案：每条 turn 根据 `expert_id` 走 `deps.Experts.Resolve(...)` 得到 `SDKSpec + Env`，并以该 spec 驱动 OpenAI/Anthropic SDK 调用。
- 备选：在 chat session 内直接写 provider/model/base_url/api_key（绕过 expert registry）。
- 取舍：保持配置收敛在 experts/llm-settings，避免 chat 独立引入第二套配置与校验逻辑。

2) **消息级 metadata 持久化，而非仅会话级字段**

- 方案：为 `chat_messages` 增加 `expert_id/provider/model` 列；新写入消息均填充；旧消息允许为空。
- 备选：只在内存/前端记录模型信息。
- 取舍：持久化是“可追溯/可排障/可迁移”的前提，也为后续审计与回放能力铺路。

3) **Anchor 安全策略：仅在同 provider+model 连续使用时复用**

- 方案：当本次 turn 的 provider/model 与 session 当前默认 provider/model 一致时才读取并使用 `chat_anchors`；否则传空 anchor（强制走本地 summary + recent）。
- 备选：为每个 provider+model 维护独立 anchor map（按 key 持久化）。
- 取舍：先以正确性与实现复杂度为先；独立 anchor map 会引入更复杂的迁移与清理策略。

4) **Session 默认模型跟随 last-used**

- 方案：每次 turn 成功后，把 session 的 `expert_id/provider/model` 更新为本次使用的值；UI 默认下拉选中该值。
- 备选：session 永远固定创建时 expert，发送时显式传 expert_id。
- 取舍：跟随 last-used 可以减少用户频繁选择，同时仍允许逐条覆盖。

5) **LLM compaction：使用当前 turn 的 expert 作为 summarizer**

- 方案：compaction 触发时用本次 turn expert/provider/model 发起一次“总结调用”，生成结构化中文 summary；失败则回退到规则摘要。
- 备选：引入专用 summarizer expert（固定模型/更稳定），或在后台异步总结。
- 取舍：先不引入新配置与异步复杂度；后续可再演进为“专用 summarizer + 异步总结”。

## Risks / Trade-offs

- [DB migration 失败或旧数据不兼容] → 采用“新增列”的向前兼容迁移；老消息列为 NULL 时 UI/JSON 均降级显示。
- [LLM 总结增加成本与延迟] → 只在超过阈值或手动触发时执行；设置较小的输出 token 上限；失败回退到规则摘要避免阻断 turn。
- [Anchor 策略导致切换模型时上下文变短] → 切换时强制 summary + recent 重建，确保正确性；必要时鼓励用户 fork 会话做对比。
- [Expert 选择包含不适合 chat 的 expert] → UI 侧过滤明显非 chat 的专家（如 `provider=process`、规划型 master 等），并在后端校验必须是 SDK provider。

## Migration Plan

1) 新增 schema version（v3）：为 `chat_messages` 增加 `expert_id/provider/model` 列。
2) 后端 Store/Scan/Append 兼容 NULL。
3) 更新 chat turn API 支持 `expert_id`，并在写入消息时填充 metadata。
4) 更新 UI：发送时携带 `expert_id`，展示消息与流式状态的 provider/model。

回滚策略：
- 本次迁移为“新增列”，旧二进制在理论上仍可读写（但会忽略新列）；若需要回滚到旧版本，需接受“新列数据不再被使用”的降级表现。

## Open Questions

- UI 里默认隐藏哪些 expert（例如 `master` 是否应排除，或改为显式 allowlist）？
- LLM compaction 的提示词与摘要格式是否需要写入基线 spec（以保证长期一致性）？
