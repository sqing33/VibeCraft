# vibe-tree

`vibe-tree` 是一个本地优先的 DAG workflow 执行器：Go daemon 提供 HTTP/WS API（SQLite 状态 + PTY runner），React SPA 通过 WebSocket 实时渲染节点日志与工作流状态。

## Quickstart（Web 优先）

### 方式 A：开发模式（backend + UI dev server）

```bash
./scripts/dev.sh
```

默认 daemon：`http://127.0.0.1:7777`  
默认 UI dev server：Vite 输出的本地地址（终端会打印）。

### 方式 B：Web 单进程（daemon 静态托管 `ui/dist`）

```bash
./scripts/web.sh
```

然后打开：`http://127.0.0.1:7777/`

说明：
- `./scripts/web.sh` **默认会先构建 UI（`cd ui && npm run build`）**，避免复用旧的 `ui/dist` 导致“看到的还是旧页面”。
- 如需跳过构建（复用现有 dist），可用：
  ```bash
  VIBE_TREE_SKIP_UI_BUILD=1 ./scripts/web.sh
  ```

## 配置与路径

- 配置文件：`~/.config/vibe-tree/config.json`
- 数据目录：`~/.local/share/vibe-tree/`
  - SQLite：`state.db`
  - Logs：`logs/<execution_id>.log`

UI 右上角 **系统设置** 会显示这些路径与诊断信息（支持复制），也可以直接访问：
- `GET /api/v1/info`
- `GET /api/v1/experts`

## UI 功能（当前已实现）

- 顶栏：健康状态、连接状态、明暗主题切换（默认浅色，持久化）、入口（开发工具/系统设置）
- 工作流首页（看板）：四列（待开始/进行中/已完成/失败），新建、启动、高级启动、刷新
- 工作流详情：模式切换（手动/自动）、手动批准可运行节点、取消工作流、节点检查器（编辑专家/提示词并保存）、节点取消/重试、DAG 视图联动终端、日志终端
- 系统设置：守护进程地址切换（持久化）、版本/路径/专家诊断信息（支持复制）
- 开发工具：示例执行（启动/取消）、执行列表、日志回放（仅开发模式可见）

## 主题与中文化

- UI 默认使用简体中文。
- 主题默认浅色；可在顶栏切换浅色/深色主题。
- 主题与守护进程地址会持久化到浏览器本地存储：
  - 主题：`vibe-tree.theme`（`light|dark`）
  - 守护进程地址：`vibe-tree.daemon_url`

## 怎么测试（推荐流程）

### 1) 前端构建（必做）

```bash
cd ui && npm run build
```

### 2) 开发模式联调（必做）

```bash
./scripts/dev.sh
```

打开 Vite 输出的本地地址（终端会打印，通常类似 `http://localhost:5174/`）。

### 3) 生产形态验证（必做）

```bash
./scripts/web.sh
```

打开 `http://127.0.0.1:7777/`，并验证“开发工具”入口在生产形态不可见（仍保留“系统设置”）。

### 4) 后端单测（可选但推荐）

```bash
cd backend && go test ./...
```

说明：Go module 位于 `backend/`，从仓库根目录执行 `go test ./...` 不适用。

## 手动验收清单

- 主题：首次打开默认浅色；切换深色立即生效；刷新后仍保持选择；浅/深色下文字与终端可读
- 中文化：工作流首页/详情/系统设置/开发工具等主流程可见文案为中文
- 环境差异：开发模式可见“开发工具”；生产构建（`ui/dist` 静态托管）隐藏“开发工具”
- 主链路回归：新建工作流出现在看板；启动进入详情；健康状态与连接状态可见且会更新

## 开发工具（开发模式可见）

UI 顶栏的“开发工具”（包含示例执行）仅在开发模式（Vite dev server，`import.meta.env.DEV === true`）可见。  
当 UI 以生产构建（`ui/dist`）由守护进程静态托管时，“开发工具”默认隐藏。

## 常用环境变量

- dotenv（开发环境）：
  - 在仓库根目录创建 `.env`（参考 `.env.example`），daemon 启动时会自动加载并写入进程环境变量（用于 `${ENV}` 注入，如 `ANTHROPIC_API_KEY`）。
  - **覆盖语义**：`.env` 会覆盖已存在的同名环境变量。
  - 禁用：`VIBE_TREE_DOTENV=0`
  - 指定路径：`VIBE_TREE_DOTENV_PATH=/path/to/.env`

- `VIBE_TREE_HOST` / `VIBE_TREE_PORT`：覆盖监听地址
- `VIBE_TREE_MAX_CONCURRENCY`：调度并发上限
- `VIBE_TREE_KILL_GRACE_MS`：Cancel grace（SIGTERM→SIGKILL）
- `VIBE_TREE_ENV=dev|development`：启用 dev CORS
- `VIBE_TREE_UI_DIST`：指定静态 UI dist 目录（默认会尝试 `./ui/dist` 与 `../ui/dist`）

前端：
- `VITE_DAEMON_URL`：构建期覆盖 daemon URL（通常不需要；生产静态托管默认同源；UI 也支持运行时切换）

## 版本信息（可选）

daemon 支持通过 ldflags 注入版本信息（用于 `/api/v1/info` 展示）：

```bash
cd backend
go build -ldflags "\
  -X vibe-tree/backend/internal/version.Commit=$(git rev-parse --short HEAD) \
  -X vibe-tree/backend/internal/version.BuiltAt=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o vibe-tree-daemon ./cmd/vibe-tree-daemon
```

## Desktop

仓库包含 `desktop/`（Wails 壳），但当前推荐先以 Web 形态完成 UI/功能闭环。
