## 1. OpenSpec 与数据模型

- [x] 1.1 为 Repo Library 增加 proposal、design、specs 并通过校验
- [x] 1.2 扩展 SQLite migration，新增 repo source/snapshot/run/card/evidence/query 表
- [x] 1.3 实现 Repo Library store 读写与列表/详情/搜索查询接口

## 2. Python Repo Analyzer Engine

- [x] 2.1 新增 `services/repo-analyzer` 目录与统一 CLI 入口
- [x] 2.2 复用现有 analyzer 脚本实现 ingest 流程与结构化结果输出
- [x] 2.3 实现知识卡片与 evidence 抽取脚本
- [x] 2.4 接入向量索引 build/query，并输出稳定 JSON 搜索结果

## 3. Go 后端 Repo Library

- [x] 3.1 新增 `backend/internal/repolib` service，负责提交分析、执行 Python engine、收敛结果
- [x] 3.2 新增 `/api/v1/repo-library/*` API 与请求/响应模型
- [x] 3.3 复用 execution manager 提供分析运行日志与状态观测

## 4. 前端 Repo Library 产品线

- [x] 4.1 扩展路由、顶栏导航与 daemon client 类型定义
- [x] 4.2 新增 Repositories 页面与 Analyze Repo 表单
- [x] 4.3 新增 Repository Detail 页面，展示 snapshots、runs、cards、report 与 evidence
- [x] 4.4 新增 Pattern Search 页面并接入搜索 API

## 5. 验证、文档与归档

- [x] 5.1 为 store/API/Python engine 增加针对性测试或验证脚本
- [x] 5.2 更新 `PROJECT_STRUCTURE.md` 与相关文档
- [ ] 5.3 完成 OpenSpec tasks 状态、执行校验并归档 change
