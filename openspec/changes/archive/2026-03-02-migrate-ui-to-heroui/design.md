## Context

- 现状：`ui/` 使用 Tailwind CSS + shadcn/ui（自维护的 Radix 组件封装）实现常用 UI（Button/Input/Dialog/Select/Tabs/Toast 等）。
- 约束：保持现有页面信息架构与交互（工作流看板、详情页、设置/诊断、开发工具）不变；继续使用 React + TS + Vite + Tailwind；主题切换沿用 `html.dark` class（Zustand store）。
- 变更点：替换组件库到 HeroUI（`@heroui/react`），并调整 OpenSpec UI 规范中的技术栈描述。

## Goals / Non-Goals

**Goals:**

- 使用 `@heroui/react` 替换 shadcn/ui 组件层，覆盖现有 UI 需求（Alert/Toast/Button/Input/Select/Tabs/Modal/Skeleton/Chip 等）。
- 迁移过程中尽量保持页面结构与行为稳定：状态展示、弹窗交互、表单、Kanban 卡片与详情页操作逻辑不变。
- 清理不再需要的 Radix/shadcn 相关依赖与本地封装组件，降低维护成本。
- 更新 `openspec/specs/ui/spec.md`（通过 delta specs 同步）以反映新技术栈要求。

**Non-Goals:**

- 不进行 UI 大改版/重新设计布局；只做必要的样式微调以匹配 HeroUI 组件 API。
- 不改动后端 API/数据结构；不改变业务流程（workflow/node/execution 行为不变）。

## Decisions

1. **组件使用方式**
   - 选择直接从 `@heroui/react` 引入组件（单一入口），避免再维护一套复杂的二次封装层。
   - 对于 `toast()`：提供一个薄适配（维持现有调用形态，内部转发到 HeroUI toast），减少业务代码改动量。

2. **Tailwind 集成**
   - 在 `ui/tailwind.config.cjs` 中启用 `@heroui/theme` 的 Tailwind plugin，并补充 `content` 扫描路径（包含 HeroUI theme 产物），确保类名生成完整。
   - 保留现有基于 CSS 变量的 `bg-background/text-foreground` 体系，HeroUI 组件使用其默认 token；两者可并存。

3. **应用入口 Provider**
   - 在 `ui/src/main.tsx`（或 App Root）引入 HeroUI Provider（如 `HeroUIProvider`）以保证 Modal/Toast/Portal 等能力正常工作。
   - 继续使用 `ThemeStore` 仅通过 `document.documentElement.classList.toggle('dark')` 控制暗色模式；HeroUI 组件通过 Tailwind darkMode 同步生效。

4. **依赖清理策略**
   - 新增：`@heroui/react`、`framer-motion`（HeroUI peer deps）。
   - 移除：`@radix-ui/*`、`class-variance-authority`、`tailwindcss-animate`（若不再使用）、shadcn 相关本地组件文件。
   - 保留：`clsx`/`tailwind-merge`（若仍用于业务 className 合并）。

## Risks / Trade-offs

- [Tailwind plugin/样式冲突] → 保持现有 `ui/src/index.css` 变量体系不动，HeroUI 插件按文档方式接入；通过 `npm run build` 验证。
- [组件 API 差异导致交互细节变化] → 以最小改动迁移（先功能对齐，再微调 className）；优先保证可用性与一致的可访问性行为。
- [Toast 行为差异] → 使用统一 `toast()` 适配层，集中处理 variant/文案映射。

