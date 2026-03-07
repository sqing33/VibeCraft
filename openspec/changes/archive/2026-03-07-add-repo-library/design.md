## Context

当前仓库已经有 Go daemon、SQLite state、React UI 与 execution manager，但没有一条独立的 Repo Library 产品线。现有 `github-feature-analyzer` 已经提供 repo 抓取、README-first 报告生成、合并子代理结果和本地向量检索脚本；这说明知识生产与检索能力已存在，只缺一层产品化包装与持久化建模。

约束：
- 保持 Go daemon 作为主产品 API 后端。
- 不把 Repo Library 首期强塞进现有 orchestration/chat 语义。
- 尽量复用 `.codex/skills/github-feature-analyzer/scripts/`。
- 需要支持向量检索，因此要保留 Python 处理链路与模型依赖。

## Goals / Non-Goals

**Goals:**
- 提供可用的 Repo Library 产品线：分析、落库、详情、卡片、搜索。
- 用 Go 管理 API、状态与 SQLite，用 Python 负责 analyzer/卡片抽取/向量检索。
- 支持长时间分析任务的状态与日志查看。
- 支持 XDG data 目录下的 Repo Library 工件存储。

**Non-Goals:**
- 本 change 不实现 chat session 的 repo-context 注入。
- 本 change 不实现 orchestration planner 的自动检索增强。
- 本 change 不把 Python 引擎升级为常驻 HTTP 服务；先采用命令式引擎。

## Decisions

### 1. Go 主后端 + Python 命令式引擎
- 方案：Go 提供 API、队列、状态与持久化；Python 提供 `ingest.py`、`extract_cards.py`、`search.py` 和共享工具。
- 原因：现有 analyzer 与向量检索脚本已经是 Python，复用成本最低；Go 继续作为产品壳最稳定。
- 备选：
  - 全量迁移到 Go：放弃，重写成本过高。
  - 直接上 FastAPI sidecar：延后，首期双进程常驻复杂度不必要。

### 2. Repo Library 独立于 orchestration/chat 主模型
- 方案：新增 `/api/v1/repo-library/*` 与独立的 store/service/ui 页面，不复用当前 orchestration create API 作为 repo ingest 入口。
- 原因：当前 orchestration 语义是 `goal + workspace_path`，不适配 `repo_url + ref + features`。
- 备选：把 repo analysis 伪装成 orchestration。缺点是语义不干净，UI 也不适合展示 repository-centric detail。

### 3. 文件工件与索引存储在 DataDir()/repo-library
- 方案：所有 source snapshot、report、derived cards 与向量索引存到 XDG data 目录。
- 原因：避免污染仓库工作区，符合应用级长期存储模型。
- 备选：继续使用 `.github-feature-analyzer/`，仅保留为兼容导入源，不作为主存储。

### 4. 用 execution manager 承接分析日志
- 方案：Repo analysis run 启动本地 wrapper 进程，并复用 execution manager 记录日志到现有日志目录；analysis run 记录 `execution_id`。
- 原因：现有日志落盘与 `GET /api/v1/executions/:id/log` 已可复用，能快速给 UI 提供长任务观测能力。
- 备选：Python 自己管理日志文件。缺点是会重复实现一套日志协议。

### 5. 搜索索引分为卡片层与 chunk 层
- 方案：卡片用于 UI 与复用摘要，chunk 用于向量检索；搜索结果指向 repository/snapshot/card/chunk。
- 原因：既保留高层语义，也保留可追溯的原始片段。

## Risks / Trade-offs

- [向量依赖较重] → 通过独立 Python venv 和脚本 wrapper 管理，缺依赖时返回明确错误。
- [分析任务耗时较长] → 使用异步 analysis run + execution log，避免阻塞请求。
- [卡片抽取质量不稳定] → 设计低置信度字段与 evidence 缺失容错，先保证结构化可用。
- [首期未做 chat/orchestration 复用] → 保持边界清晰，后续可在独立 change 中接入。

## Migration Plan

1. 新增 SQLite migration 与 Repo Library store API。
2. 新增 Python engine 与 Go repolib service，打通 ingest/extract/search。
3. 新增 Repo Library HTTP API。
4. 新增前端 Repo Library 路由与页面。
5. 补充验证与文档，更新 `PROJECT_STRUCTURE.md`。

## Open Questions

- 是否在后续 change 中把 Repo Library 搜索结果注入 orchestration artifact。
- 是否需要支持私有仓库与 GitHub token。
- 是否需要在后续将 Python engine 升级为 sidecar service。
