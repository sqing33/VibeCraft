## ADDED Requirements

### Requirement: UI SHALL provide chat session navigation and page
The UI MUST provide a navigation entry to a dedicated chat page. The chat page MUST support session list browsing and selecting one active session.

#### Scenario: Open chat page
- **WHEN** user clicks the chat entry in top navigation
- **THEN** the app navigates to `#/chat`
- **AND** the page displays the session list and conversation panel

### Requirement: UI SHALL support multi-turn chat with streaming render
The chat page MUST allow creating sessions, sending user turns, and rendering assistant streaming deltas in real time via WebSocket chat events.

#### Scenario: Send turn and receive stream
- **WHEN** user sends a message in an active chat session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns`
- **AND** assistant response appears incrementally as `chat.turn.delta` events arrive

### Requirement: UI SHALL expose compaction and session management actions
The chat page MUST provide actions for manual compaction and session fork. The UI MUST show result feedback for these actions.

#### Scenario: Trigger compact from UI
- **WHEN** user clicks manual compact for a session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/compact`
- **AND** displays success/failure feedback

#### Scenario: Fork session from UI
- **WHEN** user clicks fork on current session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/fork`
- **AND** the new forked session appears in the session list
