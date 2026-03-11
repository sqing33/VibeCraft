## Context

Repo Library 当前分析链路在 Go daemon 内调度 AI Chat，但关键后处理（仓库准备/报告校验/抽卡/检索刷新）依赖 `services/repo-analyzer`（Python CLI）。该依赖对 Windows 分发与运行时环境要求较高，也使日志、错误分类与持久化链路割裂。

本设计目标是在不引入外部数据库服务的前提下，将“GitHub 项目分析沉淀为可检索知识库”作为应用内长期能力：以 `state.db` 存业务真相，以独立 `search.db` 存可重建的派生索引（FTS5 + sqlite-vec）。

约束：
- 平台：Linux + Windows
- 不要求用户额外安装数据库服务或 Python runtime
- 分析成功不依赖索引成功（索引失败仅降级检索能力，并记录可诊断信息）
- 第一版不向量化源码，仅向量化分析产物（report_section/card/evidence）

## Goals / Non-Goals

**Goals:**
- Go daemon 端内置：prepare / validate-report / extract-cards / search-index-refresh 全链路能力，替代 Python CLI。
- 引入 `search.db`：可重建派生库，支持 FTS 术语召回与向量语义召回，并返回可追溯结果（repo/snapshot/run/card/evidence/file:line）。
- 规范化 chunk 策略与稳定 `chunk_id`，支持增量刷新与 rebuild。
- 提供索引降级策略：缺少 sqlite-vec 或 embedder 资源时仍可用（FTS-only 或 search-disabled），且不阻断分析落库。

**Non-Goals:**
- 不做源码切块入向量库（命中后只做路径定位与按 snapshot 读取源码）。
- 不做在线 embedding API（首版仅检查本地 embedder 资源；后续可扩展自动下载/内置分发）。
- 不做复杂 rerank 与多阶段重写（首版提供可配置权重的简单融合）。

## Decisions

1. **`state.db` 与 `search.db` 分离**
   - 选择：`state.db` 继续作为业务真相（repo/snapshot/run/cards/evidence/search history）；`search.db` 存派生 chunk/FTS/vec/rowmap。
   - 理由：索引可删可重建；向量/FTS 的 schema/依赖可独立演进；索引失败不影响主链路。
   - 替代方案：在 `state.db` 内引入 vec/FTS。拒绝：需要在主库加载扩展、迁移风险高、跨平台打包更脆弱。

2. **索引对象边界：只索引分析产物**
   - 选择：`report_section`（按 heading 切块）、`card`（每卡一块）、`evidence`（每证据一块）。
   - 理由：规模可控、命中可解释、与产品价值对齐。

3. **稳定 `chunk_id` 与 `content_hash`**
   - `chunk_id` 固定规则：
     - `card:{card_id}`
     - `evidence:{evidence_id}`
     - `report:{snapshot_id}:{heading_path_hash}`
   - `content_hash = sha256(search_text)`；用于增量更新与 embedding 缓存。

4. **展示文本与检索文本分离**
   - `display_text` 面向 UI；`search_text` 面向 FTS/embedding，允许 tags/symbols/evidence refs 等扁平化拼装。

5. **报告校验分级（fatal vs warning）**
   - fatal：阻断正式报告发布、阻断抽卡与索引，并触发 AI 修订重试（最多 5 次）。
   - warning：允许发布正式报告并继续抽卡与索引，但在 run summary/result 中记录。

6. **依赖策略（跨平台）**
   - `search.db` 需要支持 FTS5 与 loadable extension：使用独立 SQLite driver（CGO）仅用于 searchdb；`state.db` 维持现状。
   - sqlite-vec 与 embedder 资源首版仅做“路径检查 + 友好报错 + 降级”，不做自动下载。

## Risks / Trade-offs

- [引入 CGO driver] → Mitigation：仅用于 `search.db`，并提供 build-time 文档与 runtime 自检；`state.db` 不受影响。
- [embedder 资源缺失导致无向量] → Mitigation：默认 FTS-only；run 标记 warning；提供 rebuild 入口待资源补齐后重建。
- [抽卡质量成为检索质量瓶颈] → Mitigation：抽卡规则可版本化；search.db metadata 记录 chunk_strategy_version；支持重建与回归测试 fixtures。

