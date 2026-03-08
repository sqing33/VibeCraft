## ADDED Requirements

### Requirement: Structured Codex runtime feed MUST be recoverable from backend state
When a Codex-backed chat turn emits structured runtime activity, the daemon MUST persist each turn entry before or together with broadcasting incremental runtime events.

The persisted timeline MUST be sufficient to reconstruct the currently visible answer, tool, plan, question, system, and progress entries after a page refresh or WebSocket reconnect.

#### Scenario: Structured runtime event is persisted before refresh recovery
- **WHEN** a Codex-backed turn emits a `chat.turn.event` update for an entry
- **THEN** the daemon persists the updated turn entry in backend state
- **AND** a later page reload can rebuild that entry without relying on browser session storage

#### Scenario: Final answer matches persisted answer entry
- **WHEN** the final assistant message is stored for a Codex-backed turn
- **THEN** the persisted `kind=answer` timeline content matches the final assistant message content
- **AND** the completed turn can be restored from backend state alone
