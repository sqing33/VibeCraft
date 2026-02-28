# vibe-tree 项目结构与功能定位索引

> 更新时间：2026-02-27  
> 说明：本文档用于开发期快速定位功能文件。修改前先读本文件，再做定向检索。

## 1. 项目概览

`vibe-tree` 当前处于 MVP 早期阶段，已完成 daemon 与 UI 最小联通链路，并在此基础上推进 PTY、工作流与 DAG 编排能力。

## 2. 根目录职责表

| 路径 | 职责 |
|---|---|
| `backend/` | Go daemon：配置加载、HTTP API、后端服务启动与运行时能力 |
| `ui/` | React 前端：健康检查展示、工作流页面与后续交互 |
| `desktop/` | 桌面壳（Wails）：启动/复用 daemon，并在 WebView 内打开 UI |
| `scripts/` | 仓库级脚本：本地开发启动与辅助命令 |
| `.codex/skills/` | 项目级 Codex skills：流程规范与协作约束 |
| `docs/` | 项目文档：结构索引、MVP TodoList、详细规划与维护规则 |

## 3. 当前关键文件索引

| 文件 | 作用 |
|---|---|
| `AGENTS.md` | 仓库级 Agent 协作入口（技能路由、执行流程、交付要求） |
| `backend/cmd/vibe-tree-daemon/main.go` | daemon 进程入口，负责加载配置、启动 HTTP Server、处理优雅退出 |
| `backend/internal/server/server.go` | Gin Engine 装配：恢复中间件、请求日志、dev CORS，并挂载 `internal/api` 路由；可选挂载 UI 静态资源（`ui/dist` 或 `VIBE_TREE_UI_DIST`） |
| `backend/internal/api/api.go` | HTTP/WS handlers：health、workflow CRUD、execution start/log/cancel、WebSocket 升级入口 |
| `backend/internal/api/info.go` | 排障信息 API：`GET /api/v1/info`（version + XDG paths） |
| `backend/internal/api/experts.go` | Experts 列表 API：`GET /api/v1/experts`（仅安全字段，供 UI 下拉） |
| `backend/internal/api/workflows.go` | Workflow HTTP handlers：create/list/get/patch，并广播 `workflow.updated` |
| `backend/internal/api/workflow_start.go` | Workflow start/nodes handlers：创建 master node + execution，并提供 nodes 查询 |
| `backend/internal/api/workflow_cancel.go` | Workflow cancel handler：取消 workflow（取消 running execution + 标记未开始节点 canceled） |
| `backend/internal/api/nodes.go` | Node handlers：patch/retry/cancel（`PATCH /api/v1/nodes/:id`、`POST /api/v1/nodes/:id/retry`、`POST /api/v1/nodes/:id/cancel`） |
| `backend/internal/config/config.go` | 配置读取逻辑，处理默认值、XDG 路径、环境变量覆盖 |
| `backend/internal/expert/expert.go` | Expert 注册表：基于 config 解析 `expert_id` -> RunSpec（`{{prompt}}`/`${ENV}` 模板替换、timeout），并提供已知 expert 集合 |
| `backend/internal/runner/pty_runner.go` | PTY runner：启动子进程、流式输出、Cancel（SIGTERM→grace→SIGKILL） |
| `backend/internal/execution/manager.go` | Execution 管理：启动/取消、日志落盘、WS 推送 `execution.*`/`node.log` |
| `backend/internal/dag/dag.go` | DAG 解析与校验：从 master 输出提取第一个 JSON 对象并做 MVP 约束校验（无环/引用存在/expert 校验） |
| `backend/internal/scheduler/scheduler.go` | Workflow 调度器：依赖 + 并发上限 + fail-fast（启动 queued worker nodes 并收敛终态） |
| `backend/internal/ws/hub.go` | WebSocket hub：连接管理与广播（配合 log tail 断线补齐） |
| `backend/internal/store/sqlite.go` | SQLite state DB 打开与 pragma 初始化（WAL/busy_timeout/foreign_keys） |
| `backend/internal/store/migrate.go` | SQLite migrations（使用 `PRAGMA user_version` 管理 schema 版本） |
| `backend/internal/store/workflows.go` | Workflow 存储：SQLite CRUD + events 写入 |
| `backend/internal/store/dag.go` | DAG 落库：从 master DAG 创建 worker nodes/edges，并提供 edges 查询 |
| `backend/internal/store/nodes.go` | Node 存储：master node 创建、nodes 列表查询、GetNode、RetryNode（解开 fail-fast skipped 并重试） |
| `backend/internal/store/executions.go` | Execution 存储：execution started/exited 落库、同步 node/workflow 状态与 events |
| `backend/internal/store/approval.go` | Manual approval：runnable `pending_approval` 节点批准为 `queued` |
| `backend/internal/store/node_patch.go` | Node 编辑：PATCH prompt/expert_id，写入 `prompt.updated` 与 `node.updated` |
| `backend/internal/store/cancel.go` | Workflow 取消：标记 workflow canceled，并返回需 cancel 的 running executions |
| `backend/internal/store/failures.go` | 失败兜底：启动/落库失败时标记 node/workflow failed |
| `backend/internal/store/recovery.go` | 重启恢复：将 DB 中遗留的 running execution 标记为 failed（daemon_restarted） |
| `backend/internal/paths/paths.go` | XDG data/logs/state.db 路径解析（`~/.local/share/vibe-tree/...`） |
| `backend/internal/id/id.go` | ID 生成：`wf_`/`nd_`/`ex_` 前缀 ID（MVP 先用短随机） |
| `backend/internal/logx/logx.go` | 后端统一日志格式封装（`level=... module=... action=... msg="..."`） |
| `backend/internal/version/version.go` | 版本信息（Commit/BuiltAt，可用 ldflags 注入；用于 `/api/v1/info`） |
| `ui/src/App.tsx` | 前端首页：daemon health + workflow Kanban + `#/workflows/:id` 详情（DAG + 节点联动终端 + manual 审批/编辑）+ WS 订阅 |
| `ui/src/components/DAGView.tsx` | React Flow DAG 视图：dagre 自动布局 + 节点按状态上色 + 点击节点联动终端 |
| `ui/src/components/TerminalPane.tsx` | xterm.js 封装组件（fit + write/reset 接口） |
| `ui/src/lib/daemon.ts` | daemon URL/WS URL 解析与 health/workflow/execution API 封装 |
| `scripts/dev.sh` | 本地开发一键启动脚本（并行拉起 backend 与 UI） |
| `scripts/web.sh` | Web 单进程启动脚本（构建 UI 并由 daemon 静态托管 `ui/dist`） |
| `desktop/main.go` | Wails 桌面入口：嵌入 `frontend/src`，注册 Menu，并 Bind `App` 方法供前端调用 |
| `desktop/app.go` | Desktop 业务逻辑：解析 daemon host/port，确保 daemon 可用（必要时子进程拉起），并提供“打开数据目录”等动作 |

## 4. 功能定位索引（关键词 -> 文件）

| 关键词 | 优先查看文件 |
|---|---|
| daemon 启动入口 | `backend/cmd/vibe-tree-daemon/main.go` |
| 健康检查 API | `backend/internal/api/api.go` |
| daemon info API | `backend/internal/api/info.go`, `ui/src/lib/daemon.ts`, `ui/src/App.tsx` |
| experts 列表 API | `backend/internal/api/experts.go`, `backend/internal/expert/expert.go`, `ui/src/lib/daemon.ts`, `ui/src/App.tsx` |
| dev CORS 配置 | `backend/internal/server/server.go` |
| 请求日志格式 | `backend/internal/server/server.go` |
| UI 静态资源挂载（ui/dist） | `backend/internal/server/server.go`, `scripts/web.sh` |
| daemon 地址默认值 | `backend/internal/config/config.go`, `ui/src/lib/daemon.ts` |
| UI 运行时切换 daemon URL | `ui/src/App.tsx` |
| XDG 配置路径 | `backend/internal/config/config.go` |
| XDG 日志路径 | `backend/internal/paths/paths.go` |
| Expert 配置/模板解析 | `backend/internal/config/config.go`, `backend/internal/expert/expert.go` |
| execution timeout 语义 | `backend/internal/execution/manager.go`, `backend/internal/scheduler/scheduler.go` |
| SQLite state.db 初始化 | `backend/cmd/vibe-tree-daemon/main.go`, `backend/internal/store/sqlite.go`, `backend/internal/store/migrate.go` |
| Workflow CRUD API | `backend/internal/api/workflows.go`, `backend/internal/store/workflows.go` |
| Workflow Start（master 执行） | `backend/internal/api/workflow_start.go`, `backend/internal/store/nodes.go`, `backend/internal/store/executions.go` |
| Workflow Nodes API | `backend/internal/api/workflow_start.go`, `backend/internal/store/nodes.go` |
| 重启恢复（running→failed） | `backend/cmd/vibe-tree-daemon/main.go`, `backend/internal/store/recovery.go` |
| execution 启动 | `backend/internal/api/api.go`, `backend/internal/execution/manager.go` |
| execution 日志落盘 | `backend/internal/execution/manager.go` |
| execution log tail API | `backend/internal/api/api.go`, `backend/internal/execution/logtail.go` |
| WebSocket 推送 | `backend/internal/api/api.go`, `backend/internal/ws/hub.go`, `backend/internal/ws/envelope.go` |
| PTY runner | `backend/internal/runner/pty_runner.go` |
| execution cancel API | `backend/internal/api/api.go`, `backend/internal/execution/manager.go` |
| DAG JSON 提取/校验 | `backend/internal/dag/dag.go`, `backend/internal/api/workflow_start.go` |
| DAG 落库（nodes/edges） | `backend/internal/store/dag.go`, `backend/internal/api/workflow_start.go` |
| 调度器（依赖/并发/fail-fast） | `backend/internal/scheduler/scheduler.go`, `backend/cmd/vibe-tree-daemon/main.go`, `backend/internal/store/scheduler.go` |
| manual approve API | `backend/internal/api/approve.go`, `backend/internal/store/approval.go` |
| node patch API | `backend/internal/api/nodes.go`, `backend/internal/store/node_patch.go` |
| node retry/cancel API | `backend/internal/api/nodes.go`, `backend/internal/store/nodes.go`, `backend/internal/scheduler/scheduler.go` |
| workflow mode 切换（断点） | `backend/internal/api/workflows.go`, `backend/internal/store/workflows.go` |
| workflow cancel API | `backend/internal/api/workflow_cancel.go`, `backend/internal/store/cancel.go` |
| ID 生成规则 | `backend/internal/id/id.go` |
| 后端统一日志格式 | `backend/internal/logx/logx.go`, `backend/internal/server/server.go` |
| 前端连通性提示 | `ui/src/App.tsx` |
| Workflow 列表/创建 UI | `ui/src/App.tsx`, `ui/src/lib/daemon.ts` |
| Workflow Kanban/详情 UI | `ui/src/App.tsx`, `ui/src/lib/daemon.ts`, `ui/src/components/TerminalPane.tsx` |
| Workflow DAG 视图（React Flow） | `ui/src/components/DAGView.tsx`, `ui/src/App.tsx` |
| health/workflow/execution API 封装 | `ui/src/lib/daemon.ts` |
| 终端渲染与路由 | `ui/src/components/TerminalPane.tsx`, `ui/src/App.tsx` |
| 本地一键启动 | `scripts/dev.sh` |
| Web 单进程启动（daemon 托管 UI） | `scripts/web.sh`, `backend/internal/server/server.go` |
| UI 开发端口代理与构建配置 | `ui/vite.config.ts`, `ui/package.json` |
| desktop 拉起/复用 daemon | `desktop/app.go`, `desktop/frontend/src/main.js` |
| desktop 打开数据目录 | `desktop/main.go`, `desktop/app.go` |
| Agent 协作与 skill 路由 | `AGENTS.md` |

## 5. 维护规则

1. 新增核心模块、重命名目录或职责变化时，必须同步更新本文件的“根目录职责表”。
2. 新增关键入口文件时，必须补充到“当前关键文件索引”。
3. 新增或调整功能落点时，必须补充或修订“功能定位索引（关键词 -> 文件）”。
4. 功能定位优先使用本文件提供的候选路径，再在目标目录内使用 `rg` 做定向检索。
