## Why

当前前端仍偏 MVP 工具面板形态：信息架构与视觉一致性不足、默认暴露 demo/调试入口、且 UI 实现与 `openspec/specs/ui/spec.md` 的技术栈要求长期偏离。需要一次“生产化”变更，把 UI 调整到接近完成态并让规范与实现重新对齐。

## What Changes

- 将 UI 信息架构重心调整为 Workflows：列表（Kanban）→ 详情（DAG + Inspector + Terminal）的主路径更清晰。
- 引入并落地 Tailwind CSS + shadcn/ui + Zustand，逐步拆分 `ui/src/App.tsx`，用组件化方式替换手写布局 CSS。
- 默认隐藏 demo/调试入口（例如 oneshot demo execution），仅在开发环境或显式开启时可见。
- 优化 Web 单进程启动体验：`scripts/web.sh` 默认构建 UI（避免旧 `ui/dist` 导致“看到的还是旧页面”）。
- （可选）为 Kanban 展示补齐后端聚合字段（例如 running 节点数），以向后兼容方式扩展。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `ui`: 增加“生产环境隐藏 DevTools/诊断入口”的行为要求，并明确 Settings/诊断信息的呈现方式与信息架构（Workflows-first）。

## Impact

- 主要影响 `ui/`（依赖、组件结构、状态管理、视觉与交互）。
- 影响 `scripts/web.sh`（UI 构建策略）。
- （可选）影响 `backend/internal/api/workflows.go`（返回字段扩展，仅展示用途，向后兼容）。
