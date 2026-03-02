## Context

vibe-tree currently executes SDK calls in stateless oneshot mode. `runner.SDKSpec` contains only per-call prompt/inference parameters, and `sdk_runner` always sends a fresh request without conversation continuity. The state database stores workflows/nodes/executions/events only, so chat conversations cannot be resumed across requests or daemon restarts.

This change introduces a persistent chat session layer with automatic context compaction. It must integrate with existing HTTP + WebSocket patterns, reuse the current SQLite migration style, and keep workflow execution behavior backward compatible.

## Goals / Non-Goals

**Goals:**
- Provide resumable SDK chat sessions with durable message history.
- Support OpenAI and Anthropic provider anchors for efficient multi-turn continuity.
- Add automatic context compaction to prevent context overflow while preserving key conversation state.
- Expose stable chat APIs and WS events for UI streaming and session management.
- Add a dedicated UI page for session list and multi-turn chat interaction.

**Non-Goals:**
- Replacing workflow DAG execution with chat primitives.
- Introducing multi-user auth/tenant isolation in this iteration.
- Guaranteeing exact token counts for all providers (MVP uses estimator-first strategy).

## Decisions

### 1) Introduce dedicated chat domain (session/message/anchor/compaction)
- Decision: Add new SQLite tables (`chat_sessions`, `chat_messages`, `chat_anchors`, `chat_compactions`) rather than overloading workflow tables.
- Rationale: Workflow execution and chat conversation have different lifecycle and query patterns.
- Alternative considered: Reuse `events` table only. Rejected due to expensive replay and weak indexing for large conversations.

### 2) Use hybrid memory strategy (local history + provider anchors)
- Decision: Treat local persisted history as source of truth; use provider anchors as optimization.
- Rationale: Anchors can expire/be invalidated; local history enables deterministic fallback/recovery.
- Alternative considered: Anchor-only persistence. Rejected due to fragility during provider-side expiration.

### 3) Auto-compaction as threshold-based pre-send pipeline
- Decision: Before each turn, estimate context usage and compact when thresholds are crossed.
- Default thresholds: soft `0.82`, force `0.92`, hard-stop `0.97` of model context window.
- Rationale: Mimics CLI long-session behavior and prevents hard request failures.
- Alternative considered: Only compact after provider error. Rejected due to poor UX and retry loops.

### 4) Stream through existing WebSocket envelope channel
- Decision: Reuse `/api/v1/ws` and `ws.Envelope` with new `chat.*` event types.
- Rationale: Avoid introducing second realtime channel; frontend already has reconnect + parse path.
- Alternative considered: SSE for chat only. Rejected to reduce transport complexity.

### 5) Add chat UI as separate route/page first
- Decision: Add `#/chat` page with session list + conversation pane; keep workflow behavior unchanged.
- Rationale: Lower regression risk and faster validation of memory/compaction engine.
- Alternative considered: first integrating into workflow master node only. Rejected due to harder debugging and observability.

## Risks / Trade-offs

- [Risk] Token estimate can differ from provider true usage → Mitigation: conservative thresholds, compaction metadata, and hard-stop guard.
- [Risk] Compaction quality loss over long sessions → Mitigation: preserve recent turns uncompressed and store compaction history.
- [Risk] Provider anchor expiration breaks continuity → Mitigation: automatic fallback to reconstructed local context.
- [Risk] Larger DB footprint from chat logs → Mitigation: indexed queries, optional retention policy in follow-up.

## Migration Plan

1. Bump DB schema to v2 and add chat tables/indexes.
2. Add store layer CRUD and manager layer for turn handling/compaction.
3. Add API routes + WS events; keep existing routes unchanged.
4. Add UI route/page and daemon client methods.
5. Add tests for migration/store/api/manager behavior.
6. Rollback strategy: disable chat route usage in UI; existing workflow features remain unaffected since schema additions are additive.

## Open Questions

- Whether to expose per-session threshold overrides in UI now or keep server defaults only (default: server defaults only).
- Whether to integrate workflow master-node memory in this same change or keep as follow-up (default: follow-up).
