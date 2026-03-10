## Why

当前“模型设置”页同时暴露协议族、实际模型名、运行时级默认模型下拉等技术字段，但现有用户配置里这些值大多可以从 `模型 ID` 和 `API 来源` 推导出来。对于同时维护多种 CLI / SDK runtime 的场景，这种重复输入让配置过程冗长、难以扫描，也削弱了“按模型卡片直接操作”的直觉。

## What Changes

- 简化“模型设置”页，只保留用户直接关心的三个字段：`模型`、`显示名称`、`API 来源`。
- 将每个 runtime 的模型列表改为响应式卡片网格，并在卡片头部直接提供 `设为默认`、`测试`、`删除` 操作。
- 移除运行时级 `默认模型` 下拉、协议族编辑项、独立“实际模型名”编辑项。
- 让运行时模型保存接口兼容简化后的编辑器：当请求省略高级字段时，由后端根据 `API 来源` 推导 `provider`，并将 `model` 回填为 `模型 ID`。

## Capabilities

### New Capabilities
- 无

### Modified Capabilities
- `runtime-model-settings`: 运行时模型设置需要支持更简化的模型编辑输入，并继续保持 runtime 级默认模型语义。
- `ui`: 设置页中的“模型设置”需要改为卡片化编辑体验，并将默认模型切换下沉到单个模型卡片。

## Impact

- 前端：`ui/src/app/components/RuntimeModelSettingsTab.tsx` 的布局、交互与保存逻辑。
- 后端：`backend/internal/api/settings_runtime_models.go` 与 `backend/internal/config/runtime_settings.go` 的保存兼容逻辑。
- 验证：补充运行时模型规范化测试，并执行前端构建与后端测试。
