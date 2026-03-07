## Context

当前真实 AI Chat 分析主链已经建立：prepare 仓库、自动 chat session、assistant 生成 report、sync-chat 回写。现在的主要问题不是链路不存在，而是产品语义和后处理鲁棒性不足：
- 报告已经是 Markdown，但 UI 仍用 `<pre>` 直出；
- AI 生成报告与 cards extractor 的契约不够稳，导致卡片为空；
- 详情页仍把 execution log 当主要过程视图，和 AI Chat 路径不匹配；
- 用户继续对话后，系统没有足够明确地区分“正式报告版本”和“普通追问回复”。

## Goals / Non-Goals

**Goals:**
- 正式报告按 Markdown 渲染。
- AI 报告场景下 cards/evidence 抽取尽量稳。
- 详情页在 AI Chat 场景下优先引导用户去看 Chat 会话过程。
- 只有显式同步动作才会把后续对话回写成正式报告。
- 自动分析首轮继续强制输出结构化报告模板。

**Non-Goals:**
- 不改成完全实时双向同步每一条 Chat 回复。
- 不做新的多代理分析架构。
- 不重写整个 report extractor，只做兼容性增强与 UI 语义修复。

## Decisions

### 1. 报告正文统一用 Markdown renderer 显示
- 详情页对 `report_markdown` 走 Markdown 渲染，而不是 `<pre>`。
- 保留原始文本 fallback 用于异常内容。

### 2. AI Chat 场景下，详情页把 Chat 作为主要过程视图
- 若 analysis 有 `chat_session_id`，详情页明确展示“查看分析 Chat”入口。
- “分析日志”区域改成 AI Chat 模式说明，不再单纯显示 execution 空态。

### 3. cards extractor 同时兼容规范报告与轻微漂移报告
- 继续使用标题/标签驱动抽取，但放宽标题与标签匹配。
- 如果抽卡失败，UI 要明确说明是“报告已生成但尚未提取到稳定卡片”，而不是简单空白。

### 4. 同步策略保持显式触发
- 自动分析只对第一次正式报告使用强模板提示。
- 用户后续自由追问不会自动覆盖 report。
- 只有点击“同步最新 Chat 回复”时，才用最新 assistant reply 替换正式 report 并重抽卡片。

## Risks / Trade-offs

- [Markdown 渲染会把格式问题暴露得更明显] → 保留 fallback 与滚动容器。
- [抽卡仍可能为空] → UI 明确提示“抽取失败/证据不足”而不是误导为没有内容。
- [用户误以为后续对话自动改报告] → 在详情页加清晰提示，说明“只有同步按钮才会覆盖正式结果”。
