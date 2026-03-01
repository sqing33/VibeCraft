## Why

目前用户在「模型」设置页填完模型档案（SDK/Source/Model/Base URL/Key）后，只有真正跑 workflow 才能发现配置是否可用，反馈周期长且排障困难。需要一个“就地测试”能力，用最小成本验证该模型档案是否能通过 SDK 成功生成输出。

## What Changes

- 在「模型」设置页的每个 Model Profile 卡片上新增“测试”按钮（位于删除按钮左侧）：
  - 使用当前表单填写的配置（含未保存的草稿）发起一次简短 SDK 请求。
  - 结果以 toast 展示（成功/失败 + 简短信息），不写入日志，不泄漏 API key。
- daemon 新增模型档案测试 API：
  - 接收一次性测试参数（provider/model/base_url/api_key/prompt 可选）。
  - 调用 OpenAI/Anthropic SDK 做一次短输出（小 token、短超时、固定测试 prompt）。
  - 返回是否成功与简短输出/错误信息（输出截断）。

## Capabilities

### New Capabilities

- `llm-test`: 提供一次性 LLM 模型连通性测试 API（不持久化、不泄漏 key），供 UI 调用。

### Modified Capabilities

- `ui`: 在系统设置「模型」Tab 的 Models 区域为每个模型卡片增加“测试”按钮，并展示测试结果。

## Impact

- 后端：新增 `POST /api/v1/settings/llm/test` handler，复用现有 SDK runner；增加必要的输入校验与超时/截断策略。
- 前端：`ui/src/app/components/LLMSettingsTab.tsx` 增加测试按钮与状态；`ui/src/lib/daemon.ts` 增加 test API 调用。
