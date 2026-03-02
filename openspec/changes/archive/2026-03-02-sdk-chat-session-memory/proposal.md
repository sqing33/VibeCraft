## Why

Current SDK-driven calls in vibe-tree are one-shot and stateless. Users cannot continue long conversations like Codex CLI / Claude Code CLI, and context length can eventually overflow without automatic compaction.

## What Changes

- Add a persistent chat session subsystem for SDK conversations with session list/resume/fork support.
- Add automatic context compaction when estimated token usage crosses configured thresholds.
- Add provider anchor persistence for OpenAI (`previous_response_id`) and Anthropic (`container`) with fallback to reconstructed local history.
- Add chat REST APIs and WebSocket streaming events for session turns.
- Add a new UI chat page (session list + conversation pane) and entry from top navigation.
- Add optional workflow integration hooks so master-node SDK execution can opt in to session memory in future follow-ups (non-breaking, default off).

## Capabilities

### New Capabilities
- `chat-session-memory`: Persistent SDK chat sessions with multi-turn context, automatic compaction, provider anchor reuse, and resume/fork APIs.

### Modified Capabilities
- `ui`: Add chat route/page and realtime rendering of chat session turns/events.

## Impact

- Backend modules: `internal/api`, `internal/store`, `internal/runner`, and new `internal/chat` service.
- SQLite migration from schema v1 to v2 (new chat tables + indexes).
- New HTTP APIs under `/api/v1/chat/*` and new WS event types (`chat.turn.*`, `chat.session.compacted`).
- Frontend: new page/components/stores/API client methods and route integration.
- New unit/integration tests for store migration, chat manager, API handlers, and UI state updates.
