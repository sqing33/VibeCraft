## Context

当前 `vibe-tree` 的设置链路将 OpenAI / Anthropic 的 `sources + models` 放在 `llm` 下，再让 CLI 工具通过协议族过滤这份共享模型池。这个设计对 `codex` / `claude` / `opencode` 来说已经过于抽象：UI 看起来像“统一配置”，但运行时仍依赖各 CLI 的原生配置语义；对 `iflow` 来说则完全是另一套逻辑。结果是：

- 设置页职责不清，用户不知道哪个页面的配置会生效。
- CLI 与 SDK 共享一份模型池，导致聊天页的 runtime/model 选择不等于真实执行能力。
- 要避免污染项目目录和用户全局配置时，现有实现没有为 `codex` / `claude` 提供统一的受管配置根目录方案。

这次改造需要同时覆盖配置结构、设置 API、聊天运行时解析、CLI wrapper 以及 UI 交互，因此属于跨模块架构性重构。

## Goals / Non-Goals

**Goals:**
- 将“来源管理”“runtime 模型管理”“CLI 工具管理”拆成三个职责单元。
- 让聊天页和其他 runtime 选择面严格使用 runtime-model 设置，而不是从共享模型池推断。
- 为 4 个 CLI runtime 提供不污染项目目录、也不直接修改用户全局默认配置的受管配置/启动参数方案。
- 对旧 `llm` / `cli_tools` 数据做自动迁移，避免用户手工重配。
- 保留 SDK helper、thinking translation、expert 镜像等现有能力的可用性。

**Non-Goals:**
- 不在本次改造中实现从外部 CLI 自动发现所有可用模型。
- 不引入新的 provider 类型或新的 CLI 工具家族。
- 不重构工作流 / orchestration 的高层执行策略。

## Decisions

### 1. 新增 `api_sources` 与 `runtime_model_settings`，并将其作为设置主真相源

保留旧 `llm` / `cli_tools` 字段作为兼容镜像，但新设置页与运行时解析只读取：

- `api_sources[]`: 统一来源定义，支持 `openai`、`anthropic`、`iflow`
- `runtime_model_settings.runtimes[]`: 6 个 runtime 的模型列表、默认模型与模型到来源的绑定

这样可以让“来源”和“模型池”解耦，也可以用一个统一结构表示 SDK 和 CLI 的模型选择。

**Why this over extending `llm.models` again?**

继续把 CLI 模型混在 `llm.models` 里，会让 helper SDK 与 CLI runtime 共享同一套验证和语义，无法表达 `iflow` 官方认证来源，也无法表达“同名模型在不同 runtime 下绑定不同来源”的需求。

### 2. 使用 runtime-scoped model bindings，而不是继续按 provider 过滤全局模型池

每个 runtime 拥有独立模型列表，模型项至少包含：`id`、`label`、`provider`、`model`、`source_id`。聊天页、repo analysis、CLI 默认模型都直接基于 runtime id 查询。

**Why this over provider filtering?**

当前 provider 过滤只能表达“协议兼容”，不能表达“这个 runtime 允许哪些模型”“同一 runtime 下多个来源如何并存”“某个 CLI 的默认模型是什么”。runtime 自有模型列表更贴近实际执行语义。

### 3. CLI 采用“受管配置根目录/文件 + 启动参数”混合物化策略

受管配置文件统一落在应用数据目录下，例如 `$XDG_DATA_HOME/vibe-tree/managed-clis/...`，而不是项目目录或用户默认全局路径。

- `codex`: 使用 `CODEX_HOME=<managed-root>` 指向受管 `config.toml`
- `claude`: 使用 `--settings <managed-settings.json>` 指向受管设置文件
- `iflow`: 继续使用受管 HOME，并从新来源绑定生成 `settings.json` / 环境变量
- `opencode`: 继续使用受管 `XDG_CONFIG_HOME`

**Why this over direct global mutation?**

它满足“设置可持久化生效”与“不能污染用户默认全局配置”的双重约束，也不会把 CLI 私有配置写进项目目录。

### 4. 保留旧 LLM / CLI 工具接口的兼容镜像，用于过渡现有 helper 逻辑

`thinking translation`、expert builder、部分既有测试仍依赖 `cfg.LLM` 或 `cli_tools.default_model_id`。本次重构通过“加载后导出兼容镜像”的方式，先让这些链路继续工作，再由新 API / 新 UI 驱动主流程。

**Why this over一次性删除旧字段?**

可以在不打断现有 helper 功能的前提下完成主链路切换，降低回归风险。

## Risks / Trade-offs

- **[Codex 受管配置覆盖不完整]** → 通过受管 `config.toml` + 当前调用参数双保险，并增加运行时测试覆盖。
- **[旧配置迁移后出现默认模型缺失]** → 在加载阶段为 6 个 runtime 自动补齐默认 runtime 与兜底默认模型。
- **[同名模型跨 runtime 冲突]** → 使用 runtime-scoped model id，并在保存时校验全局唯一性。
- **[iFlow browser auth 与多来源模型绑定存在歧义]** → browser-auth 仍共享受管 iFlow HOME，但来源配置明确声明 `auth_mode`，运行时按模型绑定来源选择 browser/api_key 分支。
- **[UI 一次性变化较大]** → 优先保持聊天页与设置页数据契约清晰，旧接口仅作兼容，不再继续扩展旧页面语义。

## Migration Plan

1. 新增配置结构与规范化/迁移函数，加载旧配置时自动生成 `api_sources` 与 `runtime_model_settings`。
2. 新增 API：`/api/v1/settings/api-sources` 与 `/api/v1/settings/runtime-models`。
3. 将 CLI wrapper 与 expert/runtime 解析切到新模型绑定结构。
4. 将设置页和聊天页切到新 API。
5. 保留旧 `llm` / `cli_tools` 兼容镜像，直到新路径稳定。
6. 完成测试后归档 change，并在后续 change 中再考虑是否删除旧兼容字段。

## Open Questions

- 本次不做自动模型发现；后续如需增强，可基于各 CLI 的官方列表命令或 discovery API 增量补充。
