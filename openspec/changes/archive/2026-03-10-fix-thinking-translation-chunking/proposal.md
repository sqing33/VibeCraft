## Why

Codex app-server turns can surface duplicated visible thinking/answer text because the runtime currently consumes both structured `item/*` deltas and compatible legacy `codex/event/*` text deltas. After thinking translation is enabled, the current boundary-flush strategy also translates many undersized fragments created by Codex's interleaved status/tool events, which degrades translation quality and can trigger unnecessary helper-model traffic.

## What Changes

- Prefer Codex app-server `item/*` text streams for visible assistant/thinking content and suppress compatible legacy duplicate text notifications.
- Change thinking translation from a single boundary-forced buffer to entry-scoped ordered chunking so short Codex segments can wait for a publishable chunk or turn completion.
- Preserve raw whitespace between reasoning deltas so chunked translation keeps better sentence structure.
- Add regression tests for same-item thinking splits, duplicate Codex stream handling, and coherent translation chunking.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `cli-chat-thinking`: Codex app-server streaming should not double-append legacy and structured text deltas in the visible runtime timeline.
- `thinking-translation`: Thinking translation should stay entry-scoped while batching short interleaved segments into coherent chunks instead of forcing single-word flushes.

## Impact

- Backend chat runtime: `backend/internal/chat/codex_appserver.go`, `backend/internal/chat/codex_turn_feed.go`, `backend/internal/chat/thinking_translation.go`
- Regression tests under `backend/internal/chat/*_test.go`
- OpenSpec delta specs for `cli-chat-thinking` and `thinking-translation`
