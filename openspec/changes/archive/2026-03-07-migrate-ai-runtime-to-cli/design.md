## Context

当前仓库已经有一套很适合承接 CLI agent 的后端基座：`execution.Manager` 负责日志/取消/超时，`workspace.Manager` 负责 shared workspace / git worktree，`workflow` 与 `project-orchestration` 已经把任务生命周期拆成持久化对象。但 AI 主执行路径依然被建模成 SDK provider/model：

- `expert.Resolve()` 主要产出 `SDKSpec` 或 `process` 命令。
- `runner.MultiRunner` 仍以 `spec.SDK != nil` 作为主要 AI 分流点。
- `chat` 明确要求 session turn 只能使用 SDK expert，并依赖 provider anchor 做连续性。
- `workflow` 与 `orchestration` 的异步执行框架大体可复用，但默认 expert 解析与主执行语义仍是 SDK-first。

用户当前希望先完成“后端全面 CLI 化”，并明确排除 Repo Library / `github-feature-analyzer` 集成。因此本设计聚焦：把 chat、workflow、orchestration 三条主 AI 线路切换到 CLI runtime，同时保留 SDK 作为少量 helper 服务。

另一个现实约束是已有未归档 change `chat-per-message-model-routing` 建立在 SDK-first chat 假设上。本 change 需要复用其中可取的数据/UI思路，但在 runtime 方向上覆盖它的核心假设。

## Goals / Non-Goals

**Goals:**
- 将 chat、workflow、project orchestration 的默认 AI 执行路径统一到 CLI runtime。
- 复用现有 execution / workspace / orchestration 基座，而不是重写整套生命周期管理。
- 建立统一的 CLI wrapper / artifact / session contract，使 CLI 家族差异被限制在 adapter 层。
- 让 SDK 只保留为 helper-only 能力，例如 thinking translation、LLM connectivity test、明确批准的单次辅助生成。
- 为后续真正的 CLI session / app-server 演进留出接口，但不把它纳入本阶段交付。

**Non-Goals:**
- 不集成 Repo Library、`github-feature-analyzer` 或任何外部仓库知识库能力。
- 不在本阶段完成 `codex app-server` 级别的交互式 session lane。
- 不移除全部 SDK 代码路径；helper SDK 仍然保留并继续受支持。
- 不做完整 UI 重设计；仅要求后端 contract 与必要的兼容字段调整。

## Decisions

### 1. 以 `runtime_kind` 重构主执行抽象

所有 expert 与运行入口统一收敛到三类 runtime：

- `cli`: 主 AI 执行路径，承接 chat / workflow / orchestration 的默认运行。
- `process`: 非 AI 的本地命令型 expert，继续用于 bash/process 类节点。
- `sdk_helper`: 只允许用于翻译、测试、一次性辅助生成等 helper 场景。

这样可以把“模型/SDK”从产品主抽象降级为 runtime 内部实现细节。相比继续在 chat 保持 SDK、只迁 workflow/orchestration，这种做法更符合用户的明确目标，也能减少不同 AI surface 之间的行为分裂。

### 2. 共享 `execution.Manager`，只替换 AI runner/adapter

不重写 execution 状态机。CLI runtime 通过新的 adapter/wrapper 进入现有 `execution.Manager`，复用日志流、取消、超时、execution id 与 orchestration/workflow 关联能力。

CLI wrapper 统一写出 artifact 目录，最小 contract 包括：

- `summary.json`
- `artifacts.json`

可选文件包括：

- `final_message.md`
- `tool_calls.jsonl`
- `patch.diff`
- `session.json`

该设计将 CLI 家族差异隔离在 wrapper/adapter 层，而不是散落在 chat/workflow/orchestration 里。

### 3. 先做 oneshot CLI execution，不在第一阶段引入交互式 app-server

本阶段所有主 AI surface 先以 oneshot CLI execution 为主，通过 artifact contract 把结果结构化回传给后端。对 chat 来说，daemon 仍负责 session/message/summary 持久化；CLI 只负责单次 turn 的主生成。这样可以在不重做 transport 的前提下快速把后端切换到 CLI。

备选方案是立刻引入长生命周期 CLI session/app-server，但那会同时放大会话恢复、增量事件、多并发与权限模型复杂度，不适合作为当前阶段的第一步。

### 4. Chat 从 provider anchor 切到 daemon-owned session state

新的 chat 继续保留 persistent session、attachments、fork、manual compact 与 streaming 事件，但不再依赖 `previous_response_id` / `container` 这类 provider anchor。连续性由本地 session summary、recent history，以及必要时的 CLI runtime session reference 共同承担。

这项决策避免 chat runtime 被某一类 SDK/endpoint 能力锁死，也更符合“主对话路径可带文件/工具上下文”的目标。

### 5. SDK 只作为 helper-only lane

SDK 仍保留在以下类任务：

- thinking translation
- `POST /api/v1/settings/llm/test`
- 明确批准的单次辅助生成（例如 expert builder 草稿生成）

这些 helper 调用不得被 chat/workflow/orchestration 的默认 expert 选择逻辑自动选为主执行路径。需要 SDK fallback 的能力也只在 helper lane 内生效。

### 6. 先覆盖主运行时，Repo Library 明确延后

本 change 明确不落任何 Repo Library / analyzer 集成内容，也不把外部仓库知识抽取、索引、检索混入本轮设计。这样可以把风险聚焦在 runtime pivot 本身，避免 scope 膨胀。

## Risks / Trade-offs

- [Chat 迁移最复杂] → chat 目前同步耦合 SDK、anchor、thinking translation；通过“保留 REST/WS 形状 + 替换 turn runtime + 保留 helper translation”分阶段落地。
- [CLI 家族行为差异] → 通过 `cli_family + wrapper contract` 隔离差异，不让业务层直接拼 CLI 参数。
- [SDK 与 CLI 双栈并存增加复杂度] → 明确 `sdk_helper` 与 `cli` 的能力边界，并在 expert/runtime metadata 上强制可过滤。
- [结构化输出从 SDK 转到 CLI 后更易失稳] → 对 workflow master / orchestration planning 的结构化输出统一走 artifact file contract，由 daemon 负责验证后再接受。
- [与 `chat-per-message-model-routing` 变更重叠] → 本 change 在 runtime 假设上覆盖旧 change；实现前需要明确是合并其有价值部分还是终止旧 change。

## Migration Plan

1. 扩展 expert/config/store 的 runtime 元数据，建立 `cli`/`process`/`sdk_helper` 基本抽象。
2. 引入 CLI adapter + wrapper contract，并接入 `runner.MultiRunner` 与 daemon 依赖注入。
3. 迁移 workflow 与 orchestration，让 master/worker/agent/synthesis 默认走 CLI runtime，同时保留 execution/workspace 复用。
4. 迁移 chat turn 执行路径，去掉 provider-anchor 依赖，补齐 CLI-backed session metadata 与 compaction/attachment 兼容。
5. 收缩 SDK 使用面，只保留 helper-only surfaces，并补齐相应测试与兼容策略。

回滚策略：首轮迁移不删除旧表和旧字段，不移除 `SDKRunner`；若 CLI 主链路出现阻塞，可在局部 helper surface 继续保留 SDK，并通过配置恢复旧 expert 路径进行应急回退。

## Open Questions

- `expert builder` 是否在本阶段继续视为 `sdk_helper`，还是一并迁到 CLI oneshot？
- thinking translation 的目标匹配在 CLI chat 下继续沿用 `target_model_ids`，还是改为 `target_expert_ids` / resolved model metadata？
- 第一阶段必须支持哪些 CLI family（仅 `codex`，还是 `codex + claude` 同时落地）？
