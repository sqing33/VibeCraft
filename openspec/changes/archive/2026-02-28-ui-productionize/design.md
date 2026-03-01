## Context

当前 UI 仍偏 MVP 工具面板形态，核心逻辑集中在 `ui/src/App.tsx`，样式以手写 `App.css` 为主，默认暴露 demo/调试入口（例如 oneshot demo execution）。同时，OpenSpec 基线 `openspec/specs/ui/spec.md` 对技术栈（Tailwind/shadcn/Zustand）的要求尚未落地，导致“规范 vs 实现”长期偏离。

约束：
- 尽量复用现有 daemon HTTP/WS API（必要时仅做向后兼容扩展）。
- 保持 `./scripts/dev.sh` 开发模式可持续运行。
- Web 单进程模式（`./scripts/web.sh` + daemon 静态托管 `ui/dist`）可稳定复现“生产访问体验”。

## Goals / Non-Goals

**Goals:**
- UI 生产化：Workflows 为主线的信息架构与页面布局更接近完成态。
- 规范落地：引入并使用 Tailwind CSS + shadcn/ui + Zustand，逐步拆分 `App.tsx`。
- Demo/调试入口收敛：生产构建默认隐藏，开发环境可见。
- 生产构建可靠：`scripts/web.sh` 默认构建 UI，避免旧 `ui/dist` 误导。

**Non-Goals:**
- 不重写 scheduler/DAG/runner 的核心语义与数据模型（除非为 UI 展示补充只读字段）。
- 不在本变更中完成 desktop（Wails）打包发布流程。

## Decisions

1. **UI 结构与路由**
   - 保持现有 hash detail 路由：`/`（Kanban）与 `#/workflows/:id`（详情），对静态托管更友好。
   - 新增 App Shell（Topbar + Content），将连接信息/诊断信息从主页面移至 Settings。

2. **技术栈落地与组件化**
   - 引入 Tailwind CSS 与 shadcn/ui，统一 spacing/typography/组件风格。
   - 使用 shadcn 组件替换散落按钮/输入/错误块，并用 Toast/Alert 统一错误提示。

3. **状态管理（Zustand）**
   - 将 `App.tsx` 内的状态按领域拆分：
     - `daemonStore`：daemonUrl、health、wsState、info、experts
     - `workflowStore`：workflows、selectedWorkflowId、nodes、edges、selectedNodeId、loading/error
     - `executionStore`：selectedExecutionId、log tail、terminal 写入缓冲与断线补齐
   - WS 事件处理集中在一个 hook/store action，避免分散解析 envelope。

4. **DevTools 可见性**
   - 生产构建默认隐藏 demo-only 工具（符合 change specs 的新增要求），开发环境允许可见。

5. **构建与托管策略**
   - `scripts/web.sh` 默认构建 UI；提供 `VIBE_TREE_SKIP_UI_BUILD=1` 以便快速启动。

## Risks / Trade-offs

- **大组件重构风险** → 采用小步迁移：先引入依赖与 App Shell，再逐块替换页面与组件。
- **样式迁移冲突（全局 CSS vs Tailwind）** → 收敛 `index.css` 全局样式，逐步移除 `App.css` 的布局职责。
- **生产访问看到旧 dist** → `web.sh` 默认构建 + 明确“跳过构建”开关。

## Migration Plan

按 `tasks.md` 分阶段推进，每个任务完成后保持 `npm run build` 与 `./scripts/dev.sh` 可运行。

## Open Questions

- Kanban “running 节点数”是否需要后端聚合字段（展示增强 vs 后端侵入度）。
- 详情页日志展示是否需要实现 Terminal Pool（多 pane）还是以“选中节点单终端”为主（与基线 spec 的一致性取舍）。
