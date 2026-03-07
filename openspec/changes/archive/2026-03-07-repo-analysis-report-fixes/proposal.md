## Why

当前 Repo Library 的真实 AI Chat 分析已经能生成报告并保留对话过程，但产品上仍有三个关键问题：报告正文没有按 Markdown 渲染、知识卡片/证据在 AI 报告场景下经常为空、以及后续继续对话时缺少“正式报告模式”与“自由追问模式”的边界，导致用户不清楚同步回写会不会破坏正式结果。

## What Changes

- 修复 Repo Library 详情页的报告展示，按 Markdown 渲染正式报告内容。
- 提升 AI 报告到知识卡片/evidence 的抽取兼容性，并在详情页更明确展示“暂无卡片”的真实原因。
- 调整分析过程展示逻辑：当 analysis 走 AI Chat 时，优先展示关联 Chat 会话过程而不是 execution 日志空态。
- 为 AI 分析会话增加“正式报告同步”边界：继续对话不会自动覆盖正式结果，只有显式同步动作才会回写 report/cards/search。
- 在自动分析 Chat 的系统提示中，要求最终正式回复严格输出结构化 Markdown 报告；后续普通对话不复用该强制格式模板。

## Capabilities

### New Capabilities
- `repo-library-report-presentation`: Repo Library 报告的 Markdown 渲染与 AI Chat 过程展示。

### Modified Capabilities
- `repo-library-knowledge`: 提升 AI 报告的 cards/evidence 抽取兼容性。
- `repo-library-ui`: 改善详情页的报告、卡片、分析过程与同步交互。
- `repo-library-ai-analysis`: 明确“正式报告生成”与“后续自由对话”之间的同步边界。
- `chat-session-memory`: 系统创建的 analysis session 在继续对话时不自动等价于新的正式报告版本。

## Impact

- 前端：`ui/src/app/pages/RepoLibraryRepositoryDetailPage.tsx`, `ui/src/lib/daemon.ts`
- 后端：`backend/internal/repolib/analysis_chat.go`, `backend/internal/repolib/sync_chat.go`
- Python：`services/repo-analyzer/app/extract_cards.py`
- OpenSpec：repo-library-ui / repo-library-knowledge / repo-library-ai-analysis / chat-session-memory
