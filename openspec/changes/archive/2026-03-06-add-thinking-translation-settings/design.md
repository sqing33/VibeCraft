## Context

当前系统设置已经包含 `连接与诊断 / 模型 / 专家` 三个标签页，LLM Source 与模型列表会镜像为可执行 experts。Chat 链路中，provider 会把 reasoning 通过 `chat.turn.thinking.delta` 实时推给前端，并在 `chat.turn.completed` 中补一份整段 `reasoning_text`。这些 reasoning 目前不落库，只存在于单次会话的流式展示中。

用户希望通过独立的翻译模型，将部分 AI 模型产生的英文思考过程翻译成中文。由于翻译模型速度较快，但 reasoning delta 粒度很细，若直接按 token 调用会带来大量请求、顺序控制复杂度和额外成本，因此需要一个缓冲分段策略。

## Goals / Non-Goals

**Goals**

- 在系统设置中新增 `基本设置` 标签页，并持久化思考过程翻译的三项配置：`source_id`、`model`、`target_model_ids`。
- 对命中的模型，在 Chat turn 期间对 reasoning 执行缓冲分段翻译，并通过 WebSocket 推送中文增量。
- 成功时 UI 只展示中文 reasoning；翻译失败时立即回退显示英文原文。
- 复用现有 LLM Source（provider/base_url/api_key）作为翻译调用来源，不新增第二套密钥体系。
- 保存 LLM settings 后自动裁剪失效翻译配置，避免悬空引用。

**Non-Goals**

- 不翻译 assistant 正文输出。
- 不为历史消息或 reasoning 引入数据库持久化。
- 不支持多语言切换、多翻译器策略、按用户维度独立配置。
- 不实现 token 级别的逐词翻译或并发多段乱序翻译。

## Decisions

1. **配置模型**
   - 在 `config.Config` 顶层新增 `Basic *BasicSettings`，与 `LLM`、`Experts` 并列，避免职责混入 `llm` 或 `experts`。
   - `BasicSettings` 下新增 `ThinkingTranslation *ThinkingTranslationSettings`：
     - `source_id`: 复用 `llm.sources[].id`
     - `model`: 手动填写的翻译模型名
     - `target_model_ids`: 需要启用翻译的 `llm.models[].id` 列表
   - 仅当三项都非空时视为启用；否则视为未启用。

2. **API 设计**
   - 新增 `GET /api/v1/settings/basic` 与 `PUT /api/v1/settings/basic`，只负责读写 basic settings。
   - 前端继续通过 `GET /api/v1/settings/llm` 获取 Source 与模型候选项，不把 `settings/basic` 扩展成复合响应。

3. **目标模型命中方式**
   - 用户配置的是 `llm model id`，而 Chat 运行时选择的是 `expert_id`。
   - 为避免在 Chat 层重新猜测 expert/model 对应关系，扩展 `expert.Resolved`，附带 `ManagedSource` 与 `PrimaryModelID`。
   - 命中规则：仅当 `Resolved.PrimaryModelID` 命中 `target_model_ids` 时启用翻译。因此：
     - `llm settings` 镜像出来的 `llm-model` experts 可直接命中。
     - 通过 `primary_model_id` 引用模型的自定义 expert profile 也可命中。
     - 未绑定 `primary_model_id` 的 builtin experts（如 `codex`/`claudecode`）在 V1 不参与。

4. **翻译执行策略：缓冲分段、单飞行串行**
   - 在 `chat.Manager` 内为每个 `session_id + turn` 维护一个 reasoning translation state。
   - flush 触发条件：
     - 缓冲文本中出现句末标点/换行，且累计长度达到阈值；
     - 或累计长度达到强制分段阈值；
     - 或短时间静默（idle）后仍有未翻译缓冲；
     - 或 provider turn 完成时强制翻译剩余内容。
   - 同一 turn 任何时刻只允许一个翻译请求在途；新 delta 到来只继续累积缓冲，待前一请求结束后顺序处理，保证中文片段顺序稳定。

5. **事件与前端展示**
   - 保留现有 `chat.turn.thinking.delta` 与 `reasoning_text`，供回退英文原文与兼容旧逻辑。
   - 新增 `chat.turn.thinking.translation.delta` 与 `chat.turn.thinking.translation.failed`。
   - `chat.turn.completed` 增加：
     - `translated_reasoning_text`
     - `thinking_translation_applied`
     - `thinking_translation_failed`
   - 前端新增 translated thinking store。若启用翻译且未失败，则只展示中文；一旦失败，当前 turn 全量回退为原文显示。

6. **LLM settings 联动清理**
   - `PUT /api/v1/settings/llm` 保存成功前，自动修正 `cfg.Basic.ThinkingTranslation`：
     - 若 `source_id` 已不存在，清空整个翻译配置；
     - 若部分 `target_model_ids` 已不存在，裁剪为仍存在的子集；
     - 若裁剪后无目标模型，也清空整个翻译配置。

## Data Flow

1. 用户在 `基本设置` 中选择翻译 Source、填写翻译模型，并勾选目标模型。
2. 前端调用 `PUT /api/v1/settings/basic`，daemon 校验并写入 `config.json`。
3. Chat turn 开始时，`expert.Resolve()` 返回 `PrimaryModelID`；`chat.Manager` 根据 `Basic.ThinkingTranslation` 判定是否启用 reasoning 翻译。
4. Provider 返回 reasoning delta 时：
   - 仍照常广播原始 `chat.turn.thinking.delta`；
   - 同时将 delta 写入翻译缓冲器。
5. 缓冲器按分段条件触发翻译请求，调用由 `source_id` + `model` 决定的翻译模型，并广播 `chat.turn.thinking.translation.delta`。
6. 若翻译失败，广播 `chat.turn.thinking.translation.failed`，前端切回显示英文原文。
7. Turn 完成时，对剩余缓冲做强制 flush，并在 `chat.turn.completed` 中补充最终翻译结果与状态。

## Risks / Trade-offs

- **请求时序复杂度增加**：缓冲分段比整段翻译更复杂。通过“单飞行串行 + turn 完成强制 flush”约束顺序与收尾。
- **翻译延迟可能落后于原 reasoning**：前端在启用翻译时先显示“正在翻译思考过程…”，避免一边显示英文一边再切中文造成闪动。
- **builtin expert 无 `primary_model_id` 无法命中**：这是 V1 明确的范围边界，避免把 model/provider 文本匹配写死在 chat 层。
- **翻译模型调用失败**：通过英文原文回退兜底，保证 reasoning 至少可读。

## Validation Plan

- Go 单测覆盖 basic settings 校验、LLM settings 联动裁剪、chat translation buffer 触发与失败回退。
- `go test ./backend/internal/config ./backend/internal/api ./backend/internal/chat`
- `cd ui && npm run build`
- 手工冒烟：配置 1 个翻译 Source、1 个翻译模型、2 个目标模型，对命中/未命中模型分别发起聊天，验证中文展示与英文回退。
