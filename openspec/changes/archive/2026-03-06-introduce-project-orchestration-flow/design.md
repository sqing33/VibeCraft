## Context

`vibe-tree` 当前已经具备一条相当完整的 workflow runtime：

- 后端已有 workflow CRUD、start/approve/cancel、node retry、execution start/cancel/log tail、WebSocket 推送、SQLite 状态落库与 daemon 重启恢复。
- master 节点执行成功后，会从输出中提取第一个 JSON 对象，按 DAG schema 校验，再生成 worker nodes/edges；scheduler 基于依赖关系和全局并发上限推进 `queued` worker。
- `execution.Manager` 已经统一处理 PTY/SDK 执行、日志落盘、`execution.started` / `node.log` / `execution.exited` 推送，以及 SIGTERM → SIGKILL 的取消机制。
- 前端 `#/workflows` + `#/workflows/:id` 主要围绕 Kanban 列表、DAG 视图和终端日志展开，适合查看静态计划和节点执行链路。

这些能力说明：现有 `workflows` 更像“静态 DAG 自动化编排器”，而不是“动态 AI 项目开发协作系统”。它的可复用点主要在 runtime，不在产品模型。

### 当前 `workflows` 的真实能力

- **应保留复用**
  - execution/runtime：`runner.RunSpec`、`runner.MultiRunner`、`execution.Manager`
  - 日志流：磁盘日志、log tail API、`node.log` 推送节流
  - cancel/retry：execution cancel、node retry、workflow cancel
  - WebSocket 推送：`ws.Hub` 与事件广播机制
  - 状态持久化：SQLite store、事件审计、daemon 重启恢复
  - 并发控制：scheduler 的全局并发预算与“当前运行数”约束
- **不应继续作为主产品模型**
  - master 必须先产出一份完整静态 DAG
  - 节点/边/DAG 作为主 UI 心智模型
  - 先创建 workflow，再 start 的主交互路径
  - manual approve 作为 DAG breakpoint 的核心交互
- **需要全新设计**
  - 顶部问题输入框与 prompt-first 主入口
  - 动态 agent 分叉与多轮并行
  - round 级 synthesis 和下一轮决策
  - 面向项目开发的任务/结果视图
  - worktree / branch / merge 导向的开发流

另一个值得利用的现状是：`#/chat` 已经验证了“顶部输入 + 流式事件 + 会话详情”的交互骨架，它比 `#/workflows` 更接近新主流程的交互节奏；但 chat 的会话模型仍是单线程对话，不适合直接承载多 agent 多轮并行。因此新主流程应借用 chat 的输入/流式体验，而不是借用 workflow 的 DAG 心智模型。

## Goals / Non-Goals

**Goals:**
- 新增一套以 `Orchestrations` 为主入口的 prompt-first 项目开发流程。
- 让 master 能按运行时上下文动态决定是否拆成 1~N 个子 agent，并支持多轮并行与 synthesis。
- 最大化复用既有 execution/logging/cancel/retry/ws/persistence/concurrency 能力。
- 为修改型 agent 提供 worktree / branch / code-change summary 语义，让结果更接近真实项目开发协作。
- 在不破坏现有功能的前提下，把旧 `workflows` 降级为 `Legacy Workflows`，冻结其产品方向。

**Non-Goals:**
- 本变更不删除旧 `workflows` / DAG runtime，也不要求把旧表结构强行迁移到新模型。
- 本变更不把新 orchestration 伪装成“动态 DAG”；DAG 不再是主产品抽象。
- 本变更不承诺自动合并所有子 agent 代码结果；先提供 worktree/branch 摘要与人工 review 入口。
- 本变更不尝试把 chat session 与 orchestration 统一成同一数据模型；两者先并存，各自面向不同产品目标。

## Decisions

### 1. 采用“双轨产品、共享 runtime”策略

- **决策**：保留旧 `workflows` 的 runtime 和产品入口，但明确改名为 `Legacy Workflows`；新增 `Orchestrations` 作为未来主入口。
- **原因**：当前 runtime 仍有真实价值，直接删除会增加回归风险；但继续强化 DAG-first 路线会让产品越走越偏。
- **备选方案**：直接把旧 `workflows` UI 改皮包装成新产品。
- **不选原因**：这会把静态 DAG 的限制继续带入新主流程，难以支持真正的动态分叉和多轮 synthesis。

### 2. 新主流程采用“round-based orchestration”，而不是静态 DAG

- **决策**：master 先消费用户目标，输出当前轮的执行计划；每一轮包含若干并行 `agent_run`；所有 agent 完成后进入一个 `synthesis_step`；synthesis 再决定是 `complete`、`continue` 还是 `needs_retry`。
- **原因**：round 是 UI 可理解、也与“同一轮并行 agent 同一行展示”的要求天然匹配；同时它允许 master 基于上一轮结果继续分叉，而无需一开始就知道完整图结构。
- **备选方案**：允许 master 一次性输出完整 DAG，再把每一轮视作 DAG 的层级。
- **不选原因**：这仍然会让产品被静态图绑定，无法体现“先做一轮、看结果、再决定下一轮”的开发协作节奏。

### 3. 使用新的 orchestration 领域模型，不复用 `workflow/node/edge` 作为上层语义

- **决策**：新增以下核心数据模型：
  - `orchestration`: 用户输入的一次项目开发目标，主聚合根。
  - `round`: 一次由 master 发起的并行执行轮次。
  - `agent_run`: 一个具体子 agent 的执行单元，包含角色、目标、状态、workspace 策略和结果摘要。
  - `synthesis_step`: 针对某一轮的汇总结果、下一步判断与后续提示。
  - `artifact`: 与 agent run 或 synthesis 关联的结果物，如 code change summary、test result、git ref、analysis note。
- **原因**：这些对象直接服务于新产品语义，也便于 UI 直接消费。
- **备选方案**：继续沿用 `workflow/node/edge`，仅在 node 上增加 round/role 字段。
- **不选原因**：会把 legacy DAG 约束和新主流程耦合在一起，导致 store、UI、状态机都变得别扭。

### 4. 把 execution 复用为共享 runtime，并引入 orchestration 上下文关联层

- **决策**：保留 `execution.Manager`、log tail、cancel、WebSocket 生命周期事件；新增 execution 上下文关联，使 execution 可以绑定到 `agent_run`，并在事件中携带 `orchestration_id / round_id / agent_run_id`。
- **原因**：execution 已经很好地解决了“怎么跑、怎么打日志、怎么取消、怎么收敛”问题，新主流程没有必要重造这部分轮子。
- **备选方案**：为 orchestration 单独实现第二套运行器与日志通道。
- **不选原因**：重复实现风险高，也会让日志/取消/恢复行为出现两套语义。

### 5. 用“调度槽位 + round barrier”替代 DAG scheduler

- **决策**：复用全局并发预算，但从“调度 queued DAG node”改为“调度当前 round 内待执行的 agent runs”；同一 round 的 agent runs 可并行启动，下一轮必须等待当前 round 全部 terminal，再由 synthesis 决定是否继续。
- **原因**：保留当前系统成熟的并发预算控制，同时把调度边界从 DAG 依赖切换为 round 生命周期。
- **备选方案**：完全取消统一并发约束，让 master 输出多少就跑多少。
- **不选原因**：会导致资源失控，尤其在修改型 agent 同时打开多个 worktree 时风险更高。

### 6. 引入面向代码开发的 workspace 策略

- **决策**：每个 `agent_run` 都带有 `intent`（`analyze` / `modify` / `verify`）和 `workspace_mode`（`read_only` / `shared_workspace` / `git_worktree`）。
  - `modify` 型 agent 在 Git 仓库内默认使用 `git_worktree`。
  - `analyze` / `verify` 型 agent 默认使用 `read_only` 或 `shared_workspace`。
  - 若当前目录不是 Git 仓库，则降级为 `shared_workspace`，并在 artifact 中记录降级原因。
- **原因**：新主流程核心场景是同一项目同时推进多项工作，隔离工作目录是避免相互覆盖的关键。
- **备选方案**：所有 agent 共享同一工作目录。
- **不选原因**：多个修改型 agent 容易相互踩改动，无法形成清晰的 merge 导向交付。

### 7. 信息架构采用“Orchestrations 主入口 + Legacy Workflows 次入口”

- **决策**：
  - 新主页面名称：`Orchestrations`
  - 旧 `Workflows` 页面保留，但改名为 `Legacy Workflows`
  - 路由建议：`#/` 或 `#/orchestrations` 作为新主入口，`#/legacy-workflows` 作为旧入口，`#/workflows` 保留为兼容跳转
- **原因**：`Orchestrations` 直接对应新数据模型和用户心智；`Legacy Workflows` 清晰表达旧能力仍可用但不再是主方向。
- **备选方案**：把旧页改名为 `Pipelines` 或 `Automation`。
- **不选原因**：这会把旧能力重新包装成一个更宽泛的概念，不如 `Legacy Workflows` 直白且利于迁移阶段沟通。

### 8. UI 采用“顶部目标输入 + 轮次横向卡片 + 右侧详情面板”

- **决策**：
  - 顶部输入区：单个自然语言输入框 + workspace/context 选择 + 启动按钮。
  - 轮次视图：每个 round 一整行；行内横向排列多个 agent 卡片。
  - agent 卡片展示：角色、任务目标、状态、输出摘要、日志状态、是否改动代码。
  - 详情面板：显示选中 agent 或 synthesis 的完整日志、artifact、code change summary、验证结果与控制按钮。
- **原因**：这比 DAG 图更贴近“项目协作面板”，也更容易看出“这一轮谁和谁并行、每个人产出了什么”。
- **备选方案**：继续以 React Flow 画图作为主视图。
- **不选原因**：图结构对动态 round 和项目开发结果的表达效率都不高，而且会把注意力拉回节点/边而不是任务/结果。

### 9. 首版 API 与事件契约采用 orchestration-first 命名

- **决策**：首版后端 API 直接围绕 orchestration 领域建模，而不是复用 workflow 路径改参。
- **建议 REST 面**：
  - `POST /api/v1/orchestrations`：以用户 goal 直接创建并启动一次 orchestration
  - `GET /api/v1/orchestrations`：读取 orchestration 列表
  - `GET /api/v1/orchestrations/:id`：读取 orchestration 详情（含 rounds / agent runs / synthesis / artifacts）
  - `POST /api/v1/orchestrations/:id/cancel`：取消 orchestration
  - `POST /api/v1/orchestrations/:id/continue`：在 synthesis 后进入下一轮
  - `POST /api/v1/agent-runs/:id/retry`：重试失败或可恢复的 agent run
- **建议 WS 事件**：
  - `orchestration.updated`
  - `orchestration.round.updated`
  - `orchestration.agent_run.updated`
  - `orchestration.synthesis.updated`
  - 继续复用 `execution.started` / `node.log` / `execution.exited`
- **原因**：这能让新主流程的 API、状态机、UI 直接对齐业务概念，避免把 `workflow/node` 语义泄漏到新产品层。
- **备选方案**：在旧 `/workflows/*` 下增加 mode 或 type 字段来承载 orchestration。
- **不选原因**：会让客户端和 store 长期同时背负两套含混语义，后续拆分成本更高。

### 10. 首版交付采用“单轮可用 → 多轮闭环 → workspace 隔离 → UI 主入口”四段式推进

- **决策**：0→1 实现不一次性铺满所有野心，而是按下面四个切片推进：
  1. **切片 A：共享 runtime + 持久化地基**
     - 抽取 shared runtime service
     - 建好 orchestration/round/agent_run/synthesis/artifact 表与 execution 关联
  2. **切片 B：单轮 orchestration 跑通**
     - `POST /orchestrations` 可直接启动 master
     - master 能创建第一轮 agent runs
     - agent runs 可并行执行并回填状态/日志/摘要
  3. **切片 C：synthesis 与下一轮闭环**
     - round barrier 生效
     - synthesis 产出 `complete|continue|needs_retry`
     - 支持手动 `continue` 与 `retry`
  4. **切片 D：workspace/worktree + 新主 UI**
     - 修改型 agent 引入 worktree/branch 语义
     - 前端切换到 `Orchestrations` 主入口，并保留 `Legacy Workflows`
- **原因**：这样既能快速拿到第一个可运行闭环，又能把风险最高的 worktree/UI 部分放在 runtime 骨架稳定之后。
- **备选方案**：数据库、runtime、UI、worktree 一次性同步落地。
- **不选原因**：跨层风险过高，出了问题很难判断是状态机、执行层还是前端联动出了问题。

### 11. 首版默认策略偏保守，优先把闭环做稳

- **决策**：首版明确以下默认值与边界：
  - `max_agents_per_round` 默认值取 `4`
  - synthesis 完成后默认**不自动进入下一轮**，而是等待显式 `continue`
  - 修改型 agent 默认尝试 `git_worktree`；失败后降级到 `shared_workspace`，并写 artifact
  - 首版不做自动 merge，只产出 branch/worktree 引用与 code-change summary
- **原因**：这四项都直接决定 0→1 可控性。先让 orchestration 的“规划 → 并行 → 汇总 → 决策”闭环稳定，再扩大自动化程度。
- **备选方案**：默认自动多轮推进、自动 merge、默认更高并发。
- **不选原因**：错误恢复和结果可解释性会显著变差，不利于首版落地和调试。

## Risks / Trade-offs

- [动态 agent 数量过多导致资源争抢] → 通过 `max_agents_per_round`、全局 execution 槽位和 workspace 限制做双重约束；master 输出超限时强制裁剪并记录原因。
- [synthesis 质量不稳定，导致下一轮方向错误] → 要求 synthesis 读取所有 agent 摘要与 artifact；首版保留人工继续/取消入口，并把 synthesis decision 持久化以便审计。
- [worktree / git 依赖并非所有项目都满足] → 对非 Git 工作目录提供显式降级路径；artifact 中记录是否启用隔离分支/工作树。
- [WebSocket 事件量增加，前端渲染压力上升] → 继续复用 execution log 节流策略；高层状态变更走 `orchestration.updated` / `agent_run.updated` 这类粗粒度事件。
- [旧 workflow 与新 orchestration 同时存在，增加维护成本] → 明确 freeze legacy 产品范围；新的交互和需求优先落在 orchestration，不再向 DAG-first UI 添加主功能。
- [恢复与观测复杂度提高] → orchestration、round、agent run、synthesis 都写入持久化状态与审计事件；daemon 重启后按 terminal/non-terminal 状态恢复或失败收敛。

## Migration Plan

1. 新增 orchestration 相关表、execution context 关联与必要索引，不修改旧 workflow API 对外契约。
2. 抽取共享 runtime 启动/收敛逻辑，使 legacy workflow scheduler 与新 orchestration controller 都能复用。
3. 增加新的 orchestration API 与 WS 事件，同时保留旧 workflow 事件不变。
4. 前端新增 `Orchestrations` 页面并将其放到主导航；旧 `Workflows` 页面改名为 `Legacy Workflows` 并保留兼容路由。
5. 在迁移期保留两条路径并行运行；观察新主流程是否覆盖旧工作流的高频价值后，再决定是否提出单独的 legacy 下线 change。

回滚策略：
- 新表和新路由可独立禁用；若新 orchestration 功能需要回滚，旧 workflow 路径仍可单独继续工作。
- 由于本变更不删除旧 runtime 和旧 API，回滚重点是隐藏新入口与停用新 controller，而不是恢复被替换的旧行为。

## 0→1 Delivery Slices

### Slice A：Runtime 与 Persistence 地基

- 目标：保证 orchestration 模型可以被持久化，并复用现有 execution/log/cancel/WS 生命周期。
- 完成标志：可以创建 orchestration 记录，并让 execution 具备 orchestration 关联上下文。

### Slice B：单轮 Master → Agent Runs 闭环

- 目标：从 goal 触发 master 规划，再派生第一轮 agent runs，并在 UI/接口中看到其状态变化与日志。
- 完成标志：不需要 DAG，也能完成“创建 → 计划 → 并行执行 → round 完成”。

### Slice C：Synthesis / Continue / Retry

- 目标：为每一轮补齐 barrier、汇总与下一步控制，让 orchestration 真正形成多轮能力。
- 完成标志：一个 orchestration 至少可以完成两轮，且支持失败 agent run 的 retry 与 synthesis 后的 continue。

### Slice D：Workspace 隔离与主 UI 切换

- 目标：修改型 agent 能产出 worktree/branch/code-change 摘要；导航上新主入口切换完成。
- 完成标志：用户能从 `Orchestrations` 主入口发起一次项目开发任务，并在详情页看到并行 agent 与其代码产物。

## Open Questions

- 新主流程未来是否要与 `#/chat` 进一步融合为统一入口，还是长期保持两个并列产品面？
