## ADDED Requirements

### Requirement: Chat composer MUST expose Codex reasoning effort control
The chat page MUST render a reasoning effort selector inside the composer control rail.

The selector MUST offer `low`, `medium`, `high`, and `xhigh` when the current runtime is Codex CLI.

The selector MUST remain visible but disabled when the current runtime is not Codex CLI.

#### Scenario: User sends a Codex turn with selected effort
- **WHEN** the active composer runtime is Codex CLI
- **AND** the user selects `high`
- **AND** sends a message
- **THEN** the UI includes `reasoning_effort=high` in the turn request

#### Scenario: User switches away from Codex
- **WHEN** the active composer runtime is not Codex CLI
- **THEN** the reasoning effort selector remains visible
- **AND** the selector is disabled

### Requirement: Chat composer MUST use a compact right-side control rail
The chat composer MUST use a left-right layout where the text input occupies the dominant area and the control rail occupies a narrower fixed-width column.

The control rail MUST be arranged in three rows: runtime selector, model selector, and a compact row containing the reasoning effort selector plus attachment and send buttons.

#### Scenario: User opens the composer on a wide layout
- **WHEN** the chat page has enough width for the desktop composer layout
- **THEN** the left textarea occupies the remaining width and height
- **AND** the right control rail remains visually narrower than before

## MODIFIED Requirements

### Requirement: UI SHALL support multi-turn chat with streaming render
The chat page MUST allow creating sessions, sending user turns, and rendering assistant streaming deltas in real time via WebSocket chat events.

The chat page MUST support selecting runtime/model per message and, for Codex CLI turns, sending the selected `reasoning_effort` to the daemon.

#### Scenario: Send turn and receive stream
- **WHEN** user sends a message in an active chat session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns`
- **AND** assistant response appears incrementally as `chat.turn.delta` events arrive

#### Scenario: Send Codex turn with runtime options
- **WHEN** user selects Codex CLI, a model, and a reasoning effort
- **AND** sends a message
- **THEN** the UI includes the selected runtime/model metadata
- **AND** includes `reasoning_effort` in the turn request
