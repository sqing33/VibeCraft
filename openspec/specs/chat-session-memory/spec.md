# chat-session-memory Specification

## Purpose
TBD - created by archiving change sdk-chat-session-memory. Update Purpose after archive.
## Requirements
### Requirement: Chat sessions SHALL be persistent and resumable
The system MUST provide persistent SDK chat sessions stored in the local state database. A session MUST have a stable `session_id` and metadata including title, expert/provider/model identity, status, and timestamps. Sessions MUST remain available after daemon restart.

#### Scenario: Create and list sessions
- **WHEN** client calls `POST /api/v1/chat/sessions` and then `GET /api/v1/chat/sessions`
- **THEN** the new session appears in the list with a stable `session_id`
- **AND** session metadata includes creation and update timestamps

#### Scenario: Resume after restart
- **WHEN** a session has prior turns and daemon restarts
- **THEN** `GET /api/v1/chat/sessions/:id/messages` returns previously stored messages
- **AND** the user can continue the same session id

### Requirement: Chat turns SHALL support streaming output events
The system MUST provide a turn API that appends a user message, invokes SDK generation, and streams assistant deltas through WebSocket. The system MUST emit `chat.turn.started`, `chat.turn.delta`, and `chat.turn.completed` events.

#### Scenario: Successful streaming turn
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with `input`
- **THEN** a `chat.turn.started` event is emitted
- **AND** one or more `chat.turn.delta` events are emitted during generation
- **AND** `chat.turn.completed` is emitted with final assistant message metadata

### Requirement: Provider anchors SHALL be reused with safe fallback
For OpenAI sessions, the system MUST persist and reuse `previous_response_id` when available. For Anthropic sessions, the system MUST persist and reuse `container` when available. If anchor usage fails or is unavailable, the system MUST fallback to reconstructed local context and continue.

#### Scenario: OpenAI anchor reused
- **WHEN** a session has stored `previous_response_id`
- **THEN** subsequent OpenAI turn requests include that id
- **AND** the anchor is updated after successful response

#### Scenario: Anchor fallback path
- **WHEN** provider anchor is invalid or expired
- **THEN** the turn is retried using reconstructed context from local summary + recent messages
- **AND** the session remains usable

### Requirement: Context usage SHALL be guarded by automatic compaction
Before sending each turn, the system MUST estimate context usage ratio against model context window. If the ratio exceeds configured threshold(s), the system MUST compact older conversation content into session summary while preserving recent turns. The system MUST record compaction metadata.

#### Scenario: Soft threshold compaction
- **WHEN** estimated usage ratio exceeds configured soft threshold
- **THEN** the system compacts older turns into summary before provider call
- **AND** stores a compaction record in persistent storage

#### Scenario: Hard stop protection
- **WHEN** usage ratio remains above hard-stop threshold after compaction attempts
- **THEN** the turn request is rejected with a user-readable overflow error
- **AND** no provider call is made

### Requirement: Sessions SHALL support fork and manual compaction APIs
The system MUST provide APIs to fork a session and to trigger compaction manually for a specific session.

#### Scenario: Fork session
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/fork`
- **THEN** a new session is created with inherited summary/context baseline
- **AND** subsequent turns do not mutate the source session history

#### Scenario: Manual compact
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/compact`
- **THEN** compaction executes using current policy
- **AND** a `chat.session.compacted` event is emitted

