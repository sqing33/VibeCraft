## Context

当前 vibe-tree 的 LLM 执行走后端 SDK（OpenAI/Anthropic），其 base URL 与 API Key 主要通过 `experts[].base_url` + `experts[].env`（`${ENV_VAR}` 模板）完成注入。虽然支持 `base_url` 覆盖，但对大多数用户而言，配置环境变量（或 `.env`）门槛较高，且无法在 UI 内完成配置/排障。

同时，用户希望配置结构更“可复用”：先维护一组 API 源（URL + Key），再在多个模型档案中复用这些源，并在档案里选择使用 Codex(openai) 或 ClaudeCode(anthropic) SDK。

## Goals / Non-Goals

**Goals:**

- 在 UI 的「系统设置」中新增「模型」Tab，提供 Sources 与 Model Profiles 的可视化编辑与保存。
- daemon 提供安全的读写 API：
  - 读取配置时不返回明文 key，仅返回 `has_key` + `masked_key`（例如只显示尾部 4 位）。
  - 保存时写入 `~/.config/vibe-tree/config.json`，并确保写入原子性与权限（0600）。
- 保存后运行时生效：更新 in-memory expert registry，使后续 workflow/node 执行使用最新 endpoint/key，无需重启 daemon。
- 兼容旧方式：仍允许 experts 中保留 `${ENV_VAR}` 注入；但当 LLM settings 被写入后，系统将把对应 experts 的 key 写为明文值（不再要求 env）。

**Non-Goals:**

- 不引入系统级密钥链（Keychain/Secret Service）或端到端加密存储（本次仅做最小安全措施：文件权限 + masking + API 不回传明文）。
- 不做“联网探测/连通性测试”按钮（避免额外的网络与错误处理复杂度）。
- 不实现复杂的多客户端并发编辑/冲突合并（以最后一次保存为准）。

## Decisions

1. **配置落盘位置与 schema**
   - 仍以 `~/.config/vibe-tree/config.json` 为唯一持久化入口（避免引入新的配置文件与迁移成本）。
   - 在 config 内新增 `llm` 字段（Sources + Models），作为 UI 的编辑态数据源。
   - 保存时同时“镜像”到 `experts[]`：
     - 每个 Model Profile 生成/覆盖同 ID 的 expert（provider/model/base_url/env/system_prompt/...）。
     - 这样运行时仍只依赖现有的 expert registry 机制执行，降低侵入性与回归风险。

2. **API 设计：整包读取/整包保存**
   - 提供 `GET /api/v1/settings/llm` 与 `PUT /api/v1/settings/llm`：
     - GET 返回完整 settings（sources/models），其中 source key 仅返回 `has_key` 与 `masked_key`。
     - PUT 接收 sources/models 的完整列表；source 的 `api_key` 使用可选字段：
       - 缺省（null/omitted）表示“不修改 key”；
       - 空字符串表示“清空 key”；
       - 非空字符串表示“更新 key”。
   - 选择整包接口而非细粒度 CRUD，是为了降低 UI 状态同步与后端并发控制复杂度。

3. **运行时生效：expert registry 热更新**
   - `expert.Registry` 增加内部读写锁与 `ReloadFromConfig(cfg)` 方法（或等价接口）：
     - PUT 成功写盘后，重新构建 experts map 并替换到 registry。
     - scheduler 在下一次 Resolve 时自然使用新配置。

4. **安全与显示策略**
   - daemon 日志与 API 响应中严禁输出明文 key。
   - 前端输入框使用 `type="password"`，并仅显示 masked（例如 `****abcd`）。
   - config 写盘使用临时文件 + rename（原子替换），权限设为 0600。

## Risks / Trade-offs

- [Risk] 明文 key 落盘存在泄漏风险 → Mitigation：仅本地单机使用；强制 config 文件权限 0600；API 只返回 masked；日志不打印 value。
- [Risk] 错误配置导致运行时失败（无 key/URL 非法） → Mitigation：后端保存前做校验（provider、ID 唯一、source 引用存在、URL 解析）；前端保存时做即时校验与错误提示。
- [Trade-off] “镜像到 experts[]”会产生重复表达（llm 与 experts） → Mitigation：这是换取低侵入与复用既有执行链路的最小实现；后续可再做“以 llm 为真相源、运行时动态生成 experts”的重构。
