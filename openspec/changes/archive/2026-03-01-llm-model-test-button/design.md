## Context

vibe-tree 已有 LLM settings（Sources/Models）与对应的 `GET/PUT /api/v1/settings/llm`，并在保存后将 Models 镜像为 Experts，从而可在 workflow/node 下拉中直接选择。

但目前缺少“快速验证配置是否可用”的能力。用户希望在 Models 区域每个模型卡片上直接点“测试”，用当前填写的 provider/model/base_url/key 做一次短生成，以便立刻发现 URL/Key/模型名是否正确。

## Goals / Non-Goals

**Goals:**

- 提供 `POST /api/v1/settings/llm/test`：
  - 支持 `provider=openai|anthropic`
  - 支持传入 `model`、`base_url`（可空）、`api_key`（必填）
  - 使用固定短 prompt（或允许覆盖），短超时（例如 10s），限制输出长度与 token
  - 返回 `{ ok, output, latency_ms }` 或 `{ error }`
- UI 在每个 Model Profile 卡片的删除按钮左侧增加“测试”按钮：
  - 使用当前草稿的 provider/model/source/base_url/key 组装请求
  - 测试中按钮 loading；结束后 toast 提示结果

**Non-Goals:**

- 不把测试结果持久化、不写入 execution 日志文件
- 不回传明文 key，不在后端日志中打印 key
- 不实现复杂的流式输出（一次性短输出即可）

## Decisions

1. **测试 API 直接走 SDK，而不是复用 execution**
   - execution 会写日志与持久化 execution 记录，不符合“轻量测试”的预期；
   - 测试仅需返回短文本与错误信息。

2. **请求 payload 允许使用未保存草稿**
   - 由 UI 直接把当前卡片的 provider/model/base_url/api_key 发给后端；
   - 避免要求用户先保存再测试。

3. **固定测试 prompt + 输出截断**
   - 默认 prompt：`Reply with a single word: OK`（避免输出过长）。
   - 后端对输出做最大长度截断（例如 200 字符），避免响应过大。

## Risks / Trade-offs

- [Risk] 测试请求可能误用真实计费模型 → Mitigation：max_tokens 很小、prompt 很短；UI 文案提示“会产生少量调用”。
- [Risk] key 泄漏 → Mitigation：API 响应不包含 key；后端禁止日志打印 key；仅从 request body 使用，不落盘。

