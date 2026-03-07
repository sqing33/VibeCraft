## Context

当前 `vibe-tree` 已经完成 CLI-first 的主执行路径，但“工具/模型/专家”抽象仍然错位：

- `Codex` / `ClaudeCode` 同时承担“CLI 工具名”和“expert 名”两种语义。
- `llm.models` 仍会被镜像成 `llm-model` experts，导致模型页的模型看起来像主执行入口。
- Chat UI 仍在“选 expert”，而不是“选 CLI tool + 选 model”。
- workflow / orchestration 默认仍主要通过 expert id 选 `codex` / `claudecode`，而不是从工具配置读取默认模型。

参考 `.github-feature-analyzer/` 中的 `fengshao1227-ccg-workflow` 与 `BloopAI-vibe-kanban` 后，可以确认更清晰的抽象是 executor-first：先选 backend/CLI tool，再在该 tool 内部指定模型。

## Goals / Non-Goals

**Goals:**
- 将主执行器显式收敛成 `Codex CLI` 与 `Claude Code` 两个 tool。
- 将协议族与工具绑定：OpenAI-compatible -> Codex，Anthropic-compatible -> Claude。
- 让 Chat 页面改成“先选工具，再选模型”。
- 保留 helper SDK 能力，但不再把模型页中的模型直接暴露成主执行 expert。
- 让 workflow / orchestration 默认模型来自 CLI tool 配置，而不是 expert 镜像。

**Non-Goals:**
- 不扩展更多第三方 CLI 工具（Gemini/Qwen/Cursor 等）到本次实现。
- 不改 Repo Library / analyzer 能力。
- 不重写 execution/workspace 基座。
- 不要求本次完全重做 Expert Builder 的产品形态，只确保它不阻碍新的主抽象。

## Decisions

### 1. 新增 `cli_tools` 作为主执行配置层

在配置中新增 `cli_tools[]`，每个 tool 包含：
- `id`
- `label`
- `protocol_family`
- `cli_family`
- `enabled`
- `default_model_id`
- `command_path`（可选）

本期内建两条：
- `codex` → `openai`
- `claude` → `anthropic`

### 2. `llm.models` 不再自动暴露成主执行 expert

模型页继续维护 source/model 池，但这些模型默认只作为：
- 某个 CLI tool 的可选模型
- helper SDK 的可选模型

主执行面不再把 `llm.models` 自动展示为主 expert。

### 3. expert 回退为 persona / policy 层

expert 继续存在，但其职责改为：
- system prompt / prompt template
- skill / MCP policy
- 默认 tool / 默认 model override

默认主执行时，tool/model 由请求显式选择或 CLI tool 默认值决定。

### 4. Chat API 改为显式支持 `cli_tool_id + model_id`

chat session 和 turn API 新增：
- `cli_tool_id`
- `model_id`

`expert_id` 继续保留兼容，但在主路径里降为 persona 选择，不再承担工具/模型映射的唯一入口。

### 5. workflow / orchestration 默认模型来自 CLI tool 设置

workflow 的 master/worker 与 orchestration 的 agent/synthesis 继续沿用 CLI runtime，但当 expert 为主执行工具 expert（如 `codex` / `claude`）时，解析模型优先使用对应 CLI tool 的默认模型配置。

## Risks / Trade-offs

- [兼容性] 旧 UI / 旧 API 仍在传 `expert_id` → 保留兼容入口，并在后端做双写/兜底解析。
- [抽象重叠] expert 与 cli tool 同时存在 → 明确将 expert 定位为 persona 层，将 tool 定位为 executor 层。
- [测试改动面大] 现有测试假设 `llm-model` expert 可直接执行 → 需要更新测试夹具与前端过滤逻辑。
- [Builder 路径仍依赖 llm-model] → 本次允许 builder 继续使用 helper 模型池，不阻塞主路径收敛。

## Migration Plan

1. 新增 CLI tool 配置结构与 settings API。
2. 调整 registry / resolver，使 CLI tool + model 成为主解析路径。
3. 停止主 UI 把 `llm-model` 当作主执行 expert 使用。
4. 改 Chat 页面为工具优先选择。
5. 改 workflow / orchestration 默认模型来源为 tool 配置。
6. 同步 spec、验证、归档。
