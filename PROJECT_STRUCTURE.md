# vibe-tree 项目结构与功能定位索引

> 更新时间：2026-03-06
> 说明：本文档用于开发期快速定位功能文件。修改前先读本文件，再做定向检索。

## 1. 项目概览

`vibe-tree` 当前处于 MVP 早期阶段，已完成 daemon 与 UI 最小联通链路，并在此基础上推进 PTY、工作流与 DAG 编排能力。

## 2. 根目录职责表

| 路径             | 职责                                                                                   |
| ---------------- | -------------------------------------------------------------------------------------- |
| `backend/`       | Go daemon：配置加载、HTTP API、后端服务启动与运行时能力                                |
| `ui/`            | React 前端：健康检查展示、工作流页面与后续交互                                         |
| `desktop/`       | 桌面壳（Wails）：启动/复用 daemon，并在 WebView 内打开 UI                              |
| `scripts/`       | 仓库级脚本：本地开发启动与辅助命令                                                     |
| `.github-feature-analyzer/` | GitHub feature analyzer skill 运行产物目录：按 `{owner-repo}` 隔离源码缓存、分析工件与累计 `report.md` |
| `.codex/skills/` | 项目级 Codex skills：流程规范、分析/实施工作流与协作约束                               |
| `.codex/agents/` | 项目级 Codex 子代理角色配置（按角色覆盖模型/沙箱/指令）                               |
| `.codex/config.toml` | 项目级 Codex 配置覆盖（feature flags、agents 并发与深度等）                      |
| `openspec/`      | OpenSpec 规范管理：基线 specs（系统当前行为真相源）+ changes（变更提案与 delta specs） |

## 3. 当前关键文件索引

| 文件                                       | 作用                                                                                                                                  |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| `AGENTS.md`                                | 仓库级 Agent 协作入口（技能路由、执行流程、交付要求）                                                                                 |
| `.codex/config.toml`                       | 项目级 Codex 配置：开启 `multi_agent`，并声明 `agents.max_threads/max_depth` 与 `explorer` 角色                                      |
| `.codex/agents/explorer.toml`              | `explorer` 子代理角色配置：read-only + 证据优先的实现机理分析指令                                                                     |
| `.codex/skills/github-feature-analyzer/SKILL.md` | GitHub 功能分析 skill 主流程：README-first 仓库特点分析 + MCP-first 拉取 + 分层多代理特性分析 + 父代理汇总与报告渲染          |
| `.codex/skills/github-feature-analyzer/scripts/prepare_workspace.py` | 分析路径准备：统一输出到 `<project-root>/.github-feature-analyzer/{owner-repo}/`（`source`/`artifacts`/`report.md`） |
| `.codex/skills/github-feature-analyzer/scripts/merge_agent_results.py` | 子代理 JSON 结果合并脚本：规范化/去重/冲突与缺失注记后输出单一合并工件                                       |
| `.codex/skills/github-feature-analyzer/scripts/render_report.py` | 原理优先报告渲染：README 特点提炼与实现机制说明 + 五维机制分析（控制流/数据流/状态生命周期/失败恢复/并发时序）+ `file:line` 证据链 |
| `.codex/skills/github-feature-analyzer/scripts/reference_retrieval.py` | 历史分析检索脚本：从 `.github-feature-analyzer/` 下 `report.md + subagent_results.json` 构建本地向量索引，并按查询输出 `compact/semi/full` 参考摘录 |
| `.codex/skills/github-feature-analyzer/scripts/ensure_uv_unix.sh` | UV 引导脚本：在 Linux/macOS 上检测或安装 uv（二进制不入库），为检索脚本提供统一运行前置 |
| `.codex/skills/github-feature-analyzer/scripts/setup_reference_venv.sh` | UV 环境初始化：强制 uv-managed Python 3.12，创建并同步固定 `.venv-reference` |
| `.codex/skills/github-feature-analyzer/scripts/reference_retrieval_uv.sh` | UV 检索入口：先确保环境，再执行 `reference_retrieval.py` 的 `build/query` |
| `backend/cmd/vibe-tree-daemon/main.go`     | daemon 进程入口，负责加载配置、启动 HTTP Server、处理优雅退出                                                                         |
| `backend/internal/server/server.go`        | Gin Engine 装配：恢复中间件、请求日志、dev CORS，并挂载 `internal/api` 路由；可选挂载 UI 静态资源（`ui/dist` 或 `VIBE_TREE_UI_DIST`） |
| `backend/internal/api/api.go`              | HTTP/WS handlers：health、workflow CRUD、execution start/log/cancel、WebSocket 升级入口                                               |
| `backend/internal/api/orchestrations.go`   | Orchestration HTTP handlers：create/list/detail/cancel 与 agent-run retry                                                              |
| `backend/internal/api/chat.go`             | Chat Session API：会话创建/列表/消息查询/多轮发送/附件上传/附件内容预览（JSON + multipart）/手动压缩/分叉/归档（`/api/v1/chat/*`）                |
| `backend/internal/executionflow/runtime.go` | Execution 共享 helper：统一 execution 启动记录、超时上下文与终态摘要/错误信息                                                         |
| `backend/internal/api/info.go`             | 排障信息 API：`GET /api/v1/info`（version + XDG paths）                                                                               |
| `backend/internal/api/experts.go`          | Experts 列表 API：`GET /api/v1/experts`（仅安全字段，供 UI 下拉）                                                                     |
| `backend/internal/api/settings_llm.go`     | 模型设置 API：`GET/PUT /api/v1/settings/llm`（sources/models；key masking；写盘并热更新 experts）                                     |
| `backend/internal/api/settings_experts.go` | 专家设置 API：`GET/PUT /api/v1/settings/experts` 与 `POST /api/v1/settings/experts/generate`（专家详情、保存、AI 生成）              |
| `backend/internal/api/settings_expert_sessions.go` | 专家生成会话 API：session 列表/详情、追加消息、快照发布与继续优化                                                             |
| `backend/internal/api/workflows.go`        | Workflow HTTP handlers：create/list/get/patch，并广播 `workflow.updated`                                                              |
| `backend/internal/api/workflow_start.go`   | Workflow start/nodes handlers：创建 master node + execution，并提供 nodes 查询                                                        |
| `backend/internal/api/workflow_cancel.go`  | Workflow cancel handler：取消 workflow（取消 running execution + 标记未开始节点 canceled）                                            |
| `backend/internal/api/nodes.go`            | Node handlers：patch/retry/cancel（`PATCH /api/v1/nodes/:id`、`POST /api/v1/nodes/:id/retry`、`POST /api/v1/nodes/:id/cancel`）       |
| `backend/internal/config/config.go`        | 配置读取逻辑，处理默认值、XDG 路径、环境变量覆盖                                                                                      |
| `backend/internal/dotenv/dotenv.go`        | dotenv 加载：daemon 启动时从 repo root/指定路径读取 `.env` 并注入到进程环境变量（用于 `${ENV}`）                                      |
| `backend/internal/expert/expert.go`        | Expert 注册表：基于 config 解析 `expert_id` -> RunSpec（`{{prompt}}`/`${ENV}` 模板替换、timeout），并提供已知 expert 集合             |
| `backend/internal/runner/pty_runner.go`    | PTY runner：启动子进程、流式输出、Cancel（SIGTERM→grace→SIGKILL）                                                                     |
| `backend/internal/execution/manager.go`    | Execution 管理：启动/取消、日志落盘、WS 推送 `execution.*`/`node.log`                                                                 |
| `backend/internal/orchestration/manager.go` | Orchestration 管理：goal 拆分首轮 agent runs、并发调度 queued agent run、cancel/retry/synthesis 收敛                                 |
| `backend/internal/chat/manager.go`         | Chat 管理：多轮会话、provider anchor（OpenAI/Anthropic）、附件持久化接入、自动上下文压缩/跳过策略、WS `chat.*` 推送                |
| `backend/internal/chat/attachments.go`      | Chat 附件能力：附件类型校验、大小限制、文件落盘、provider 多模态 block 构造、调试输入摘要                                               |
| `backend/internal/chat/provider_input.go`    | Chat 多模态重建：基于本地消息 + 附件重建 OpenAI/Anthropic provider 输入                                                                |
| `backend/internal/workspace/manager.go`    | Workspace 策略管理：`read_only/shared_workspace/git_worktree` 解析、worktree 分配、代码变更检查与 artifact 生成                      |
| `backend/internal/dag/dag.go`              | DAG 解析与校验：从 master 输出提取第一个 JSON 对象并做 MVP 约束校验（无环/引用存在/expert 校验）                                      |
| `backend/internal/scheduler/scheduler.go`  | Workflow 调度器：依赖 + 并发上限 + fail-fast（启动 queued worker nodes 并收敛终态）                                                   |
| `backend/internal/ws/hub.go`               | WebSocket hub：连接管理与广播（配合 log tail 断线补齐）                                                                               |
| `backend/internal/store/sqlite.go`         | SQLite state DB 打开与 pragma 初始化（WAL/busy_timeout/foreign_keys）                                                                 |
| `backend/internal/store/migrate.go`        | SQLite migrations（使用 `PRAGMA user_version` 管理 schema 版本；含 chat attachments 与 expert builder sessions）                     |
| `backend/internal/store/chat.go`           | Chat 存储：chat sessions/messages/attachments/anchors/compactions 的 SQLite CRUD + hydration                                          |
| `backend/internal/store/orchestrations.go` | Orchestration 存储：SQLite orchestration/round/agent_run/synthesis/artifact CRUD 与详情查询                                           |
| `backend/internal/store/orchestration_executions.go` | Agent-run execution 存储：agent_run_executions 落库、终态收敛、synthesis 生成与 retry/cancel 支撑                        |
| `backend/internal/store/orchestration_recovery.go` | Orchestration 恢复：daemon 重启后把遗留 running agent run 标记为 failed/retryable                                          |
| `backend/internal/store/workflows.go`      | Workflow 存储：SQLite CRUD + events 写入                                                                                              |
| `backend/internal/store/dag.go`            | DAG 落库：从 master DAG 创建 worker nodes/edges，并提供 edges 查询                                                                    |
| `backend/internal/store/nodes.go`          | Node 存储：master node 创建、nodes 列表查询、GetNode、RetryNode（解开 fail-fast skipped 并重试）                                      |
| `backend/internal/store/executions.go`     | Execution 存储：execution started/exited 落库、同步 node/workflow 状态与 events                                                       |
| `backend/internal/store/approval.go`       | Manual approval：runnable `pending_approval` 节点批准为 `queued`                                                                      |
| `backend/internal/store/node_patch.go`     | Node 编辑：PATCH prompt/expert_id，写入 `prompt.updated` 与 `node.updated`                                                            |
| `backend/internal/store/cancel.go`         | Workflow 取消：标记 workflow canceled，并返回需 cancel 的 running executions                                                          |
| `backend/internal/store/failures.go`       | 失败兜底：启动/落库失败时标记 node/workflow failed                                                                                    |
| `backend/internal/store/recovery.go`       | 重启恢复：将 DB 中遗留的 running execution 标记为 failed（daemon_restarted）                                                          |
| `backend/internal/paths/paths.go`          | XDG data/logs/state.db/chat-attachments 路径解析（`~/.local/share/vibe-tree/...`）                                                   |
| `backend/internal/id/id.go`                | ID 生成：`wf_`/`nd_`/`ex_` 前缀 ID（MVP 先用短随机）                                                                                  |
| `backend/internal/logx/logx.go`            | 后端统一日志格式封装（`level=... module=... action=... msg="..."`）                                                                   |
| `backend/internal/version/version.go`      | 版本信息（Commit/BuiltAt，可用 ldflags 注入；用于 `/api/v1/info`）                                                                    |
| `ui/src/App.tsx`                           | 前端入口：daemon health + WS 连接管理 + 路由（`#/orchestrations` 主入口、`#/chat`、隐藏兼容的 legacy workflow 路由）                 |
| `ui/src/app/pages/OrchestrationsPage.tsx`  | Orchestrations 首页：顶部 goal 输入区 + orchestration 列表                                                                            |
| `ui/src/app/pages/OrchestrationDetailPage.tsx` | Orchestration 详情页：按 round 展示并行 agent 卡片、详情面板、日志、artifact、continue/retry/cancel 控制                    |
| `ui/src/app/pages/ChatSessionsPage.tsx`    | Chat 会话页：会话列表、消息流式渲染、发送消息/上传附件、拖拽上传、附件标签与图片/PDF/文本代码预览、手动压缩/分叉/归档                                       |
| `ui/src/app/components/AttachmentPreviewModal.tsx` | Chat 附件预览弹窗：图片/PDF 预览、Markdown 渲染、代码高亮与纯文本展示                                                              |
| `ui/src/lib/chatAttachmentPreview.ts`      | Chat 附件预览判断：按文件后缀/MIME 推断图片/PDF/Markdown/代码/纯文本预览模式                                                         |
| `ui/src/components/DAGView.tsx`            | React Flow DAG 视图：dagre 自动布局 + 节点按状态上色 + 点击节点联动终端                                                               |
| `ui/src/components/TerminalPane.tsx`       | xterm.js 封装组件（fit + write/reset 接口）                                                                                           |
| `ui/src/app/components/LLMSettingsTab.tsx` | 系统设置「模型」Tab：编辑 Sources（base_url+key）与 Models（model+source+SDK），保存后刷新 experts                                    |
| `ui/src/app/components/ExpertSettingsTab.tsx` | 系统设置「专家」Tab：专家列表、AI 生成专家、生成会话历史、快照发布                                                                   |
| `ui/src/lib/daemon.ts`                     | daemon URL/WS URL 解析与 health/workflow/execution/chat attachment API 封装                                                           |
| `ui/src/stores/chatStore.ts`               | Chat 前端状态：sessions/messages/streaming/sending 状态与 chat API actions                                                            |
| `scripts/dev.sh`                           | 本地开发一键启动脚本（并行拉起 backend 与 UI）                                                                                        |
| `backend/.air.toml`                        | 后端热重载配置（Air）：本地开发时监听 Go 源码变更并自动重建/重启 daemon                                                                |
| `scripts/web.sh`                           | Web 单进程启动脚本（构建 UI 并由 daemon 静态托管 `ui/dist`）                                                                          |
| `desktop/main.go`                          | Wails 桌面入口：嵌入 `frontend/src`，注册 Menu，并 Bind `App` 方法供前端调用                                                          |
| `desktop/app.go`                           | Desktop 业务逻辑：解析 daemon host/port，确保 daemon 可用（必要时子进程拉起），并提供“打开数据目录”等动作                             |

## 4. 功能定位索引（关键词 -> 文件）

| 关键词                             | 优先查看文件                                                                                                                                                                                                                               |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| daemon 启动入口                    | `backend/cmd/vibe-tree-daemon/main.go`                                                                                                                                                                                                     |
| 健康检查 API                       | `backend/internal/api/api.go`                                                                                                                                                                                                              |
| daemon info API                    | `backend/internal/api/info.go`, `ui/src/lib/daemon.ts`, `ui/src/App.tsx`                                                                                                                                                                   |
| experts 列表 API                   | `backend/internal/api/experts.go`, `backend/internal/expert/expert.go`, `ui/src/lib/daemon.ts`, `ui/src/App.tsx`                                                                                                                           |
| dev CORS 配置                      | `backend/internal/server/server.go`                                                                                                                                                                                                        |
| 请求日志格式                       | `backend/internal/server/server.go`                                                                                                                                                                                                        |
| UI 静态资源挂载（ui/dist）         | `backend/internal/server/server.go`, `scripts/web.sh`                                                                                                                                                                                      |
| daemon 地址默认值                  | `backend/internal/config/config.go`, `ui/src/lib/daemon.ts`                                                                                                                                                                                |
| dotenv/.env 自动加载               | `backend/internal/dotenv/dotenv.go`, `backend/cmd/vibe-tree-daemon/main.go`                                                                                                                                                                |
| UI 运行时切换 daemon URL           | `ui/src/App.tsx`                                                                                                                                                                                                                           |
| 模型设置（Sources/Models）         | `backend/internal/api/settings_llm.go`, `backend/internal/config/llm_settings.go`, `backend/internal/config/llm_mirror.go`, `ui/src/app/components/SettingsDialog.tsx`, `ui/src/app/components/LLMSettingsTab.tsx`, `ui/src/lib/daemon.ts` |
| 专家设置 / AI 创建专家             | `backend/internal/api/settings_experts.go`, `backend/internal/api/settings_expert_sessions.go`, `backend/internal/expertbuilder/service.go`, `backend/internal/skillcatalog/catalog.go`, `.codex/skills/expert-creator/SKILL.md`, `ui/src/app/components/ExpertSettingsTab.tsx`, `ui/src/lib/daemon.ts` |
| XDG 配置路径                       | `backend/internal/config/config.go`                                                                                                                                                                                                        |
| XDG 日志路径                       | `backend/internal/paths/paths.go`                                                                                                                                                                                                          |
| Expert 配置/模板解析               | `backend/internal/config/config.go`, `backend/internal/expert/expert.go`                                                                                                                                                                   |
| execution timeout 语义             | `backend/internal/execution/manager.go`, `backend/internal/scheduler/scheduler.go`                                                                                                                                                         |
| SQLite state.db 初始化             | `backend/cmd/vibe-tree-daemon/main.go`, `backend/internal/store/sqlite.go`, `backend/internal/store/migrate.go`                                                                                                                            |
| Chat Session API                   | `backend/internal/api/chat.go`, `backend/internal/chat/manager.go`, `backend/internal/chat/attachments.go`, `backend/internal/store/chat.go`, `ui/src/lib/daemon.ts`                                                                                |
| Chat 自动上下文压缩               | `backend/internal/chat/manager.go`, `backend/internal/store/chat.go`                                                                                                                                                                                           |
| Chat provider anchor 续上下文      | `backend/internal/chat/manager.go`, `backend/internal/store/chat.go`                                                                                                                                                                                           |
| Chat 附件上传与多模态输入          | `backend/internal/api/chat.go`, `backend/internal/chat/attachments.go`, `backend/internal/chat/provider_input.go`, `backend/internal/store/chat.go`, `ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/lib/daemon.ts`                                 |
| Chat 附件预览与拖拽上传            | `backend/internal/api/api.go`, `backend/internal/api/chat.go`, `backend/internal/store/chat.go`, `ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/app/components/AttachmentPreviewModal.tsx`, `ui/src/lib/chatAttachmentPreview.ts`, `ui/src/lib/daemon.ts` |
| Orchestration 列表/创建 UI         | `ui/src/App.tsx`, `ui/src/app/pages/OrchestrationsPage.tsx`, `ui/src/lib/daemon.ts`                                                                                                                                                      |
| Orchestration 详情 UI              | `ui/src/App.tsx`, `ui/src/app/pages/OrchestrationDetailPage.tsx`, `ui/src/lib/daemon.ts`, `ui/src/lib/ws.ts`                                                                                                                             |
| Legacy Workflows 隐藏兼容入口      | `ui/src/app/routes.ts`, `ui/src/app/pages/WorkflowsPage.tsx`, `ui/src/app/pages/WorkflowDetailPage.tsx`                                                                                                                                   |
| Orchestration API                  | `backend/internal/api/orchestrations.go`, `backend/internal/orchestration/manager.go`, `backend/internal/store/orchestrations.go`, `backend/internal/store/orchestration_executions.go`                                                   |
| Orchestration continue / retry / cancel | `backend/internal/api/orchestrations.go`, `backend/internal/orchestration/manager.go`, `backend/internal/store/orchestration_updates.go`, `ui/src/app/pages/OrchestrationDetailPage.tsx`                                             |
| Workspace / Worktree 策略         | `backend/internal/workspace/manager.go`, `backend/internal/store/orchestration_updates.go`, `backend/internal/store/orchestration_executions.go`                                                                                           |
| Workflow CRUD API                  | `backend/internal/api/workflows.go`, `backend/internal/store/workflows.go`                                                                                                                                                                 |
| Workflow Start（master 执行）      | `backend/internal/api/workflow_start.go`, `backend/internal/store/nodes.go`, `backend/internal/store/executions.go`                                                                                                                        |
| Workflow Nodes API                 | `backend/internal/api/workflow_start.go`, `backend/internal/store/nodes.go`                                                                                                                                                                |
| 重启恢复（running→failed）         | `backend/cmd/vibe-tree-daemon/main.go`, `backend/internal/store/recovery.go`                                                                                                                                                               |
| execution 启动                     | `backend/internal/api/api.go`, `backend/internal/execution/manager.go`                                                                                                                                                                     |
| execution 日志落盘                 | `backend/internal/execution/manager.go`                                                                                                                                                                                                    |
| execution log tail API             | `backend/internal/api/api.go`, `backend/internal/execution/logtail.go`                                                                                                                                                                     |
| WebSocket 推送                     | `backend/internal/api/api.go`, `backend/internal/ws/hub.go`, `backend/internal/ws/envelope.go`                                                                                                                                             |
| PTY runner                         | `backend/internal/runner/pty_runner.go`                                                                                                                                                                                                    |
| execution cancel API               | `backend/internal/api/api.go`, `backend/internal/execution/manager.go`                                                                                                                                                                     |
| DAG JSON 提取/校验                 | `backend/internal/dag/dag.go`, `backend/internal/api/workflow_start.go`                                                                                                                                                                    |
| DAG 落库（nodes/edges）            | `backend/internal/store/dag.go`, `backend/internal/api/workflow_start.go`                                                                                                                                                                  |
| 调度器（依赖/并发/fail-fast）      | `backend/internal/scheduler/scheduler.go`, `backend/cmd/vibe-tree-daemon/main.go`, `backend/internal/store/scheduler.go`                                                                                                                   |
| manual approve API                 | `backend/internal/api/approve.go`, `backend/internal/store/approval.go`                                                                                                                                                                    |
| node patch API                     | `backend/internal/api/nodes.go`, `backend/internal/store/node_patch.go`                                                                                                                                                                    |
| node retry/cancel API              | `backend/internal/api/nodes.go`, `backend/internal/store/nodes.go`, `backend/internal/scheduler/scheduler.go`                                                                                                                              |
| workflow mode 切换（断点）         | `backend/internal/api/workflows.go`, `backend/internal/store/workflows.go`                                                                                                                                                                 |
| workflow cancel API                | `backend/internal/api/workflow_cancel.go`, `backend/internal/store/cancel.go`                                                                                                                                                              |
| ID 生成规则                        | `backend/internal/id/id.go`                                                                                                                                                                                                                |
| 后端统一日志格式                   | `backend/internal/logx/logx.go`, `backend/internal/server/server.go`                                                                                                                                                                       |
| 前端连通性提示                     | `ui/src/App.tsx`                                                                                                                                                                                                                           |
| Workflow 列表/创建 UI              | `ui/src/App.tsx`, `ui/src/lib/daemon.ts`                                                                                                                                                                                                   |
| Workflow Kanban/详情 UI            | `ui/src/App.tsx`, `ui/src/lib/daemon.ts`, `ui/src/components/TerminalPane.tsx`                                                                                                                                                             |
| Workflow DAG 视图（React Flow）    | `ui/src/components/DAGView.tsx`, `ui/src/App.tsx`                                                                                                                                                                                          |
| health/workflow/execution API 封装 | `ui/src/lib/daemon.ts`                                                                                                                                                                                                                     |
| 终端渲染与路由                     | `ui/src/components/TerminalPane.tsx`, `ui/src/App.tsx`                                                                                                                                                                                     |
| Chat 会话 UI                       | `ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/stores/chatStore.ts`, `ui/src/App.tsx`, `ui/src/app/components/Topbar.tsx`                                                                                                                       |
| 本地一键启动                       | `scripts/dev.sh`                                                                                                                                                                                                                           |
| Web 单进程启动（daemon 托管 UI）   | `scripts/web.sh`, `backend/internal/server/server.go`                                                                                                                                                                                      |
| UI 开发端口代理与构建配置          | `ui/vite.config.ts`, `ui/package.json`                                                                                                                                                                                                     |
| desktop 拉起/复用 daemon           | `desktop/app.go`, `desktop/frontend/src/main.js`                                                                                                                                                                                           |
| desktop 打开数据目录               | `desktop/main.go`, `desktop/app.go`                                                                                                                                                                                                        |
| Agent 协作与 skill 路由            | `AGENTS.md`                                                                                                                                                                                                                                |
| Codex 多代理角色配置               | `.codex/config.toml`, `.codex/agents/explorer.toml`                                                                                                                                                                                      |
| GitHub 仓库功能机理分析 skill      | `.codex/skills/github-feature-analyzer/SKILL.md`, `.codex/skills/github-feature-analyzer/references/report-schema.md`, `.codex/skills/github-feature-analyzer/scripts/render_report.py`                                               |
| GitHub feature analyzer 产物目录规范 | `.codex/skills/github-feature-analyzer/scripts/prepare_workspace.py`, `.github-feature-analyzer/`                                                                                                                               |
| GitHub 历史分析检索（向量索引） | `.codex/skills/github-feature-analyzer/scripts/reference_retrieval_uv.sh`, `.codex/skills/github-feature-analyzer/scripts/setup_reference_venv.sh`, `.codex/skills/github-feature-analyzer/scripts/reference_retrieval.py`, `.codex/skills/github-feature-analyzer/scripts/requirements-vector.lock.txt`, `.github-feature-analyzer/` |
| OpenSpec 基线规范（workflow）      | `openspec/specs/workflow/spec.md`                                                                                                                                                                                                          |
| OpenSpec 基线规范（dag）           | `openspec/specs/dag/spec.md`                                                                                                                                                                                                               |
| OpenSpec 基线规范（scheduler）     | `openspec/specs/scheduler/spec.md`                                                                                                                                                                                                         |
| OpenSpec 基线规范（execution）     | `openspec/specs/execution/spec.md`                                                                                                                                                                                                         |
| OpenSpec 基线规范（experts）       | `openspec/specs/experts/spec.md`                                                                                                                                                                                                           |
| OpenSpec 基线规范（store）         | `openspec/specs/store/spec.md`                                                                                                                                                                                                             |
| OpenSpec 基线规范（ui）            | `openspec/specs/ui/spec.md`                                                                                                                                                                                                                |
| OpenSpec 变更提案                  | `openspec/changes/`                                                                                                                                                                                                                        |

## 5. 维护规则

1. 新增核心模块、重命名目录或职责变化时，必须同步更新本文件的“根目录职责表”。
2. 新增关键入口文件时，必须补充到“当前关键文件索引”。
3. 新增或调整功能落点时，必须补充或修订“功能定位索引（关键词 -> 文件）”。
4. 功能定位优先使用本文件提供的候选路径，再在目标目录内使用 `rg` 做定向检索。
