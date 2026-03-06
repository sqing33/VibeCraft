## Context

当前项目里的 OpenAI 调用是显式写死 `Responses.NewStreaming(...)`，不仅用于聊天主链路，也用于模型测试和思考过程翻译。这样能保留 `responses` 提供的 `previous_response_id` 和 reasoning summary 等能力，但会让只支持 `/v1/chat/completions` 的 OpenAI 兼容网关完全不可用。

用户希望系统在第一次使用时自动测试两种接口风格，把可用结果记录下来，后续直接复用，而且不在前端 UI 中展示这个技术细节。结合现有架构，最合适的做法是把结果隐藏持久化到 `config.json` 的 `llm.models[]` 中，而不是写进运行态 `state.db`。

## Goals / Non-Goals

**Goals**

- 仅针对 `provider=openai` 增加 `responses / chat_completions / auto-detect` 适配能力。
- 首次测试与首次真实使用时都可探测；测试时可预热，运行时可兜底。
- 将探测结果按模型持久化到 `llm.models[]`，但不暴露给前端 UI。
- 当 source/model/base_url/api_key 变化时，自动清空旧探测结果并重新探测。
- 普通聊天/文本生成功能优先可用；`responses` 专属能力缺失时做明确降级或报错。

**Non-Goals**

- 不覆盖 Anthropic 或其他非 OpenAI provider。
- 不新增前端设置控件或公开 API 字段来手动选择接口风格。
- 不把探测结果作为主要事实源写入 `state.db`。
- 不为 `chat/completions` 自动实现与 `responses` 等价的全部高级能力（尤其是严格结构化输出）。

## Decisions

1. **隐藏持久化字段放在 `llm.models[]`**
   - 在 `LLMModelConfig` 新增：
     - `openai_api_style`
     - `openai_api_style_detected_at`
   - `openai_api_style` 允许值：`"" | "responses" | "chat_completions"`
   - `GET /api/v1/settings/llm` 不返回这些字段；仅保存在 `config.json` 内部。

2. **建立统一 OpenAI 兼容适配层**
   - 新增 `backend/internal/openaicompat/`：
     - 探测：`responses -> chat/completions`
     - 错误分类：只在“端点不支持类错误”时回退
     - 调用：按 style 实际发起流式文本请求
   - 上层（chat/test/runner/translation）不再直接决定调用哪条 OpenAI 资源路径。

3. **探测顺序固定：优先 `responses`**
   - 因为它保留更多项目已有能力（anchor、reasoning summary、统一输入结构）。
   - 只有在 `responses` 明确不被网关支持时，才回退到 `chat/completions`。

4. **失效规则与保留规则**
   - 若 `provider/source_id/model` 变化，清空 `openai_api_style`。
   - 若 source 的 `base_url/api_key/provider` 变化，则该 source 下所有 openai 模型的 style 全部失效。
   - `PUT /settings/llm` 做整包保存时，需基于归一化后的 model id/source/model 关系保留或清空隐藏字段。

5. **能力降级策略：聊天可用优先**
   - 若某模型最终记录为 `chat_completions`：
     - 普通聊天/普通文本 runner 继续可用。
     - `previous_response_id` anchor 自动关闭。
     - OpenAI reasoning summary 与基于它的思考过程翻译输入自动退化为“无 reasoning 内容”。
     - 严格结构化输出（`OutputSchema != ""`）不做静默降级，直接报 `requires responses-compatible endpoint`。

6. **测试时预热，运行时兜底**
   - `POST /api/v1/settings/llm/test`：
     - 若能映射到已保存模型，则探测成功后写回 `config.json`。
     - 若只是未保存草稿，仅临时探测，不落盘。
   - 真实运行时若 style 为空，则自动探测并写回。
   - 若已有 style 但出现“端点不支持类错误”，则清空后重探测一次。

## Data Flow

1. 用户在 `模型` 设置页保存 Source/模型；`config.json` 中的 `llm.models[]` 初始无 `openai_api_style`。
2. 用户点“测试”或首次真实使用某 OpenAI 模型。
3. 系统读取模型配置：
   - 若已有 `openai_api_style`，先直接按该 style 调用；
   - 若没有，则先探测 `responses`，失败且属于端点不支持类错误时再探测 `chat/completions`。
4. 探测成功则将 style 回写到该模型条目；后续复用。
5. 若模型/source 被修改，隐藏 style 自动失效。
6. 运行时若检测到当前 style 已失效（例如网关更换），则清空并重探测一次。

## Risks / Trade-offs

- **错误分类误判**：如果把认证/模型错误误判为 endpoint mismatch，会错误回退。通过严格限制回退条件（404/405/501/明确错误文本）规避。
- **能力不对称**：`chat/completions` 无法等价替代 `responses` 的全部能力。通过“普通文本可用、结构化严格报错”的边界控制来避免静默错误。
- **整包保存与隐藏字段保留复杂度**：`PUT /settings/llm` 需要做旧配置比对，不能直接丢弃隐藏字段。

## Validation Plan

- Go 单测覆盖：
  - hidden field 保留/失效逻辑
  - endpoint mismatch 探测与回退
  - 测试 API 的探测写回/草稿不落盘
  - chat/runner 在 `chat_completions` 下的降级行为
- `go test ./backend/internal/config ./backend/internal/api ./backend/internal/chat ./backend/internal/runner`
- 若 UI 不变，只需确保 `cd ui && npm run build` 不受类型影响。
