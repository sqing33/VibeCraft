## 1. OpenSpec 对齐

- [x] 1.1 编写 proposal、design 与 delta specs，明确时间线分段、翻译绑定和默认折叠要求
- [x] 1.2 运行 OpenSpec 校验并修正文档问题

## 2. 后端事件模型

- [x] 2.1 为 Codex `chat.turn.event` 增加稳定 `seq`，并把 thinking 拆成多段 entry_id
- [x] 2.2 在 thinking 片段切换时强制 flush 翻译，并为翻译 delta 增加可选 `entry_id`
- [x] 2.3 增加回归测试，覆盖 thinking/tool/thinking 顺序与 stable seq

## 3. 前端时间线渲染

- [x] 3.1 更新 feed reducer，按 `seq` 保序并把翻译增量附着到对应 thinking 条目
- [x] 3.2 重构过程区渲染为单一时间线，命令输出默认折叠
- [x] 3.3 调整 WS 接线与 store 类型，消费新的翻译 payload

## 4. 验证与归档

- [x] 4.1 运行 Go 测试、UI 构建与 OpenSpec 校验
- [x] 4.2 完成后归档 `codex-chat-timeline-ux` 变更
