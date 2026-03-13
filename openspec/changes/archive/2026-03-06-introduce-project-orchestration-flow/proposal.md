## Why

当前 `workflows` 的核心价值，其实是“master 生成静态 DAG → daemon 落库 → scheduler 按依赖驱动 execution”的运行时链路：它已经具备 execution、日志流、取消、重试、WebSocket 推送、SQLite 持久化与并发控制这些底层能力，但它的主产品模型仍然是 DAG-first automation。这个模型并不匹配 `vibecraft` 真正想做的“AI 辅助项目开发”：用户先输入一个开发目标，master 再按需要动态拆成多名子 agent 并行分析/修改/验证，经过 synthesis 后必要时继续下一轮。

现在需要在不破坏既有 `workflows` 价值的前提下，把 `vibecraft` 的主入口从“静态 DAG 编排器”升级为“面向项目开发的 orchestration 系统”：保留旧 runtime，冻结旧产品方向，新增一套 prompt-first 的主流程作为未来主入口。

## What Changes

- 新增 `Orchestrations` 主流程与主页面：用户从顶部输入任务目标直接启动一次 orchestration，而不是先创建 workflow 再启动。
- 新增 `orchestration / round / agent run / synthesis step / artifact` 数据模型与对应 API，支持动态分叉、多轮并行、结果汇总、继续下一轮、取消与重试。
- 新增面向项目开发的 UI：同一轮并行 agent 同一行展示，每个 agent 卡片都能看到角色、任务目标、状态、输出摘要、日志入口、是否修改代码与代码变更摘要。
- 新增代码开发导向的 workspace/worktree/branch 语义，为修改型 agent 记录工作目录策略、分支/工作树信息与 merge-ready 摘要。
- 明确复用边界：沿用既有 execution manager、runner、日志落盘/回放、cancel/retry、WebSocket hub、SQLite 持久化模式与全局并发预算；不再把静态 DAG、DAG-first UI、以及“先建 workflow 再 start”的交互路径当作主产品模型。
- 保留旧 `workflows` 功能与 runtime，不立即删除；UI 上将其降级为 `Legacy Workflows` 次级入口，等新主流程覆盖实际价值后再评估移除。

## Capabilities

### New Capabilities
- `project-orchestration`: 面向项目开发的 prompt-first orchestration 生命周期，覆盖 master 规划、动态 round、agent run 并行执行、synthesis 汇总与继续下一轮。
- `project-orchestration-ui`: 新的主入口与详情视图，覆盖顶部目标输入区、轮次视图、并行 agent 卡片、详情面板、日志与代码变更展示，以及 `Legacy Workflows` 的信息架构定位。
- `agent-workspace-flow`: 面向代码开发的 agent workspace/worktree/branch 流程与 artifact 摘要模型，支持分析、修改、验证三类 agent 的工作目录策略与 merge 导向交付。

### Modified Capabilities
- `execution`: 扩展 execution 的上下文关联能力，使其既能继续服务 legacy workflow/node，也能服务新的 orchestration/round/agent run，并复用现有日志、取消与 WebSocket 生命周期事件。

## Impact

- 后端：新增 orchestration 领域模型、store、controller、HTTP/WS API，并抽象 workflow 专用的调度/启动逻辑为可复用 runtime 服务。
- 前端：新增 `Orchestrations` 页面与详情组件；调整顶层导航；保留并弱化旧 `Workflows` 页面。
- 数据：SQLite 需要新增 orchestration 相关表，并为 execution 增加可关联 orchestration agent run 的上下文持久化。
- 运行时：继续复用 `execution.Manager`、`runner.MultiRunner`、磁盘日志、`ws.Hub`、恢复流程与并发限制，但不复用 DAG 解析/落库作为新主流程的核心模型。
- 迁移：需要提供“新主入口 + 旧 workflow 并存”的渐进式引入方案，保证现有用户和已有 workflow API 不被破坏。
