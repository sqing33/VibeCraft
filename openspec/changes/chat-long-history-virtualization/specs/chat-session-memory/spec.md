## ADDED Requirements

### Requirement: Chat message history API MUST support pagination by turn

The daemon MUST support reading chat message history in bounded pages via `GET /api/v1/chat/sessions/:id/messages`.

The endpoint MUST accept:
- `limit` (integer, optional): maximum number of messages to return.
- `before_turn` (integer, optional): when present, the endpoint MUST return only messages whose `turn` is strictly less than `before_turn`.

The endpoint MUST return results ordered ascending by conversation order (older → newer).

The endpoint MUST continue returning attachment metadata on each message when available.

#### Scenario: List recent messages without a cursor
- **WHEN** client calls `GET /api/v1/chat/sessions/:id/messages?limit=4`
- **THEN** the daemon returns up to 4 most recent messages for the session
- **AND** the returned messages are ordered from older to newer

#### Scenario: List older messages using before_turn
- **GIVEN** a session contains messages with turns 1..N
- **WHEN** client calls `GET /api/v1/chat/sessions/:id/messages?limit=4&before_turn=3`
- **THEN** the daemon returns messages whose `turn` is strictly less than 3
- **AND** no returned message has `turn >= 3`
- **AND** the returned messages are ordered from older to newer

#### Scenario: Reject invalid before_turn
- **WHEN** client calls `GET /api/v1/chat/sessions/:id/messages?before_turn=0`
- **THEN** the daemon returns a validation error

