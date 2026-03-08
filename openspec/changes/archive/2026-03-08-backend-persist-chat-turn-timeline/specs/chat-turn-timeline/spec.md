## ADDED Requirements

### Requirement: Chat sessions MUST persist structured turn timelines
The system MUST persist a structured timeline for each chat turn separately from the final transcript messages.

Each persisted turn MUST include the owning `session_id`, the initiating `user_message_id`, the turn number, lifecycle status, and any completed `assistant_message_id` when available.

Each persisted turn item MUST include a stable `entry_id`, chronological `seq`, `kind`, `status`, visible `content_text`, and serialized metadata required to restore the UI timeline.

#### Scenario: Create running turn timeline on turn start
- **WHEN** the daemon accepts a new chat turn and broadcasts `chat.turn.started`
- **THEN** the store creates a running turn record linked to that `session_id` and `user_message_id`
- **AND** later runtime entries for the same turn reuse that persisted turn record

#### Scenario: Complete turn timeline when assistant message is stored
- **WHEN** a chat turn finishes and the final assistant message is written
- **THEN** the persisted turn is updated to completed state
- **AND** the turn record stores the completed `assistant_message_id`

### Requirement: Chat timeline API MUST return restorable turn snapshots
The system MUST provide a backend API that returns persisted chat turns together with their structured timeline items for a session.

The API MUST include both completed turns and any currently running turn that already has persisted items.

The API MUST return items sorted by turn order and item `seq` so the frontend can rebuild the timeline without local guesswork.

#### Scenario: Read completed turn timeline
- **WHEN** the client requests the timeline of a session that contains completed assistant replies
- **THEN** the API returns completed turn records with their persisted items
- **AND** each completed turn includes the linked `assistant_message_id` when it exists

#### Scenario: Read running turn timeline after refresh
- **WHEN** the client refreshes while a turn is still running
- **THEN** the API returns the running turn together with its already persisted items
- **AND** the frontend can continue rendering the pending timeline before new WebSocket events arrive
