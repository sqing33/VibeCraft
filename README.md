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

## 配置与路径

- 配置文件：`~/.config/vibe-tree/config.json`
- 数据目录：`~/.local/share/vibe-tree/`
  - SQLite：`state.db`
  - Logs：`logs/<execution_id>.log`

UI 首页的 **Daemon** 面板会显示这些路径（支持 Copy），也可以直接访问：
- `GET /api/v1/info`
- `GET /api/v1/experts`

## 常用环境变量

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

仓库包含 `desktop/`（Wails 壳），但当前推荐先以 Web 形态完成 UI/功能闭环；desktop 验收项见 `docs/plan.md` Phase 5。

