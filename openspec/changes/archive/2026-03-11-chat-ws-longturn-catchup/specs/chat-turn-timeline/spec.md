## MODIFIED Requirements

### Requirement: Chat timeline API MUST return restorable turn snapshots
The system MUST provide a backend API that returns persisted chat turns together with their structured timeline items for a session.

The API MUST include both completed turns and any currently running turn that already has persisted items.

The API MUST return items sorted by turn order and item `seq` so the frontend can rebuild the timeline without local guesswork.

While a turn is running, the API MUST be safe to call repeatedly for catch-up: successive responses MUST remain stable for already persisted items (same `entry_id`/`seq`/content) and MAY include additional newly persisted items.

#### Scenario: Read completed turn timeline
- **WHEN** the client requests the timeline of a session that contains completed assistant replies
- **THEN** the API returns completed turn records with their persisted items
- **AND** each completed turn includes the linked `assistant_message_id` when it exists

#### Scenario: Read running turn timeline after refresh
- **WHEN** the client refreshes while a turn is still running
- **THEN** the API returns the running turn together with its already persisted items
- **AND** the frontend can continue rendering the pending timeline before new WebSocket events arrive

#### Scenario: Poll running turn timeline for catch-up
- **WHEN** the client polls the turns API while a turn is still running
- **THEN** the API returns the same running turn record
- **AND** already persisted items keep stable `entry_id` and `seq`
- **AND** any newly persisted items appear in `seq` order in the response

