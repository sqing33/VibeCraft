## Context

当前 UI 通过 `GET /api/v1/ws` 订阅 daemon 的 WebSocket 事件流，并依赖 `chat.turn.*` 增量事件在 Chat 页渲染“进行中”状态与流式内容。与此同时，后端已将结构化 turn timeline 落库，并通过 `GET /api/v1/chat/sessions/:id/turns` 提供可恢复快照，因此刷新页面可以恢复长任务期间的新增思考过程。

在长耗时且高频输出的 turn 中，后端 WS hub 对慢客户端采用“队列满则丢弃”的策略，且写协程每次仅写一条消息，容易出现吞吐不足与事件断档，进而导致前端 UI 看起来“停住”，直到手动刷新。

## Goals / Non-Goals

**Goals:**

- 降低长任务期间 WS 下行背压导致的丢消息概率，让实时渲染更稳定。
- 在 WS 断开或静默（无增量事件）时，Chat 页可自动从 turns/messages API 追帧，避免用户必须刷新页面。
- 协议保持向后兼容：继续支持单 envelope 帧；新增数组帧形态由前端兼容解析。

**Non-Goals:**

- 不引入“客户端 ACK + 服务端重放”这类可靠传输协议（复杂度过高）。
- 不把 `POST /chat/sessions/:id/turns` 改为异步/立即返回（属于更大 API 行为变更）。
- 不承诺 WS 完全无丢包（仍保留队列满丢弃作为最后兜底）。

## Decisions

1. **WS 写协程批量发送（JSON 数组）**
   - 现状：每条 envelope 单独 `WriteMessage`，系统调用与锁竞争开销大。
   - 方案：在 `writePump` 读取到一条消息后，非阻塞 drain 追加若干条（有上限），将 1..N 条 envelope 组装为单个 JSON 数组后一次写出；仅 1 条时可保持单对象写出。
   - 理由：显著减少写调用次数，提高吞吐；且对消息顺序保持 FIFO。

2. **扩大单客户端 send 缓冲并保留“满则丢”兜底**
   - 将 `client.send` 缓冲从 256 提升到更高（例如 2048），缓解瞬时突刺。
   - 继续使用 non-blocking broadcast，避免单个慢客户端阻塞整个 hub。

3. **前端解析兼容 envelope 或 envelope[]**
   - 将 `parseWsEnvelope` 升级为返回 0..N 个 envelopes，支持单对象与数组帧。
   - `App.tsx` 中对解析结果逐个 `emitWsEnvelope`，保持下游消费逻辑不变。

4. **Chat 页“追帧轮询”作为 WS 断档兜底**
   - 维护 chat 相关 WS 活动时间戳；当检测到 active session 存在 pending turn 且 WS 断开或长时间无增量事件时，以固定间隔轮询 `fetchChatTurns`（必要时联动 `fetchChatMessages`）追上 backend 持久化的 timeline。
   - 轮询仅在“确实进行中”时开启，并在 turn 终态/恢复收到 WS 更新后自动停止，避免常态增加负载。

## Risks / Trade-offs

- [旧客户端不兼容数组帧] → 通过前端解析兼容解决；发布时需保证前后端同步升级。
- [更大 send 缓冲带来额外内存占用] → 缓冲仍为有界；并通过批量发送减少积压形成。
- [轮询造成额外 API 负载] → 仅在 pending turn 且 WS 异常/静默时启用，并使用较低频率与停止条件控制。

## Migration Plan

1. 先落地前端对数组帧的解析兼容（单帧仍可为对象）。
2. 再上线后端批量写与更大缓冲（在负载下才会产生数组帧）。
3. 验证：长耗时 turn 下 UI 不需要刷新即可持续看到新增 timeline；断网/断 WS 后能自动追帧并在恢复后回到实时。

## Open Questions

- “静默阈值”取值（例如 20s/30s）与轮询间隔（例如 3s/5s）在真实 Codex 输出节奏下的最优组合。
- 后端批量 drain 的上限策略：按条数限制（更简单）还是按字节数限制（更稳但实现更复杂）。

