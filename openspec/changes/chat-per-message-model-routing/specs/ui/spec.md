## ADDED Requirements

### Requirement: Chat input MUST allow per-message expert selection
The chat page MUST allow the user to choose an `expert_id` for each message sent.

#### Scenario: User selects expert for a message
- **WHEN** user selects an expert in the message composer
- **AND** user sends a message
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns` with `expert_id` set to the selected value

### Requirement: Chat UI MUST display per-message model identity
For assistant messages, the chat UI MUST display the message's `expert_id` and its `provider/model` identity when available.

#### Scenario: Assistant message shows model identity
- **WHEN** an assistant message is rendered
- **THEN** the message UI shows `expert_id` and `provider/model` for that message

## MODIFIED Requirements

### Requirement: UI SHALL support multi-turn chat with streaming render
The chat page MUST allow creating sessions, sending user turns, and rendering assistant streaming deltas in real time via WebSocket chat events.

The chat page MUST support selecting `expert_id` per message and sending it to the daemon as part of the turn request.

#### Scenario: Send turn and receive stream
- **WHEN** user sends a message in an active chat session
- **THEN** the UI calls `POST /api/v1/chat/sessions/:id/turns`
- **AND** assistant response appears incrementally as `chat.turn.delta` events arrive

#### Scenario: Send turn with expert selection
- **WHEN** user selects an expert and sends a message
- **THEN** the UI includes the selected `expert_id` in the turn request
- **AND** the streaming UI indicates which expert/provider/model is responding for the turn

