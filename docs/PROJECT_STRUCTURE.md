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
| `desktop/` | 桌面壳（预留）：后续承载 Wails 集成 |
| `scripts/` | 仓库级脚本：本地开发启动与辅助命令 |
| `.codex/skills/` | 项目级 Codex skills：流程规范与协作约束 |
| `docs/` | 项目文档：结构索引、MVP TodoList、详细规划与维护规则 |

## 3. 当前关键文件索引

| 文件 | 作用 |
|---|---|
| `AGENTS.md` | 仓库级 Agent 协作入口（技能路由、执行流程、交付要求） |
| `backend/cmd/vibe-tree-daemon/main.go` | daemon 进程入口，负责加载配置、启动 HTTP Server、处理优雅退出 |
| `backend/internal/server/server.go` | Gin Engine 装配：恢复中间件、请求日志、dev CORS，并挂载 `internal/api` 路由 |
| `backend/internal/api/api.go` | HTTP/WS handlers：health、workflow CRUD、execution start/log/cancel、WebSocket 升级入口 |
| `backend/internal/api/workflows.go` | Workflow HTTP handlers：create/list/get/patch，并广播 `workflow.updated` |
| `backend/internal/api/workflow_start.go` | Workflow start/nodes handlers：创建 master node + execution，并提供 nodes 查询 |
| `backend/internal/config/config.go` | 配置读取逻辑，处理默认值、XDG 路径、环境变量覆盖 |
| `backend/internal/runner/pty_runner.go` | PTY runner：启动子进程、流式输出、Cancel（SIGTERM→grace→SIGKILL） |
| `backend/internal/execution/manager.go` | Execution 管理：启动/取消、日志落盘、WS 推送 `execution.*`/`node.log` |
| `backend/internal/ws/hub.go` | WebSocket hub：连接管理与广播（配合 log tail 断线补齐） |
| `backend/internal/store/sqlite.go` | SQLite state DB 打开与 pragma 初始化（WAL/busy_timeout/foreign_keys） |
| `backend/internal/store/migrate.go` | SQLite migrations（使用 `PRAGMA user_version` 管理 schema 版本） |
| `backend/internal/store/workflows.go` | Workflow 存储：SQLite CRUD + events 写入 |
| `backend/internal/store/nodes.go` | Node 存储：master node 创建、nodes 列表查询 |
| `backend/internal/store/executions.go` | Execution 存储：execution started/exited 落库、同步 node/workflow 状态与 events |
| `backend/internal/store/failures.go` | 失败兜底：启动/落库失败时标记 node/workflow failed |
| `backend/internal/store/recovery.go` | 重启恢复：将 DB 中遗留的 running execution 标记为 failed（daemon_restarted） |
| `backend/internal/paths/paths.go` | XDG data/logs/state.db 路径解析（`~/.local/share/vibe-tree/...`） |
| `backend/internal/id/id.go` | ID 生成：`wf_`/`nd_`/`ex_` 前缀 ID（MVP 先用短随机） |
| `backend/internal/logx/logx.go` | 后端统一日志格式封装（`level=... module=... action=... msg="..."`） |
| `ui/src/App.tsx` | 前端首页：daemon health + workflow Kanban + `#/workflows/:id` 详情（nodes 列表联动终端）+ WS 订阅 |
| `ui/src/components/TerminalPane.tsx` | xterm.js 封装组件（fit + write/reset 接口） |
| `ui/src/lib/daemon.ts` | daemon URL/WS URL 解析与 health/workflow/execution API 封装 |
| `scripts/dev.sh` | 本地开发一键启动脚本（并行拉起 backend 与 UI） |

## 4. 功能定位索引（关键词 -> 文件）

| 关键词 | 优先查看文件 |
|---|---|
| daemon 启动入口 | `backend/cmd/vibe-tree-daemon/main.go` |
| 健康检查 API | `backend/internal/api/api.go` |
| dev CORS 配置 | `backend/internal/server/server.go` |
| 请求日志格式 | `backend/internal/server/server.go` |
| daemon 地址默认值 | `backend/internal/config/config.go`, `ui/src/lib/daemon.ts` |
| XDG 配置路径 | `backend/internal/config/config.go` |
| XDG 日志路径 | `backend/internal/paths/paths.go` |
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
| ID 生成规则 | `backend/internal/id/id.go` |
| 后端统一日志格式 | `backend/internal/logx/logx.go`, `backend/internal/server/server.go` |
| 前端连通性提示 | `ui/src/App.tsx` |
| Workflow 列表/创建 UI | `ui/src/App.tsx`, `ui/src/lib/daemon.ts` |
| Workflow Kanban/详情 UI | `ui/src/App.tsx`, `ui/src/lib/daemon.ts`, `ui/src/components/TerminalPane.tsx` |
| health/workflow/execution API 封装 | `ui/src/lib/daemon.ts` |
| 终端渲染与路由 | `ui/src/components/TerminalPane.tsx`, `ui/src/App.tsx` |
| 本地一键启动 | `scripts/dev.sh` |
| UI 开发端口代理与构建配置 | `ui/vite.config.ts`, `ui/package.json` |
| Agent 协作与 skill 路由 | `AGENTS.md` |

## 5. 维护规则

1. 新增核心模块、重命名目录或职责变化时，必须同步更新本文件的“根目录职责表”。
2. 新增关键入口文件时，必须补充到“当前关键文件索引”。
3. 新增或调整功能落点时，必须补充或修订“功能定位索引（关键词 -> 文件）”。
4. 功能定位优先使用本文件提供的候选路径，再在目标目录内使用 `rg` 做定向检索。
