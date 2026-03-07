## MODIFIED Requirements

### Requirement: Chat sessions SHALL be persistent and resumable
The system MUST provide persistent chat sessions stored in the local state database. A session MUST have a stable `session_id` and metadata including title, selected expert/runtime identity, status, and timestamps. Sessions MUST remain available after daemon restart.

When the active chat runtime uses CLI execution, the session MUST persist any runtime session reference needed for continuation, but the session MUST remain resumable even if no provider-specific anchor exists.

#### Scenario: Create and list sessions
- **WHEN** client calls `POST /api/v1/chat/sessions` and then `GET /api/v1/chat/sessions`
- **THEN** the new session appears in the list with a stable `session_id`
- **AND** session metadata includes creation and update timestamps

#### Scenario: Resume after restart
- **WHEN** a session has prior turns and daemon restarts
- **THEN** `GET /api/v1/chat/sessions/:id/messages` returns previously stored messages
- **AND** the user can continue the same session id without requiring provider-specific anchor recovery

### Requirement: Chat turns SHALL support streaming output events
The system MUST provide a turn API that appends a user message, invokes CLI generation, and streams assistant deltas through WebSocket. The system MUST emit `chat.turn.started`, `chat.turn.delta`, and `chat.turn.completed` events.

For turns that have thinking translation enabled, the system MUST additionally emit `chat.turn.thinking.translation.delta` events during helper-SDK translation and `chat.turn.thinking.translation.failed` if translation fails.

The final `chat.turn.completed` event for those turns MUST include translated reasoning output when available together with explicit translation status fields.

The turn API MUST support both pure-text requests and attachment-bearing requests. For attachment-bearing requests, the user message history MUST remain persistent and resumable together with its attachment metadata.

#### Scenario: Successful streaming turn
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with `input`
- **THEN** a `chat.turn.started` event is emitted
- **AND** one or more `chat.turn.delta` events are emitted during CLI generation
- **AND** `chat.turn.completed` is emitted with final assistant message metadata

#### Scenario: Successful translated reasoning turn
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` for a turn that has thinking translation enabled
- **THEN** the system emits one or more `chat.turn.thinking.translation.delta` events as translated reasoning becomes available
- **AND** `chat.turn.completed` includes the translated reasoning result and translation status fields

#### Scenario: Successful streaming turn with attachments
- **WHEN** client posts `POST /api/v1/chat/sessions/:id/turns` with attachments and optional input
- **THEN** the user message and its attachment metadata are persisted before CLI execution
- **AND** assistant streaming events are emitted using the same `chat.turn.*` event types

### Requirement: Context usage SHALL be guarded by automatic compaction
Before sending each chat turn, the system MUST estimate context usage ratio against the effective chat runtime context budget. If the ratio exceeds configured threshold(s), the system MUST compact older conversation content into session summary while preserving recent turns. The system MUST record compaction metadata.

For sessions whose persisted history contains attachments, the system MUST NOT run the existing text-only automatic compaction path.

#### Scenario: Soft threshold compaction
- **WHEN** estimated usage ratio exceeds configured soft threshold
- **THEN** the system compacts older turns into summary before launching the next CLI turn
- **AND** stores a compaction record in persistent storage

#### Scenario: Hard stop protection
- **WHEN** usage ratio remains above hard-stop threshold after compaction attempts
- **THEN** the turn request is rejected with a user-readable overflow error
- **AND** no CLI runtime call is made

#### Scenario: Attachment session skips automatic compaction
- **WHEN** the session history already contains persisted attachments
- **THEN** automatic compaction is skipped for that turn
- **AND** the system preserves attachment-bearing context without generating a text-only compaction summary

### Requirement: Sessions SHALL support fork and manual compaction APIs
The system MUST provide APIs to fork a session and to trigger compaction manually for a specific session.

Forked sessions MUST inherit the source session's summary/context baseline without depending on provider-anchor reuse.

#### Scenario: Fork session
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/fork`
- **THEN** a new session is created with inherited summary/context baseline
- **AND** subsequent turns do not mutate the source session history

#### Scenario: Manual compact
- **WHEN** client calls `POST /api/v1/chat/sessions/:id/compact`
- **THEN** compaction executes using current policy
- **AND** a `chat.session.compacted` event is emitted

## REMOVED Requirements

### Requirement: Provider anchors SHALL be reused with safe fallback
**Reason**: CLI-backed chat sessions no longer depend on provider-specific `previous_response_id` or `container` anchors for their default continuation model.

**Migration**: Existing stored anchor data MAY remain readable for compatibility, but new chat turns MUST ignore provider-anchor reuse and continue through local summary, recent history, and CLI runtime session metadata.
