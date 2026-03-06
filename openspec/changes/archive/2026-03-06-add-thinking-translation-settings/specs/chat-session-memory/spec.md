# chat-session-memory (delta): add-thinking-translation-settings

## MODIFIED Requirements

### Requirement: Chat turns SHALL support streaming output events

The system MUST provide a turn API that appends a user message, invokes SDK generation, and streams assistant deltas through WebSocket. The system MUST emit `chat.turn.started`, `chat.turn.delta`, and `chat.turn.completed` events.

For turns that have thinking translation enabled, the system MUST additionally emit `chat.turn.thinking.translation.delta` events during translation and `chat.turn.thinking.translation.failed` if translation fails.

The final `chat.turn.completed` event for those turns MUST include translated reasoning output when available together with explicit translation status fields.

The turn API MUST support both pure-text requests and attachment-bearing requests. For attachment-bearing requests, the user message history MUST remain persistent and resumable together with its attachment metadata.

#### Scenario: Successful streaming turn

- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with `input`
- **THEN** a `chat.turn.started` event is emitted
- **AND** one or more `chat.turn.delta` events are emitted during generation
- **AND** `chat.turn.completed` is emitted with final assistant message metadata

#### Scenario: Successful translated reasoning turn

- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` for a model that has thinking translation enabled
- **THEN** the system emits one or more `chat.turn.thinking.translation.delta` events as translated reasoning becomes available
- **AND** `chat.turn.completed` includes the translated reasoning result and translation status fields

#### Scenario: Successful streaming turn with attachments

- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with attachments and optional input
- **THEN** the user message and its attachment metadata are persisted before provider execution
- **AND** assistant streaming events are emitted using the same `chat.turn.*` event types
