## Why

`vibe-tree` 已经具备本地 daemon、SQLite、UI 与 execution 观测基础，但当前对外部 GitHub 仓库的知识沉淀仍停留在本地 skill 层，无法作为产品能力被长期保存、搜索和复用。现在引入 Repo Library，可以把外部仓库分析结果变成稳定的知识资产，并为后续 AI 开发提供真实参考样本。

## What Changes

- 新增 Repo Library 后端能力：支持提交 GitHub 仓库分析、持久化仓库/快照/分析运行记录、保存 analyzer 产物。
- 新增 Python repo-analyzer 引擎入口，复用现有 `github-feature-analyzer` 脚本完成 ingest、知识抽取和语义检索。
- 新增知识卡片与证据链抽取流程，将 `report.md` 与 `subagent_results.json` 结构化为可展示、可搜索的卡片数据。
- 新增 Repo Library 搜索能力，支持基于向量索引的语义查询与仓库过滤。
- 新增前端 Repo Library 产品线：仓库列表、分析表单、仓库详情、Pattern Search 页面。
- 新增分析运行状态与日志查看能力，允许用户观察长时间 repo 分析任务的进度与结果。

## Capabilities

### New Capabilities
- `repo-library-ingestion`: 受理仓库分析请求、运行 analyzer、保存仓库快照与分析产物。
- `repo-library-knowledge`: 从 analyzer 产物抽取知识卡片、证据项与派生元数据。
- `repo-library-search`: 基于向量索引与结构化过滤执行 Repo Library 搜索。
- `repo-library-ui`: 提供 Repo Library 的列表、分析、详情与搜索交互界面。

### Modified Capabilities
- `store`: 扩展 SQLite schema 与持久化访问层，支持 Repo Library 相关实体与查询。

## Impact

- 后端：`backend/internal/api/`, `backend/internal/store/`, `backend/internal/repolib/`, `backend/internal/paths/`。
- Python 引擎：新增 `services/repo-analyzer/`，并复用 `.codex/skills/github-feature-analyzer/scripts/`。
- 前端：`ui/src/app/routes.ts`, `ui/src/App.tsx`, `ui/src/app/components/Topbar.tsx`, `ui/src/app/pages/` 与 `ui/src/lib/daemon.ts`。
- 数据与依赖：SQLite 新表、XDG DataDir 下的 repo-library 文件存储、Python 向量检索运行环境。
