## 1. Theme Toggle

- [x] 1.1 新增主题状态管理（light/dark），默认 `light` 并持久化到 localStorage
- [x] 1.2 在 Topbar 增加明暗主题切换控件，并在切换时更新 `document.documentElement` 的 `dark` class
- [x] 1.3 确认初始化流程：无本地配置时默认浅色，有配置时恢复用户选择

## 2. UI 文案中文化

- [x] 2.1 将 Workflows 首页（标题、按钮、卡片状态提示、空态、错误提示）统一为中文
- [x] 2.2 将 Workflow 详情页（控制栏、Inspector、Terminal 空态、错误提示）统一为中文
- [x] 2.3 将 Settings / DevTools 可见文案统一为中文，并保留生产环境隐藏 DevTools 规则

## 3. 规范与回归

- [x] 3.1 对新增/调整组件进行基本样式检查，确保浅色与深色下可读性正常
- [x] 3.2 更新本次 change 的任务勾选与必要文档（若结构职责变化则更新 `PROJECT_STRUCTURE.md`）

## 4. 验证

- [x] 4.1 `cd ui && npm run build`
- [x] 4.2 `timeout 20s ./scripts/dev.sh`
