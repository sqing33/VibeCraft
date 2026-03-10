## Why

Repo Library 目前只要求 AI 尽量按模板输出正式报告，但没有把“格式可被后处理稳定消费”当成硬门槛。结果是报告肉眼可读却无法被抽卡与搜索链路消费，最终页面显示为空，且脏结果会被当成成功分析保存。

现在需要把正式报告从“提示词约束”升级为“机器校验通过后才生效”，避免 Repo Library 持续积累无法解析的分析结果。

## What Changes

- 为 Repo Library AI 分析增加正式报告格式校验闭环：候选报告生成后先校验，再决定是否作为正式报告落盘。
- 新增基于脚本的报告校验器，校验标题层级、必填段落、按 feature 拆分、表格要求、证据格式，以及是否至少能抽出一张知识卡片。
- 当校验失败时，系统自动把校验错误反馈给 AI，要求在同一会话中重写报告，直到通过或达到重试上限。
- 若达到重试上限仍失败，保留最后一版无效草稿与校验结果，但不写入正式卡片与搜索索引。
- 统一自动分析与“同步最新 Chat 回复”两条路径，均采用同一校验逻辑，避免行为分叉。

## Capabilities

### New Capabilities
- `repo-library-report-validation`: 约束正式报告必须通过结构与可抽卡校验，失败时自动重写并保留校验工件。

### Modified Capabilities
- `repo-library-ai-analysis`: 正式报告生成不再以单次 AI 输出为准，而是以校验通过后的版本为准。

## Impact

- Affected code: `backend/internal/repolib/`, `services/repo-analyzer/app/`, `services/repo-analyzer/tests/`.
- Affected behavior: Repo Library AI 分析完成条件、同步最新 Chat 回复后的正式结果落盘条件。
- New artifacts: 报告校验结果 JSON、每轮候选报告草稿。
- No public HTTP API changes and no database schema changes.
