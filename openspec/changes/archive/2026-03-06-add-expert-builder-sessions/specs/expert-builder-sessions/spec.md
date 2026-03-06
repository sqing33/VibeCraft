## ADDED Requirements

### Requirement: Expert builder uses persistent sessions
The system MUST provide persistent expert builder sessions so users can continue long-form conversations while refining an expert.

#### Scenario: Create builder session
- **WHEN** client creates a new expert builder session with a selected builder model
- **THEN** the system stores a new session record with status `draft`
- **AND** returns the session id and its metadata

#### Scenario: Continue existing session
- **WHEN** client loads an existing expert builder session
- **THEN** the system returns prior messages and the latest draft snapshot
- **AND** the user can append more messages to continue refinement

### Requirement: Builder messages and draft snapshots are stored separately
The system MUST persist builder conversation messages and generated expert snapshots as separate records.

#### Scenario: Append message generates new snapshot
- **WHEN** client appends a user message to a builder session
- **THEN** the system stores the user message and assistant reply
- **AND** stores a new expert draft snapshot with an incremented version number

#### Scenario: Read snapshot history
- **WHEN** client requests a builder session detail
- **THEN** the system returns the ordered snapshot history for that session
- **AND** each snapshot includes version, created time, and draft payload

### Requirement: Builder sessions can publish snapshots to experts
The system MUST allow publishing a builder snapshot into the expert registry while preserving source references.

#### Scenario: Publish latest snapshot
- **WHEN** client publishes a builder session snapshot
- **THEN** the system writes or updates the target expert config from that snapshot
- **AND** records the source session id and snapshot id on the expert

#### Scenario: Continue optimizing a published expert
- **WHEN** client opens AI refinement for an existing expert that has a source session
- **THEN** the UI can load that session and continue from its saved history
