## Why

当前 Repo Library 的报告主要由脚本模板与索引启发式生成，不能真实复用 `vibe-tree` 已经打通的 CLI AI Chat 能力，导致分析质量偏低、过程不可追问，也无法把分析过程作为一个可继续优化的真实对话留给用户。现在需要把 Repo analysis 的核心分析阶段切换到真实 AI Chat，会话过程保留在 Chat 页面，同时保持自动化完成与后处理闭环。

## What Changes

- 新增 AI Chat 驱动的 Repo analysis 主流程：分析请求创建后自动准备仓库快照、自动创建 chat session，并由 CLI AI 连续完成规划与最终报告产出。
- Repo analysis run 持久化关联 chat session、自动生成的 user/assistant 消息与所选 CLI tool/model。
- 保留现有 Python 引擎用于仓库抓取、代码索引、知识卡片抽取与搜索索引刷新，但不再让它负责报告正文分析。
- Repo Library UI 新增 CLI tool/model 选择，并允许从分析详情直接跳转到对应 Chat session 继续优化结果。
- Chat 路由支持直接打开指定 session，便于从 Repo Library 详情跳转查看完整 AI 对话过程。

## Capabilities

### New Capabilities
- `repo-library-ai-analysis`: 使用真实 CLI AI Chat 会话完成 GitHub 仓库分析，并自动产出最终报告。

### Modified Capabilities
- `repo-library-ingestion`: 分析主流程改为“准备仓库 -> AI Chat 生成报告 -> 抽卡片/刷新检索”，并允许选择 CLI tool/model。
- `repo-library-ui`: 支持 tool/model 选择、查看关联分析 Chat、从详情跳转继续对话。
- `store`: Repo analysis run 持久化 chat session linkage、runtime/tool/model 元数据。
- `chat-session-memory`: 支持系统自动创建的 Repo analysis chat session 并允许后续继续对话。

## Impact

- 后端：`backend/internal/repolib/`, `backend/internal/store/`, `backend/internal/api/`, `backend/cmd/vibe-tree-daemon/main.go`
- Chat/CLI 运行时：`backend/internal/chat/manager.go`, `backend/internal/api/chat.go`, `backend/internal/expert/`
- Python 引擎：`services/repo-analyzer/app/cli.py`, `services/repo-analyzer/app/ingest.py`（或 prepare 流程）, `services/repo-analyzer/app/search.py`
- 前端：`ui/src/app/pages/RepoLibrary*`, `ui/src/app/routes.ts`, `ui/src/app/pages/ChatSessionsPage.tsx`, `ui/src/lib/daemon.ts`
- OpenSpec：新增 `repo-library-ai-analysis`，并修改 repo-library/chat/store 相关基线规范。
