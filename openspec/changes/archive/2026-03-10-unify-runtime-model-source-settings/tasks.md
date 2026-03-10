## 1. 配置与迁移

- [x] 1.1 为配置模型新增 `api_sources` 与 `runtime_model_settings` 结构，并补齐规范化与校验逻辑
- [x] 1.2 实现从旧 `llm` / `cli_tools` 到新结构的加载时迁移与兼容镜像导出
- [x] 1.3 为新配置结构补充单元测试与持久化回归测试

## 2. 后端设置 API 与运行时解析

- [x] 2.1 新增 `GET/PUT /api/v1/settings/api-sources`，支持来源读写、掩码密钥与 iFlow 来源字段
- [x] 2.2 新增 `GET/PUT /api/v1/settings/runtime-models`，支持 6 个 runtime 的模型列表与默认模型读写
- [x] 2.3 重构聊天 / repo analysis / expert 解析链，改为按 runtime-model 绑定解析来源、模型与默认值

## 3. CLI 受管配置物化

- [x] 3.1 为 Codex 与 Claude 增加受管配置物化逻辑，并在 wrapper 中接入 `CODEX_HOME` / `--settings`
- [x] 3.2 将 iFlow 与 OpenCode wrapper 切换到新来源/模型绑定输入，同时保留现有 artifact contract
- [x] 3.3 为受管配置目录与 CLI 运行时新增覆盖测试或集成测试

## 4. 前端设置页与聊天页

- [x] 4.1 将原“模型”页改为 `API 来源` 页，只编辑来源信息
- [x] 4.2 新增 `模型设置` 页，统一配置 2 个 SDK 与 4 个 CLI runtime 的模型、默认模型与来源绑定
- [x] 4.3 调整 `CLI 工具` 页为工具级设置页，并更新聊天页 runtime/model 选择逻辑使用新 API

## 5. 验证、文档与归档

- [x] 5.1 更新相关测试、必要文档与 `PROJECT_STRUCTURE.md` 中的设置定位说明
- [x] 5.2 运行后端/UI 定向验证命令并修复回归
- [x] 5.3 完成实现后归档 OpenSpec change
