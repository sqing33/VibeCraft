## Why

当前 `API 来源` 把 `provider/来源类型` 绑定在来源卡片本身，默认假设“一条来源只对应一种协议”。这和真实网关/中转站的使用方式不一致：同一个来源往往只是一组连接信息，可能同时被 OpenAI、Anthropic、iFlow 或不同 runtime 复用。

继续保留来源级 provider 会让设置逻辑变得别扭：用户需要为了同一组连接信息重复建多张来源卡片，模型测试与 runtime 绑定也被迫依赖来源类型而不是模型自身的协议。

## What Changes

- **BREAKING** 移除 `API 来源` 设置中的来源级 `provider/来源类型` 字段，来源仅保存通用连接信息：`id`、`label`、`base_url`、`api_key`、可选 `auth_mode`。
- 调整 runtime model 归一化与校验逻辑：模型绑定的协议由 `model.provider` / runtime 决定，不再要求和来源卡片上的 provider 匹配。
- 调整设置 UI：`API 来源` 页移除“来源类型”，`模型设置` 页测试模型时改用模型绑定自身的 provider，而不是来源 provider。
- 调整 runtime → legacy LLM/basic translation 的兼容链路，使运行时分支读取 `model.provider`，来源仅提供连接参数。

## Capabilities

### New Capabilities

### Modified Capabilities
- `api-source-settings`: API 来源不再暴露或校验来源级 provider，来源只承载连接信息与可选 iFlow 认证元数据。
- `runtime-model-settings`: runtime 模型绑定不再从来源继承 provider，也不再校验来源 provider 兼容性；provider 由模型绑定 / runtime 决定。
- `ui`: API 来源设置页移除来源类型字段，模型测试和来源筛选逻辑不再依赖来源级 provider。

## Impact

- Backend: `backend/internal/config/runtime_settings.go`, `backend/internal/api/settings_api_sources.go`, `backend/internal/api/settings_runtime_models.go`, `backend/internal/api/settings_llm_test_call.go`, `backend/internal/config/basic_settings.go`, `backend/internal/api/translate.go`
- Frontend: `ui/src/lib/daemon.ts`, `ui/src/app/components/APISourceSettingsTab.tsx`, `ui/src/app/components/RuntimeModelSettingsTab.tsx`, `ui/src/app/components/BasicSettingsTab.tsx`
- Specs: `openspec/specs/api-source-settings/spec.md`, `openspec/specs/runtime-model-settings/spec.md`, `openspec/specs/ui/spec.md`
- API compatibility: `GET/PUT /api/v1/settings/api-sources` 响应与请求不再包含 `provider`
