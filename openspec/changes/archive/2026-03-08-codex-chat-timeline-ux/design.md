## Context

当前实现已经把 Codex 运行时拆成 `chat.turn.event`，但后端对所有 reasoning 统一使用 `entry_id="thinking"`，前端 reducer 也主要按 `entry_id` 合并更新，因此真实时间线里的多段思考会被压扁成一个条目。与此同时，命令卡片会直接渲染完整 `stdout/stderr`，让过程区在长命令或失败日志场景下非常拥挤。

这个问题跨越后端事件建模、翻译流转和前端渲染，因此需要先统一时间线模型，再局部调整 UI。

## Goals / Non-Goals

**Goals:**
- 让 Codex 过程区按真实发生顺序展示 thinking / tool / plan / question / answer。
- thinking 片段在发生穿插事件后自动断开，并在后续 reasoning 时开启新片段。
- 翻译增量只写回对应的 thinking 片段，避免跨片段污染。
- 命令输出默认折叠，保留“看见执行了什么 + 按需展开细节”的体验。

**Non-Goals:**
- 不改变 legacy `chat.turn.delta` / `chat.turn.thinking.delta` 兼容事件。
- 不新增新的聊天协议通道或独立日志面板。
- 不对非 Codex provider 的 thinking 展示模型做大改。

## Decisions

1. **由后端生成稳定时间线主键，而不是让前端猜测分段**
   - 为每个结构化条目分配稳定 `entry_id`，并新增 `seq` 作为同一 turn 内的时间线顺序。
   - tool 条目继续按 `callId` 复用，thinking 条目改为 `thinking:1`、`thinking:2` 这类分段 ID。

2. **非 thinking 运行时活动会关闭当前 thinking 片段**
   - 连续 reasoning delta 追加到当前片段。
   - 一旦出现 tool / plan / question / system / progress / answer，就关闭当前 thinking；后续 reasoning 重新建段。
   - 这样可以稳定表达 `thinking → tool → thinking`，并与参考项目的 timeline 模型一致。

3. **翻译缓冲绑定到当前 thinking 片段，并在片段切换时强制 flush**
   - translation runtime 记录当前缓冲所属的 `entry_id`。
   - 片段边界出现时强制翻译并广播带 `entry_id` 的 delta，避免译文落到新的 thinking 卡片上。
   - 对非结构化 provider 保持 `entry_id` 可空，前端继续回退到最近一个 thinking 条目。

4. **UI 使用单一时间线渲染，工具输出本地折叠**
   - feed reducer 始终按 `seq` 排序。
   - 组件不再把 answer 特殊挪到末尾，而是按时间线顺序渲染并仅通过样式突出回答。
   - 命令输出折叠状态放在条目组件本地 state，并由稳定 `key=entry_id` 保持。

## Risks / Trade-offs

- **事件数量略增** → thinking 分段会增加条目数，但每个条目更短、更可读，且 payload 仍是轻量 JSON。
- **翻译片段更细** → 片段切换时会更早触发翻译请求；通过已有阈值与 idle 窗口控制请求频率。
- **旧前端兼容性** → 本仓库前后端同步交付；同时保留 legacy 事件，降低回归风险。

## Migration Plan

- 无需数据迁移。
- 部署顺序可以直接前后端一起发布；若仅后端先发，旧前端会忽略新增字段。

## Open Questions

- 暂无；本次方案仅增强现有协议，不引入未决依赖。
