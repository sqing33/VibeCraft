## 1. 依赖与基础接入

- [x] 1.1 在 `ui/package.json` 引入 `@heroui/react` 与 `framer-motion`，并清理不再使用的 shadcn/Radix 相关依赖
- [x] 1.2 更新 `ui/tailwind.config.cjs`：接入 `@heroui/theme` Tailwind plugin，并补充 content 扫描路径
- [x] 1.3 在 `ui/src/main.tsx` 接入 HeroUI Provider/Toast Provider，移除 shadcn Toaster 挂载

## 2. 页面与组件迁移（功能保持不变）

- [x] 2.1 迁移 `ui/src/app/components/Topbar.tsx`：Button/Badge 等替换为 HeroUI
- [x] 2.2 迁移 `ui/src/app/components/SettingsDialog.tsx`：Modal/Tabs/Input/Button/Alert/Skeleton/Toast
- [x] 2.3 迁移 `ui/src/app/components/LLMSettingsTab.tsx`：Select/Input/Button/Alert/Skeleton/Toast
- [x] 2.4 迁移 `ui/src/app/pages/WorkflowsPage.tsx`：Modal/Select/Input/Button/Alert/Skeleton/Toast
- [x] 2.5 迁移 `ui/src/app/pages/WorkflowDetailPage.tsx`：Select/Button/Alert/Skeleton/Chip/Toast
- [x] 2.6 迁移 `ui/src/app/components/DevToolsDialog.tsx`：Modal/Button/Chip/Skeleton/Toast

## 3. 清理与验证

- [x] 3.1 删除/替换 `ui/src/components/ui/*` 的 shadcn 组件实现，保留必要的工具函数（如 `cn()`）
- [x] 3.2 更新 `openspec/specs/ui/spec.md`（或确保 delta specs 能正确同步）以反映 HeroUI 技术栈要求
- [x] 3.3 在 `ui/` 目录执行 `npm run build` 与 `npm run lint`，修复迁移引入的 TS/ESLint 问题
