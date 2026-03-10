## Context

当前 runtime settings 已经把“模型池”和“来源池”拆开，但 `APISourceConfig` 仍保留 `Provider` 字段，导致来源既承担连接信息又承担协议语义。实际运行时里，真正决定 SDK/CLI 分支的是模型绑定的 `provider`；来源主要提供 `base_url`、`api_key` 与 iFlow 的认证元数据。

设置页也因此出现不一致：`API 来源` 强制用户选一个来源类型，而 `模型设置` 又把模型协议隐藏起来并尝试从来源反推。对大多数 runtime 这只是冗余，对支持多协议的 runtime（如 `opencode`）则形成隐式耦合。

## Goals / Non-Goals

**Goals:**
- 让 API 来源只表达连接信息，不再承担单一 provider 语义。
- 让 runtime model 绑定和测试逻辑改为以 `model.provider` 为准。
- 保持现有 runtime-first 模型池结构与 UI 简化方向不变。
- 尽量兼容已有配置，避免要求用户手动迁移已有 model/provider 数据。

**Non-Goals:**
- 不重做 legacy `LLM` 配置结构。
- 不新增一套复杂的“来源支持多协议列表”编辑器。
- 不为 `opencode` 增加新的显式协议选择 UI；新建模型仍沿用 runtime 默认 provider，已有 provider 则继续保留。

## Decisions

### 1. 从 `APISourceConfig` 和 API 来源设置接口中移除 `provider`
- 原因：来源只应表达连接参数；同一个来源可被多个 provider/runtime 复用。
- 替代方案：保留 `provider[]` 列表。未采用，因为当前用户明确要去掉来源类型，而且会扩大 UI/校验范围。

### 2. `model.provider` 成为唯一的运行协议来源
- 决策：runtime model 归一化时，若请求未传 `model.provider`，优先保留已有值；新建模型则回退到 runtime 默认 provider。
- 原因：CLI/SDK 真正执行、env 注入、测试调用都应跟着模型绑定走，而不是跟着来源走。

### 3. `API 来源` 页移除来源类型字段，但保留 `auth_mode`
- 决策：来源卡片继续允许配置 `auth_mode`，仅在被 iFlow runtime 使用时生效，其他 provider/runtime 忽略。
- 原因：这样可以保留 iFlow 的网页登录/API Key 切换能力，又不需要在来源级保留 provider。

### 4. 兼容链路改读模型 provider
- 决策：basic thinking translation、文本翻译、模型测试等后端逻辑改从模型绑定读取 provider，source 仅补全连接参数。
- 原因：否则去掉来源 provider 后，这些路径会直接失效。

## Risks / Trade-offs

- [同一来源被多协议复用时，legacy LLM sources 无法精确表达 provider] → 兼容链路尽量改读 `model.provider`，legacy source provider 仅作为保底兼容字段。
- [`opencode` 这类多协议 runtime 在 UI 中仍无显式 provider 切换] → 保留已有模型 provider，新增模型默认 runtime 主 provider；后续如有需要再单独补 UI。
- [API 来源接口是 breaking change] → 同步更新前端请求/响应类型与设置页表单。

## Migration Plan

1. 更新 OpenSpec 与前后端 DTO，先去掉 API 来源请求/响应中的 `provider`。
2. 修改 runtime model 归一化/校验与测试逻辑，确保运行路径不再依赖来源 provider。
3. 调整设置 UI：移除来源类型字段，模型测试改读模型 provider。
4. 运行前端 build 与后端测试，验证设置保存、模型测试、思考翻译链路。
