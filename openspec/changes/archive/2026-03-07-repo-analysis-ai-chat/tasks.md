## 1. OpenSpec 与数据模型

- [x] 1.1 完成 `repo-analysis-ai-chat` proposal、design、specs 并通过校验
- [x] 1.2 扩展 SQLite schema，给 repo analysis run 增加 chat linkage 与 runtime metadata 字段
- [x] 1.3 增加 repo analysis restart recovery，避免 daemon 重启后非终态 run 悬挂

## 2. Python 准备与后处理链路

- [x] 2.1 为 `services/repo-analyzer/app/cli.py` 增加 `prepare` 命令
- [x] 2.2 调整 Python ingest/prepare 逻辑，只做仓库抓取、snapshot/source 准备和 code index 生成
- [x] 2.3 保持 extract-cards 与 search 刷新兼容 AI 生成的 markdown report

## 3. 后端 AI Chat 驱动分析

- [x] 3.1 在 `backend/internal/repolib/service.go` 接入 chat manager 与 expert resolution
- [x] 3.2 创建 analysis 时自动创建 chat session，并以 snapshot/source 作为 workspace
- [x] 3.3 实现后台自动多轮 AI turn（计划轮 + 最终报告轮）
- [x] 3.4 将最终 assistant message 持久化为 snapshot report，并触发 cards/search 后处理
- [x] 3.5 更新 Repo Library API 请求/响应，支持 CLI tool/model 选择与 chat session linkage 暴露

## 4. UI 与可见性

- [x] 4.1 在 Repo Library Analyze Repo 表单中增加 CLI tool/model 选择
- [x] 4.2 在 Repo Library 详情页展示关联 chat session，并提供一键打开入口
- [x] 4.3 在 Chat 路由中支持直接打开指定 session

## 5. 验证、文档与归档

- [x] 5.1 为关键后端/前端/Python 逻辑补充定向测试或校验
- [x] 5.2 更新 `PROJECT_STRUCTURE.md` 与必要说明文档
- [x] 5.3 完成 OpenSpec 校验并归档 change
