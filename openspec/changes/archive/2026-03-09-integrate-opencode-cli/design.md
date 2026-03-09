## Context

当前仓库的 CLI runtime 已经形成了较稳定的三层抽象：

1. `cli_tools`：声明工具 id、协议族、CLI family、默认模型与命令路径。
2. `expert.ResolveWithOptions()`：把 chat / workflow / orchestration / repo-library 请求解析成最终 `RunSpec`。
3. `scripts/agent-runtimes/*.sh`：把统一环境变量翻译成具体 CLI 调用，并落 `summary.json` / `artifacts.json` / `session.json` / `final_message.md` / `patch.diff`。

`iflow` 先前已经证明了“新增 CLI family + wrapper + chat parser”这条增量接入路径可行，但 `opencode` 与 `iflow` 的关键差异在于：

- `opencode` 的模型选择格式是 `provider/model`，天然跨多 provider。
- `opencode` 可以同时消费 OpenAI / Anthropic source 连接配置。
- 当前 UI 与后端配置都把 CLI tool 视为“单协议工具”，这会直接限制 `opencode` 的模型池展示与兼容性校验。

因此本次实现不能只是复制一个 shell wrapper，还需要把 CLI tool 抽象升级到“支持多协议，但兼容旧字段”。

## Goals / Non-Goals

**Goals:**
- 把 `OpenCode CLI` 作为新的 primary CLI tool 接入 Settings / Chat / Repo Library / workflow-orchestration 默认 CLI runtime 链路。
- 让 CLI tool 设置与前端模型筛选支持多协议工具，同时对现有单协议工具保持零配置兼容。
- 为 OpenCode wrapper 提供标准 artifact contract、native session resume、best-effort JSON streaming。
- 继续让 `reasoning_effort` 仅服务 `Codex CLI`，避免把 Codex-only 语义扩散到其他工具。
- 保持现有 Codex app-server warm runtime、Claude / IFLOW wrapper 路径不回归。

**Non-Goals:**
- 不在本次把 `opencode serve` 接成新的 daemon-side persistent runtime；先走 `run` wrapper 路径。
- 不把 `opencode` 设为系统默认 CLI tool。
- 不在本次实现 Codex 级别的 permission / agent / MCP rich event UI；Phase 1 只保证核心开发链与 best-effort 流式体验。
- 不新增数据库 schema；继续复用现有 `cli_tool_id` / `cli_session_id` / `model_id` / MCP 选择字段。

## Decisions

### 1. CLI tool 配置新增 `protocol_families`，保留 `protocol_family` 兼容旧 API
- **方案**：`CLIToolConfig` / settings API / UI type 新增 `protocol_families?: string[]`，现有 `protocol_family` 保留为兼容与展示字段；单协议工具自动回填为单元素数组。
- **原因**：这样能最小代价支持 `opencode` 的多 provider 模型池，同时不破坏现有持久化文件与前端调用方。
- **备选**：把 `protocol_family` 直接改成数组并移除旧字段。
- **不选原因**：会造成现有配置、API 测试与 UI 大面积破坏性改动。

### 2. OpenCode 继续走 shell wrapper，而不是本次直接接 `serve`
- **方案**：新增 `scripts/agent-runtimes/opencode_exec.sh`，调用 `opencode run`，使用 `--format json` 收集事件、`--session` 续跑、`--model provider/model` 选模，并自行落标准 artifact。
- **原因**：这能最快复用现有 `RunSpec -> PTYRunner -> artifact` 链路，并覆盖 chat / workflow / orchestration / repo-library。
- **备选**：新增 daemon-side `opencode` client，直接接 `serve`。
- **不选原因**：实现量显著更大，会引入新的常驻服务生命周期与 richer event adapter；适合作为下一阶段。

### 3. OpenCode 的 provider/baseURL/apiKey 通过临时 XDG config 注入
- **方案**：后端继续把所选 `model_id` 的 source 连接信息注入统一 env，wrapper 在需要时基于这些 env 生成临时 `XDG_CONFIG_HOME/opencode/opencode.json`，仅覆盖目标 provider 的 `options.apiKey/baseURL` 与当前 model。
- **原因**：本机 `opencode` 配置结构已经证明 provider 配置使用 `provider.<name>.options.{apiKey,baseURL}`；生成临时 config 比猜测私有 env 名更稳，并且支持 OpenAI / Anthropic 两类 provider。
- **备选**：仅依赖 `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` 等环境变量。
- **不选原因**：官方公开 help 没有承诺这些 env 就是稳定配置面，直接写临时 config 更可控。

### 4. OpenCode 选模在 wrapper 层统一规范成 `provider/model`
- **方案**：后端为 CLI spec 额外透出解析后的 provider family；wrapper 若收到的 `VIBE_TREE_MODEL` 不含 `/`，则按 `provider/model` 规范拼接后传给 `opencode run --model`。
- **原因**：可兼容 builtin expert 的显式 `provider/model` 与 UI 通过 `model_id` 选择出的普通 `model` 字符串两种来源。
- **备选**：要求所有 LLM model 配置都改成 `provider/model`。
- **不选原因**：会破坏现有 LLM 设置与 SDK helper 路径。

### 5. Streaming 采用“JSON 事件 best effort + artifact 覆盖最终结果”
- **方案**：chat manager 为 `opencode` 增加 JSON line parser，尽量映射 session、assistant delta、thinking/progress、final；turn 完成后仍以 wrapper 落盘的 `final_message.md` / `session.json` 为准。
- **原因**：这能兼顾用户的实时反馈与最终一致性。
- **备选**：完全不做 streaming，等待进程退出后统一返回。
- **不选原因**：体验明显落后于现有 CLI 工具。

## Risks / Trade-offs

- **[Risk] `opencode run --format json` 的事件 shape 随版本变化** → parser 采用宽松字段匹配，失败时仍以 artifact 作为真相源，避免 turn 整体失败。
- **[Risk] 临时 `XDG_CONFIG_HOME` 覆盖用户 opencode 配置的一部分行为** → wrapper 只在确有 source/baseURL/apiKey 注入需求时创建最小 config，并尽量合并现有 `~/.config/opencode/opencode.json` 的非敏感结构。
- **[Risk] 多协议工具改造波及前端多个模型过滤点** → 统一在 shared type 与 helper 层增加“兼容 provider 列表”函数，避免页面各自实现。
- **[Risk] OpenCode rich permission/agent event 没有接入 UI** → 在 spec 与交付中明确 Phase 1 仅保证核心开发链，不承诺 Codex-level rich event parity。

## Migration Plan

1. 新建 OpenSpec proposal/design/tasks 与 delta specs。
2. 后端实现 `opencode` tool / expert / wrapper / multi-protocol config / parser / tests。
3. 前端实现 multi-protocol tool filtering 与 `OpenCode CLI` 展示。
4. 更新 `PROJECT_STRUCTURE.md`。
5. 运行 targeted tests/build。
6. 完成后执行 `/opsx:archive` 合并 delta specs。
