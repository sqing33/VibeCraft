## Why

Repo Library 当前依赖 `services/repo-analyzer`（Python）完成 prepare / validate / extract-cards / search refresh，导致 Windows 打包与分发复杂、日志与排障链路割裂，且检索能力无法作为稳定的内置长期能力沉淀。

本变更将把 GitHub 知识库的分析与检索闭环收回到 Go daemon 内，以“本地 SQLite + 可重建检索派生库”的形态交付跨平台可用的知识库检索能力。

## What Changes

- 将 Repo Library 的 prepare / formal report validate / cards&vidence extract / search index refresh 全部替换为 Go 内置实现，不再要求用户侧存在 Python runtime。
- 新增 `search.db`（SQLite）作为检索派生库：FTS5 全文检索 + sqlite-vec 向量检索；`state.db` 继续作为业务真相库。
- 在 `POST /api/v1/repo-library/analyses` 的自动分析流程中，报告通过校验后自动抽取 cards/evidence 并刷新检索索引（分析成功不依赖索引成功）。
- 为 `search.db` 提供重建能力：rebuild all / rebuild snapshot / rebuild run（用于模型或扩展补齐、索引损坏、策略版本升级等场景）。
- 明确快照不可变：同一 `snapshot_id` 的源码内容生成完成后不可被覆盖；重跑分析创建新 snapshot。

## Capabilities

### New Capabilities

- `repo-library-searchdb`: Repo Library 检索派生库能力（`search.db` schema、版本化元信息、chunk 生成与向量/FTS 索引、重建与降级策略）。

### Modified Capabilities

- `repo-library-ingestion`: 由 Go 完成仓库抓取/快照准备/代码索引生成，并固化 snapshot 不可变语义。
- `repo-library-report-validation`: formal report 校验从 Python 迁到 Go，并引入 fatal/warning 分级语义。
- `repo-library-knowledge`: cards/evidence 抽取从 Python 迁到 Go，保证稳定 schema 与证据去重规则。
- `repo-library-search`: 搜索从 Python 迁到 Go，并升级为 FTS + vec 融合检索，返回以 card 为主的可解释结果。

## Impact

- 后端：`backend/internal/repolib` 将移除对 `services/repo-analyzer/app/cli.py` 的依赖；新增 `search.db` 管理、validator、cards extractor、repo fetcher 与 embedder（首版仅资源检查 + 失败降级）。
- 依赖：后端新增一个仅用于 `search.db` 的 SQLite driver（支持 load extension 与 FTS5），并引入 sqlite-vec 动态扩展的加载路径约定。
- API：`POST /api/v1/repo-library/search` 的能力增强（可选过滤/刷新/模式参数），并新增索引重建的内部 API（可先不暴露 UI）。
