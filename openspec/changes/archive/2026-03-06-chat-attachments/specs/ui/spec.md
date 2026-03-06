## ADDED Requirements

### Requirement: Chat composer MUST support attachment selection and removal
The chat page MUST provide an attachment upload entry in the composer for the active session.

The user MUST be able to select one or more supported files, review the selected attachments before sending, and remove selected attachments before submission.

#### Scenario: User selects attachments before sending
- **WHEN** user chooses one or more supported files in the chat composer
- **THEN** the UI shows the selected attachments in the composer before sending

#### Scenario: User removes a selected attachment
- **WHEN** user clicks remove on a selected attachment chip
- **THEN** that attachment is removed from the pending send list

### Requirement: Chat history MUST render message attachment metadata
When a message includes attachments, the chat UI MUST display attachment metadata in the corresponding message bubble.

The initial rendering MUST include at least file name and attachment kind.

#### Scenario: User message shows sent attachments
- **WHEN** a previously sent message has attachments
- **THEN** the chat history shows those attachments below the message content

## MODIFIED Requirements

### Requirement: UI SHALL support multi-turn chat with streaming render
The chat page MUST allow creating sessions, sending user turns, and rendering assistant streaming deltas in real time via WebSocket chat events.

The chat page MUST support sending turns with text, with attachments, or with both.

#### Scenario: Send turn and receive stream
- **WHEN** user sends a message in an active chat session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns`
- **AND** assistant response appears incrementally as `chat.turn.delta` events arrive

#### Scenario: Send turn with attachments
- **WHEN** user selects attachments and sends a turn
- **THEN** the UI submits the turn request including the selected attachments
- **AND** the message history refresh shows the attachments on the user message
