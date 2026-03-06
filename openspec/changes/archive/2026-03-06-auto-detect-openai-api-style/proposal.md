## Why

当前项目对 `provider=openai` 的调用统一走 `/v1/responses`，当用户接入只兼容 `/v1/chat/completions` 的 OpenAI-like 网关时，首次测试和真实运行都会直接失败。随着更多用户使用自建或第三方 AI 网关，需要让系统在不增加前端设置复杂度的前提下，自动判定并记住每个模型应使用的 OpenAI API 风格。

## What Changes

- 为 OpenAI 模型增加后端自动探测：首次测试或首次真实使用时，自动判定该模型应走 `responses` 还是 `chat/completions`。
- 将探测结果按模型隐藏持久化到 `llm.models[]`，不在前端模型设置 UI 中展示。
- 在 Chat、thinking translation、SDK runner、模型测试 API 中统一接入 OpenAI API 风格适配层。
- 对 `chat/completions` 无法支持的 `responses` 专属能力提供明确降级或报错策略：普通聊天优先可用，结构化输出等严格能力不静默降级。

## Capabilities

### New Capabilities
- `openai-api-style-routing`: 为 OpenAI 兼容网关提供按模型自动探测、隐藏持久化与运行时自动路由能力。

### Modified Capabilities
- `llm-settings`: LLM settings 需要在后端内部为 OpenAI 模型维护隐藏的 API 风格元数据，并在 source/model 变化时自动失效。
- `llm-test`: 模型测试 API 需要在 OpenAI 模型测试时自动探测可用接口风格，并在命中已保存模型时持久化探测结果。

## Impact

- 后端：`backend/internal/config/` 扩展 `LLMModelConfig` 隐藏字段与失效规则；新增 `backend/internal/openaicompat/` 统一探测/路由；`backend/internal/api/settings_llm_test_call.go`、`backend/internal/chat/manager.go`、`backend/internal/chat/thinking_translation.go`、`backend/internal/runner/sdk_runner.go` 接入自动切换。
- 前端：保持现有 UI 结构与返回结构，不新增设置项。
- 测试：新增 config/openaicompat/api/chat/runner 单测，覆盖探测、持久化、失效、回退和能力降级边界。
