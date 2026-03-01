# Tasks: ui-productionize

## 1. Bootstrap

- [x] 1.1 为 `ui/` 引入 Tailwind CSS（postcss + tailwind config），并收敛 `ui/src/index.css` 的全局样式冲突
- [x] 1.2 引入 shadcn/ui 基础设施（`components.json`、`ui/src/lib/utils.ts`、基础组件 Button/Dialog/Input/Select/Badge）
- [x] 1.3 引入 Zustand，并建立最小 store 目录结构（daemon/workflow/execution 三个 store 的骨架）
- [x] 1.4 验证：`cd ui && npm run build` 通过；`./scripts/dev.sh` 可启动

## 2. App Shell & Settings

- [x] 2.1 新建 App Shell（Topbar + Content），展示 Health/WS 状态与 Settings 入口（替代当前大块 Daemon 面板）
- [x] 2.2 将 daemon URL 切换、info(paths/version)、experts 列表迁移到 Settings Drawer/Modal（默认收起）
- [x] 2.3 统一错误提示为 Toast/Alert，并为关键请求提供 loading skeleton（替代 scattered 的 errorBox）

## 3. Workflows Kanban

- [x] 3.1 重做 Workflows 首页：Kanban 四列布局（Todo/Running/Done/Failed），卡片视觉与信息层级按 `openspec/specs/ui/spec.md` 对齐
- [x] 3.2 实现 `+ New`：用 Dialog 创建 workflow（title/workspace/mode），调用 `POST /api/v1/workflows`
- [x] 3.3 实现 Start：Todo card 提供一键 Start，并提供 Advanced（master expert/prompt）调用 `POST /api/v1/workflows/{id}/start`
- [x] 3.4 卡片补齐“last updated time（人类可读）”；（可选）展示 running 节点数 badge（若后端提供聚合字段）

## 4. Workflow Detail

- [x] 4.1 拆分路由：`/` 与 `#/workflows/:id` 两页（保留 hash 路由），并从 Kanban 卡片进入详情页
- [x] 4.2 详情页布局：左侧 DAG（沿用 `DAGView` 做视觉适配），右侧 Inspector + Terminal（xterm）
- [x] 4.3 Run controls：mode toggle、Approve runnable（manual only）、Cancel workflow（running only），并完善禁用态/错误态
- [x] 4.4 Node inspector：展示 node 基本信息；在允许状态下支持编辑 expert/prompt 并保存（`PATCH /api/v1/nodes/{id}`）
- [x] 4.5 Terminal 日志：WS 实时写入 + 断线/切换时 log tail 补齐；为“没有 execution”提供明确空态

## 5. Cleanup & Production Build

- [x] 5.1 将 oneshot execution（含 demo）迁移到 DevTools，并在生产构建默认隐藏（dev 可见）
- [x] 5.2 （可选）后端补齐 Kanban 需要的聚合字段（例如 running 节点数），并在 UI 上显示（向后兼容新增字段）
- [x] 5.3 修复 `scripts/web.sh`：默认构建 UI（避免旧 dist），并提供 `VIBE_TREE_SKIP_UI_BUILD=1` 跳过构建
- [x] 5.4 更新 `README.md`：说明 dev/web 两种启动方式、dist 构建策略、以及 DevTools 可见性规则

## 6. Verification

- [x] 6.1 `./scripts/dev.sh`：开发模式可用（后端 + Vite）
- [x] 6.2 `cd ui && npm run build`：生产构建成功
- [x] 6.3 `./scripts/web.sh`：访问 `http://127.0.0.1:7777/` 展示新 UI（非旧 dist）
- [x] 6.4 `cd backend && go test ./...`：后端测试通过（如本 change 有后端改动）
