## ADDED Requirements

### Requirement: Store MUST persist chat turn timelines in SQLite
The store MUST persist chat turn timelines in SQLite using dedicated turn and turn-item tables in addition to existing chat session and message tables.

The turn table MUST record turn-level metadata needed for recovery, including session linkage, user message linkage, completion linkage, lifecycle state, and restoration metadata.

The turn-item table MUST record persisted structured timeline entries keyed by stable turn identity and `entry_id`, and it MUST preserve chronological `seq` ordering for recovery.

#### Scenario: Timeline tables exist after migration
- **WHEN** the daemon opens a database that has not yet seen chat timeline persistence
- **THEN** migration creates the chat turn and chat turn item tables with required indexes
- **AND** the schema version is updated to the latest version

#### Scenario: Store upserts one timeline entry in place
- **WHEN** the daemon receives a new update for an existing persisted timeline `entry_id`
- **THEN** the store updates the existing turn-item row instead of creating a duplicate visible entry
- **AND** the original chronological `seq` remains stable

### Requirement: Store MUST list persisted chat turns with ordered items
The store MUST provide query helpers that return persisted chat turns together with their ordered timeline items for one session.

Returned items MUST be sorted by ascending `seq` within each turn.

#### Scenario: Query returns ordered timeline items
- **WHEN** the backend reads persisted turns for a session
- **THEN** each turn includes its associated persisted items in chronological order
- **AND** the query result is sufficient for the frontend to rebuild completed and running timelines
