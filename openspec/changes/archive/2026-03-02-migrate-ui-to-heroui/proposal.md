## Why

当前前端 UI 基于 Tailwind CSS + shadcn/ui（Radix UI 组件封装）。随着页面与交互逐步扩展，继续维护一套“半自研”的 shadcn 组件层会带来重复成本与一致性问题。将组件库替换为 HeroUI，可以获得更完整的组件覆盖、更一致的交互与主题体系，并减少维护负担。

## What Changes

- 将 React 前端 UI 组件库从 shadcn/ui（Radix 封装）替换为 HeroUI（@heroui/react）。
- **BREAKING**：移除/替换 `ui/src/components/ui/*` 这类 shadcn 组件封装层；页面与业务组件改为直接使用 HeroUI 组件（或少量薄封装）。
- 更新 Tailwind 配置以启用 HeroUI theme/plugin，并补充必要的依赖（如 `framer-motion`）。
- 迁移 Dialog/Select/Tabs/Toast/Alert/Skeleton/Button/Input/Badge 等组件用法，保持现有功能与交互不变（健康状态、设置弹窗、Kanban、详情页、开发工具等）。
- 更新 OpenSpec UI 规范：技术栈要求从 shadcn/ui 调整为 HeroUI。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `ui`：Technology Stack 要求从 “Tailwind CSS + shadcn/ui” 调整为 “Tailwind CSS + HeroUI(@heroui/react)”。

## Impact

- 影响范围：`ui/` 前端（依赖、Tailwind 配置、组件与页面代码）。
- 不影响：后端 daemon API、desktop 壳逻辑。
- 风险：组件 API 差异导致样式/交互细节变化；需要通过 `ui` 构建与基本交互回归验证。

