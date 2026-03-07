## 1. OpenSpec 与规范

- [x] 1.1 完成 `repo-analysis-report-fixes` proposal、design、specs 并通过校验

## 2. 报告与卡片链路修复

- [x] 2.1 修复 Repo Library 详情页的 Markdown 报告渲染
- [x] 2.2 提升 AI 报告 cards/evidence 抽取兼容性
- [x] 2.3 让详情页在 AI Chat 场景下优先展示 Chat 过程，而不是 execution 空日志

## 3. 同步语义修复

- [x] 3.1 明确 sync-chat 只在显式触发时回写正式报告
- [x] 3.2 为自动分析 final turn 保留强格式提示，为后续自由对话保留普通 chat 语义
- [x] 3.3 在 UI 上提示“继续对话不会自动覆盖正式结果”

## 4. 验证与归档

- [x] 4.1 运行相关前端/后端/Python 验证
- [x] 4.2 归档 OpenSpec change
