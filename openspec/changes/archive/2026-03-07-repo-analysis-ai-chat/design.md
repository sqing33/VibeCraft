## Context

当前 `vibe-tree` 已经把 Chat 主路径切换到 CLI runtime，并且 Chat 页面支持 CLI tool/model 选择；与此同时，Repo Library 仍然沿用“Python 脚本生成报告 + 抽卡片/建索引”的链路。问题不在于后处理，而在于“分析正文”没有复用真实 AI Chat，从而导致报告模板化、内容空泛、不可继续对话。

目标是让 Repo Library 的“真实分析”通过 AI Chat 完成，同时保留：
- 自动化：用户只需点击一次创建分析任务
- 可见性：分析过程保留在 Chat 会话中
- 可继续性：用户后续可继续在该会话里优化分析
- 兼容性：现有 cards/search 后处理继续使用 markdown report + JSON 结果

## Goals / Non-Goals

**Goals:**
- Repo analysis 默认走真实 CLI AI Chat，而不是脚本模板报告。
- 后台自动完成多轮 AI 分析，无需人工介入。
- Repo analysis run 持久化关联 chat session/tool/model。
- Repo Library 详情页能跳到 Chat 页面查看和继续会话。
- 继续复用 Python 做 prepare/cards/search。

**Non-Goals:**
- 不实现多代理 fan-out 版 Repo analysis orchestration。
- 不把 Repo analysis 改成普通用户手动 chat workflow。
- 不要求 Chat 页面显示 token-by-token CLI 内部工具细节；保留真实多轮会话即可。

## Decisions

### 1. 采用“Python prepare + AI Chat analysis + Python post-process”三段式
- Python `prepare`：只负责抓仓库、准备 snapshot/source、生成 code_index。
- AI Chat：负责真实分析、输出最终 markdown report。
- Python post-process：负责 extract-cards 与 search refresh。
- 原因：既复用现有 Repo Library 工具链，又把“核心理解”交给真实 AI。

### 2. 自动分析使用真实 chat session，并执行多轮自动 turn
- 创建 analysis 时自动创建 chat session，workspace 指向 snapshot/source。
- 后台自动执行至少两轮：
  - 第 1 轮：分析计划 / 重点风险 / 关键入口
  - 第 2 轮：输出最终 markdown report
- 原因：让 Chat 页面中保留可读的中间过程，同时不需要人工点击继续。

### 3. 最终报告以 assistant message 文本为真相源
- 不要求 AI 自己写 report 文件。
- 后端直接把最终 assistant message 保存为 `report.md`。
- 原因：最小化对 CLI agent 文件写入 contract 的依赖，复用现有 chat turn 收敛结果。

### 4. Repo analysis run 增加 chat linkage 与 runtime metadata
- 持久化 `chat_session_id`、`chat_user_message_id`、`chat_assistant_message_id`、`cli_tool_id`、`model_id`、`runtime_kind`。
- 原因：详情页需要打开会话，后续排障与重试也需要知道运行身份。

### 5. Chat 路由新增“打开指定 session”能力
- 新增 `#/chat/<sessionId>` 路由或等价机制。
- Chat 页面在进入时自动选中指定 session。
- 原因：Repo Library 详情页需要一键跳转到分析会话。

## Risks / Trade-offs

- [AI 输出报告格式漂移] → 用严格模板 prompt + 后端清洗 markdown fence，并在失败时保留 chat 供人工补救。
- [自动多轮 chat 耗时更长] → 接受成本增加，换取真实分析质量。
- [无 execution_id 导致详情页失去原 execution log] → 用 chat session 可见性替代 execution log 作为主要过程可视化。
- [daemon 重启时 running analysis 悬挂] → 增加 repo analysis run restart recovery，把非终态 run 标记为 failed。

## Migration Plan

1. 增加 Repo analysis run 的 chat linkage 字段与恢复逻辑。
2. 新增 Python `prepare` 命令，拆分掉当前 `pipeline` 的前置准备能力。
3. 在 repolib service 中接入 chat manager + expert resolution，自动执行多轮 AI 分析。
4. 继续使用 extract-cards/search 刷新后处理。
5. 前端新增 CLI tool/model 选择与“打开分析 Chat”跳转。
6. 完成测试、验证、文档更新并归档。

## Open Questions

- 自动多轮 chat 是否未来要支持配置为 2 轮以上。
- 是否要在后续把 planning turn 和 final report turn 的 system prompt 拆成独立模板文件。
