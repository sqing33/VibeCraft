## MODIFIED Requirements

### Requirement: Chat turns SHALL support streaming output events
The system MUST provide a turn API that appends a user message, invokes SDK generation, and streams assistant deltas through WebSocket. The system MUST emit `chat.turn.started`, `chat.turn.delta`, and `chat.turn.completed` events.

The turn API MUST support both pure-text requests and attachment-bearing requests. For attachment-bearing requests, the user message history MUST remain persistent and resumable together with its attachment metadata.

#### Scenario: Successful streaming turn
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with `input`
- **THEN** a `chat.turn.started` event is emitted
- **AND** one or more `chat.turn.delta` events are emitted during generation
- **AND** `chat.turn.completed` is emitted with final assistant message metadata

#### Scenario: Successful streaming turn with attachments
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with attachments and optional input
- **THEN** the user message and its attachment metadata are persisted before provider execution
- **AND** assistant streaming events are emitted using the same `chat.turn.*` event types

### Requirement: Provider anchors SHALL be reused with safe fallback
For OpenAI sessions, the system MUST persist and reuse `previous_response_id` when available. For Anthropic sessions, the system MUST persist and reuse `container` when available. If anchor usage fails or is unavailable, the system MUST fallback to reconstructed local context and continue.

For sessions that contain persisted attachments, local reconstruction MUST rebuild the multimodal message history using stored message text and attachment metadata/file contents instead of falling back to a text-only prompt.

#### Scenario: OpenAI anchor reused
- **WHEN** a session has stored `previous_response_id`
- **THEN** subsequent OpenAI turn requests include that id
- **AND** the anchor is updated after successful response

#### Scenario: Anchor fallback path
- **WHEN** provider anchor is invalid or expired
- **THEN** the turn is retried using reconstructed context from local summary + recent messages
- **AND** the session remains usable

#### Scenario: Anchor fallback reconstructs attachments
- **WHEN** a session with historical attachments loses its provider anchor
- **THEN** the fallback path rebuilds the prior multimodal context from locally persisted attachments
- **AND** the session remains usable

### Requirement: Context usage SHALL be guarded by automatic compaction
Before sending each turn, the system MUST estimate context usage ratio against model context window. If the ratio exceeds configured threshold(s), the system MUST compact older conversation content into session summary while preserving recent turns. The system MUST record compaction metadata.

For sessions whose persisted history contains attachments, the system MUST NOT run the existing text-only automatic compaction path.

#### Scenario: Soft threshold compaction
- **WHEN** estimated usage ratio exceeds configured soft threshold
- **THEN** the system compacts older turns into summary before provider call
- **AND** stores a compaction record in persistent storage

#### Scenario: Hard stop protection
- **WHEN** usage ratio remains above hard-stop threshold after compaction attempts
- **THEN** the turn request is rejected with a user-readable overflow error
- **AND** no provider call is made

#### Scenario: Attachment session skips automatic compaction
- **WHEN** the session history already contains persisted attachments
- **THEN** automatic compaction is skipped for that turn
- **AND** the system preserves attachment-bearing context without generating a text-only compaction summary
