# vibe-tree MVP 实现 TodoList（细化到可直接照做）

> 详细规格/架构/协议：`docs/规划.md`

## 0. 约定（先定死，避免返工）
- [ ] 约定仓库结构（MVP）：`backend/`（Go daemon）、`ui/`（React SPA）、`desktop/`（Wails 壳）
- [ ] 约定后端技术栈：Go 1.22+、Gin、gorilla/websocket、SQLite（WAL）、PTY（creack/pty）
- [ ] 约定前端技术栈：Vite + React + TS、Tailwind + shadcn/ui、Zustand、React Flow + dagre、xterm.js
- [ ] 约定数据目录（Ubuntu/XDG）：
  config：`~/.config/vibe-tree/config.json`
  data：`~/.local/share/vibe-tree/`
  sqlite：`~/.local/share/vibe-tree/state.db`
  logs：`~/.local/share/vibe-tree/logs/`
- [ ] 约定 ID 规则（便于日志与排障）：workflow `wf_...`、node `nd_...`、execution `ex_...`（后端生成）
- [ ] 约定状态枚举（先用 MVP 子集，后续再扩）：
  workflow：`todo` `running` `done` `failed` `canceled`
  node：`draft` `pending_approval` `queued` `running` `succeeded` `failed` `canceled` `skipped`
  execution：`queued` `running` `succeeded` `failed` `canceled` `timeout`
- [ ] 约定 WS envelope（字段名对齐 `docs/规划.md` 8.2）：`type` `ts` `workflow_id` `node_id` `execution_id` `payload`

验收：
- [ ] `docs/规划.md` 里 8.x/9.x 的协议点，在本文件里都能找到对应的落地步骤

## Phase 0：仓库与骨架（0.5–1 天）

### 0.1 目录与工程初始化
- [x] 创建目录：`backend/`、`ui/`、`desktop/`
- [x] 初始化后端 Go module：`backend/go.mod`
  当前 module path：`vibe-tree/backend`（后续如需发布可改为 `github.com/<you>/vibe-tree/backend` 或 `github.com/<you>/vibe-tree`）
- [x] 初始化前端工程：`ui/package.json`（Vite React TS）
- [x] （可选）加一个统一入口脚本：`scripts/dev.sh`（并行启动 backend/ui）

验收：
- [x] `backend/` 下能 `go test ./...`（即使只有空包也应通过）
- [x] `ui/` 下能 `pnpm dev` 或 `npm run dev` 启动

### 0.2 Daemon 最小可运行（Health + 静态配置）
- [x] 新增 `backend/cmd/vibe-tree-daemon/main.go`
  要点：读取 config（默认 host=127.0.0.1 port=7777）并启动 HTTP
- [x] 新增路由：`GET /api/v1/health` 返回 `{ "ok": true }`
- [x] 新增中间件：请求日志（method/path/status/latency）
- [x] 开发期 CORS（仅 dev）：允许 `http://127.0.0.1:*` / `http://localhost:*`

验收：
- [x] curl `GET /api/v1/health` 返回 OK

### 0.3 UI 最小可运行（空态页面 + 能打到 health）
- [x] 新增页面：`/`（Workflows 列表空态）
- [x] 在 UI 启动时请求 `GET /api/v1/health`，失败则显示明确错误（daemon 未启动/端口不对）

验收：
- [x] UI 页面可打开；daemon 关闭时有可读错误提示

## Phase 1：PTY → WS → xterm 链路（3–5 天）

### 1.1 Runner 抽象与 PTY 实现
- [x] 定义接口：`Runner.Start(ctx, cmd, args, env, cwd) -> handle`，`handle.Cancel()`，`handle.Wait()`
- [x] 实现 PTY runner（Ubuntu 优先）：使用 `github.com/creack/pty`
- [x] 约定取消策略：先 SIGTERM，等待 `kill_grace_ms`，再 SIGKILL
- [x] 采集退出信息：exit code、signal（如果有）、start/end 时间

验收：
- [x] 能用 runner 拉起一个持续输出的命令（例如 `bash -lc 'for i in {1..1000}; do echo $i; sleep 0.1; done'`）

### 1.2 日志落盘（每个 execution 一个文件）
- [x] 定义 logfile 路径：`~/.local/share/vibe-tree/logs/<execution_id>.log`
- [x] runner 输出写入文件（append），并定期 flush（例如 100ms 或按行）
- [x] 新增 API：`GET /api/v1/executions/{id}/log?tail=2000`
  约定：`tail` 以字节为单位（和 `docs/规划.md` 一致），返回纯文本（UTF-8，包含 ANSI）

验收：
- [x] 进程运行期间 logfile 持续增长；daemon 重启后仍可读取 tail

### 1.3 WebSocket 推送增量日志
- [x] 新增 WS：`GET /api/v1/ws`
- [x] 实现事件广播器（最小版）：维护连接列表，支持按 workflow 过滤（先全量广播也可）
- [x] 推送事件：
  `execution.started`（开始时发一次）
  `node.log`（持续 chunk）
  `execution.exited`（结束时发一次）
- [x] `node.log` payload 形如：`{ "chunk": "..." }`

验收：
- [x] UI 连接 WS 后能实时看到输出；断线后可通过 log tail 补齐

### 1.4 UI xterm.js 渲染 + 多终端切换
- [x] 引入 xterm.js（fit addon）
- [x] 实现 TerminalPane：
  初始化终端实例
  订阅 WS `node.log`
  按 execution_id/node_id 做路由（避免写错窗口）
- [x] 实现最小的“终端列表”UI：先支持单个 execution，后扩到多 pane

验收：
- [x] ANSI 彩色正常显示（例如 `\x1b[32m` 绿字）
- [ ] 3 个并发 execution 同时输出，UI 不冻结

### 1.5 Cancel 端到端
- [x] 新增 API：`POST /api/v1/executions/{id}/cancel`（MVP 先用该路径；Phase 2 再按 node/workflow 收敛）
- [x] Cancel 后：
  runner 终止进程
  更新 execution 状态
  WS 推送 `execution.exited`（带 canceled）

验收：
- [x] Cancel 后 1–2s 内进程结束；状态与 UI 一致

## Phase 2：Workflow/Node/Execution 元数据闭环（4–7 天）

### 2.1 SQLite 初始化与迁移
- [x] 引入 SQLite（建议 `mattn/go-sqlite3` 或 `modernc.org/sqlite`，二选一）
- [x] 启用 WAL + busy_timeout；设置 `db.SetMaxOpenConns(1)`（MVP 先保守）
- [x] 建立 migrations（或 GORM AutoMigrate）：创建表
  `workflows` `nodes` `edges` `executions` `events`
  字段参考：`docs/规划.md` 7.2

验收：
- [x] daemon 启动会自动创建 sqlite 文件与表

### 2.2 Workflow CRUD（先不做 DAG）
- [x] `POST /api/v1/workflows`：创建 workflow（title/workspace/mode）
- [x] `GET /api/v1/workflows`：列表（含状态、更新时间、mode）
- [x] `GET /api/v1/workflows/{id}`：详情（含配置与汇总字段）
- [x] `PATCH /api/v1/workflows/{id}`：更新 title/workspace/mode
- [x] 每次变更写 events：`workflow.updated`

验收：
- [x] UI 首页能创建 workflow 并在列表中看到

### 2.3 Node/Execution 模型与 Start（创建 master 执行）
- [ ] `POST /api/v1/workflows/{id}:start`：
  创建 master node（type=master）
  创建 execution（status=running）
  通过 runner 启动 master expert（先用 `bash` stub 也行）
  workflow 状态置为 `running`
- [ ] `GET /api/v1/workflows/{id}/nodes`：返回 nodes（先只有 master）
- [ ] execution 结束后：写回 executions 表（exit_code/duration/status），写 events

验收：
- [ ] 点击 Start 后 UI 能看到 master 的终端输出与结束状态

### 2.4 重启恢复（最小版）
- [ ] daemon 启动时扫描 DB：
  将 `executions.status=running` 标为 `failed`（原因=daemon_restarted）或 `canceled`（二选一，建议 failed）
  对应 node/workflow 状态也做一致性修正
- [ ] UI 加一个“Retry”入口（先只支持 node retry）

验收：
- [ ] 手动 kill daemon 后重启，workflow 仍可打开；历史日志可 tail；running 的 execution 不会卡死在 running

### 2.5 UI：Kanban + 详情页骨架
- [ ] 首页 Kanban（最小版可以先用 4 列分组列表，不强制拖拽）：Todo/Running/Done/Failed
- [ ] 详情页 `/workflows/:id`：
  左：先放 nodes 列表（后续再换 DAG）
  右：TerminalPane（显示选中 node 的最新 execution 日志）

验收：
- [ ] 从首页进入详情页，能切换 node 并查看日志

## Phase 3：DAG 生成与 Manual approval（5–9 天）

### 3.1 定义 DAG JSON schema + 校验器
- [ ] 定义 master 输出 schema（按 `docs/规划.md` 9）：nodes/edges + expert_id/prompt 等
- [ ] 实现严格 JSON 解析策略：
  尝试从 stdout 提取第一个 JSON 对象（允许外层有解释文本）
  解析失败时：把原始输出摘要写入 node result_summary，并在 UI 展示错误
- [ ] 校验规则（MVP 必做）：
  无环
  node.id 唯一
  edges 引用的节点必须存在
  expert_id 必须存在（否则直接 reject 或 fallback，二选一；建议先 reject）

验收：
- [ ] 给一份坏 DAG（有环/缺节点/未知 expert），后端能给出可读错误

### 3.2 master 输出落库为 nodes/edges
- [ ] master execution 结束后：
  解析 DAG
  创建 worker nodes（status 取决于 mode：auto=queued 或 manual=pending_approval）
  创建 edges
  写 events：`dag.generated`
- [ ] `workflow_title` 可覆盖 workflow.title（可选；建议 UI 提示用户确认再改）

验收：
- [ ] UI 在 DAG 生成后能看到节点列表变成多节点

### 3.3 调度器（依赖 + 并发 + fail-fast）
- [ ] 实现可重复运行的调度循环（例如每 200ms tick 或事件驱动）：
  找到 runnable nodes（依赖全部 succeeded）
  mode=auto 时把 runnable 推进 queued→running
  mode=manual 时把 runnable 推进 pending_approval（不自动跑）
- [ ] 并发上限：从 config `execution.max_concurrency` 读取
- [ ] fail-fast：任一 node failed 后，把未开始的节点标记为 skipped（或留在 pending，二选一；建议 skipped）

验收：
- [ ] 同一个 DAG 能按依赖顺序自动推进；并发数不会超过上限

### 3.4 Manual approval：审批与编辑
- [ ] `PATCH /api/v1/nodes/{id}`：允许修改 `prompt` 与 `expert_id`
  修改写 events：`prompt.updated`（payload 含 old/new 摘要）
- [ ] `POST /api/v1/workflows/{id}:approve`：
  仅在 mode=manual 生效
  将所有 runnable 且 `pending_approval` 的 nodes 推到 queued（等待调度执行）
- [ ] `PATCH /api/v1/workflows/{id}` 支持运行中切换 mode（Execution Breakpoint Toggle）：
  auto→manual：阻断后续启动，把 runnable/queued 的未运行节点拦为 pending_approval
  manual→auto：恢复调度，把 runnable 的 pending_approval 推进 queued

验收：
- [ ] 手动修改 prompt 后 approve，执行使用的是新 prompt
- [ ] 运行中切 manual 能拦截后续节点不启动

### 3.5 UI：React Flow DAG + 节点联动终端
- [ ] 引入 React Flow + dagre 自动布局
- [ ] DAG 节点样式：按 status 上色（running 黄、succeeded 绿、failed 红、pending 灰）
- [ ] 点击节点：右侧切换到对应 TerminalPane（高亮标题）
- [ ] manual 模式下：节点侧栏编辑 prompt/expert，保存即调用 PATCH
- [ ] 顶部控制条：
  execution breakpoint toggle（mode 切换）
  Approve all runnable（仅 manual）
  Cancel workflow（调用 `POST /api/v1/workflows/{id}:cancel`）

验收：
- [ ] 10 节点 DAG：审批后按依赖并发执行；点击节点能准确看到对应日志

## Phase 4：接入真实 AI Expert（3–10 天）

### 4.1 Expert 配置系统（config.json）
- [ ] 定义并实现 `config.json` 读取（参考 `docs/规划.md` 5.1）：
  server.host/port
  execution.max_concurrency/kill_grace_ms
  experts[]（id/label/run_mode/command/args/env/timeout_ms）
- [ ] env 模板替换：支持 `${OPENAI_API_KEY}` 这种从环境变量注入
- [ ] prompt 模板替换：支持 `{{prompt}}` 传参（至少 args 能用）

验收：
- [ ] 未配置时使用内置默认（至少 bash）
- [ ] 配置错误时给出可读错误（缺字段/未知 run_mode）

### 4.2 bash expert（作为兜底与可测基线）
- [ ] 内置 `bash` expert：`bash -lc "{{prompt}}"`
- [ ] node 的 prompt 直接当 shell script 执行（MVP 先这么做）

验收：
- [ ] worker 节点能执行真实命令（例如 `ls`, `rg`, `go test`）并把输出实时显示

### 4.3 codex（或你常用 AI CLI）expert
- [ ] 新增 `codex` expert（oneshot）：把 prompt 传给 CLI，收集 stdout/stderr
- [ ] master 节点专用 prompt 模板：强制输出 DAG JSON（严格 JSON）
- [ ] 失败处理：
  超时 -> execution.timeout
  非零退出码 -> execution.failed（保存 stderr 摘要到 result_summary）

验收：
- [ ] 用 codex 生成 DAG；daemon 校验通过后自动落库并进入 manual pending

### 4.4 Retry/Cancel 完整闭环（按 node 粒度）
- [ ] `POST /api/v1/nodes/{id}:retry`：创建新的 execution（保留历史）
- [ ] `POST /api/v1/nodes/{id}:cancel`：取消当前 running execution
- [ ] UI：failed/canceled 节点显示 Retry；running 节点显示 Cancel

验收：
- [ ] 同一 node 多次 retry，能在 UI 里查看每次 execution 的状态与日志

## Phase 5：桌面壳与发布（3–6 天）

### 5.1 Wails 工程初始化
- [ ] 初始化 Wails v2 工程：`desktop/`
- [ ] desktop 启动时拉起 daemon（子进程或内嵌同进程，二选一；MVP 建议子进程）
- [ ] desktop WebView 打开 `http://127.0.0.1:<port>/`（开发态也可先直连 `ui dev server`）

验收：
- [ ] 双击启动 desktop 后，能看到 UI 且 health 正常

### 5.2 生命周期与数据入口
- [ ] 约定关闭窗口行为（MVP 先简单：关闭即退出并停止 daemon）
- [ ] 提供“打开数据目录”入口（至少在菜单里）

验收：
- [ ] 用户能快速定位 sqlite 与 logs 文件位置

## 测试（贯穿，按最低可用集先落地）
- [ ] 状态机单测：workflow/node/execution 的合法/非法转移
- [ ] DAG 校验单测：无环、缺节点、未知 expert、依赖未满足
- [ ] Runner 集成测：启动、cancel、超时、kill_grace
- [ ] SQLite 压测：高频状态更新不出现 `database is locked`（WAL + busy_timeout + 连接策略）
- [ ] WS 断线重连：UI 重连后继续接收日志；缺失部分通过 tail 补齐
