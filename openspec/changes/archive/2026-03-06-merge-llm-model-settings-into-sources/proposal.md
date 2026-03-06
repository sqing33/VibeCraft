## Why

当前“模型”设置把 API 源和模型配置拆成两块编辑，用户需要在模型卡片里重复选择 SDK 与 Source，编辑路径长且容易配错。现在希望把模型直接挂到 API 源下维护，让一个 Source 只配置一次 SDK，并允许用户用自己习惯的大小写输入模型名，同时保证真正发给 SDK 的模型标识统一为小写。

## What Changes

- 将设置页里的独立“模型”区块并入每个 API 源卡片，在 API Key 下直接维护该 Source 的模型列表。
- 将 SDK 选择从模型级别上移到 Source 级别，放在 Base URL 后面，每个 Source 只配置一次。
- 模型编辑改为单字段录入模型 ID；保存与测试时自动转成小写，同时保留用户输入的显示大小写作为标签。
- 保持 `PUT /api/v1/settings/llm` 与专家列表热刷新流程，但补充后端的小写归一化，避免大小写输入导致请求不一致。

## Capabilities

### New Capabilities

- _None_

### Modified Capabilities

- `ui`: 模型设置页改为按 API 源分组维护模型，并在 Source 内完成 SDK、模型录入与测试。
- `llm-settings`: LLM settings 保存后需将模型 `id/model` 统一归一化为小写，同时保留显示标签。
- `llm-test`: 测试调用需对模型名做小写归一化后再交给 SDK。

## Impact

- 前端：`ui/src/app/components/LLMSettingsTab.tsx` 的状态结构与交互布局会重构。
- 后端：`backend/internal/config/llm_settings.go`、`backend/internal/config/llm_mirror.go`、`backend/internal/api/settings_llm_test_call.go` 需要补充归一化逻辑。
- 测试：需要增加设置保存与大小写归一化的回归用例。
