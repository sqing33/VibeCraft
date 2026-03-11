## ADDED Requirements

### Requirement: Chat UI MUST catch up running turns when live updates stall
When a chat turn is running, the UI MUST avoid requiring a manual page refresh to see newly persisted turn timeline progress.

If the WebSocket connection is disconnected, or if chat live-update events stop arriving for a sustained period while a turn is still pending, the UI MUST poll the backend for persisted turn snapshots and update the visible pending timeline accordingly.

#### Scenario: Stale WebSocket triggers turns polling catch-up
- **WHEN** a chat session has a pending turn (assistant is still running)
- **AND** the WebSocket is disconnected OR no `chat.turn.*` live-update events are received for a sustained period
- **THEN** the UI polls `GET /api/v1/chat/sessions/:id/turns`
- **AND** the pending process timeline advances to reflect newly persisted timeline items without a full page refresh

#### Scenario: Catch-up polling stops when the turn becomes terminal
- **WHEN** the UI is polling for catch-up during a pending turn
- **AND** the backend snapshot shows the latest turn is no longer running
- **THEN** the UI stops catch-up polling
- **AND** the chat page converges to a completed assistant message bubble with attached process details

## MODIFIED Requirements

### WebSocket Subscription

The system MUST connect to `GET /api/v1/ws` for real-time event subscription.

Each received WebSocket message MUST be interpreted as either:
- a single event envelope JSON object, or
- an array of event envelope JSON objects.

When a message contains an array of envelopes, the client MUST process them in array order.

The system MUST handle events: `workflow.updated`, `dag.generated`, `node.updated`, `execution.started`, `execution.exited`, `node.log`. `node.log` chunks MUST be written directly to the corresponding xterm instance.

#### Scenario: WebSocket reconnection

- **WHEN** WebSocket connection is lost
- **THEN** the client automatically attempts reconnection
- **AND** resumes receiving events after reconnection

#### Scenario: Batched WebSocket message is processed in order
- **WHEN** the client receives a WebSocket message containing an array of multiple envelopes
- **THEN** the client applies each envelope in order
- **AND** downstream UI state reflects all envelopes as if they were delivered in separate messages

