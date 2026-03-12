## Context

Repo Library 目前通过 REST 接口返回仓库摘要与 analysis 状态，但 UI 侧缺少“状态变化驱动刷新”的机制，用户需要手动刷新才能看到 queued/running/failed/succeeded 的变化。该需求属于典型的服务端单向事件通知，更适合 SSE。

## Goals / Non-Goals

**Goals:**
- 提供 `GET /api/v1/repo-library/stream` SSE 事件流，推送 analysis 状态变更。
- UI 在 Repo Library 路由活跃时订阅 SSE，并在收到事件后防抖刷新仓库摘要列表。
- 左侧仓库列表在所有 Repo Library 页面显示状态图标（不影响现有点击导航/其他 action）。
- 刷新页面后仍能通过 REST 拉取恢复正确状态；SSE 只用于“实时更新”。

**Non-Goals:**
- 不把完整仓库摘要通过 SSE 传给前端；SSE 只推最小事件负载，前端仍以 REST 为真相源。
- 不实现基于 `Last-Event-ID` 的事件重放（v1 不需要，刷新页面可直接拉取最新状态）。
- 不替换现有 WebSocket 实时体系（工作流/编排/Chat 仍使用 WS）。

## Decisions

1. **SSE 事件驱动 + REST 真相源**
   - SSE 用于触发刷新，保证丢包/乱序/重复不会导致前端状态漂移。

2. **进程内 broker，慢客户端丢弃**
   - 每个连接一个带缓冲 channel；广播时非阻塞写入，慢连接丢弃，避免拖垮服务端。
   - 服务端发送 `: ping` keep-alive 防止代理空闲断开。

3. **订阅范围：仅 Repo Library 路由活跃时**
   - App 层根据路由开关 EventSource，避免全站常驻额外连接。

## Risks / Trade-offs

- [SSE 连接占用资源] → 仅在 Repo Library 路由活跃时建立；连接关闭及时 unsubscribe；keep-alive 控制频率。
- [dev 跨域下 EventSource 可能被代理缓冲] → 设置 `Cache-Control: no-cache` 与 `X-Accel-Buffering: no`；并保持 `Flush`。
- [事件丢失] → 前端收到事件后拉取列表作为最终状态；页面刷新也可恢复。

