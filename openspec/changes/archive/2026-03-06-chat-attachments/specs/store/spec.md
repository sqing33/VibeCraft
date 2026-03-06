## ADDED Requirements

### Requirement: Chat attachment metadata SHALL be stored in SQLite
The system MUST persist chat attachment metadata in SQLite using a table dedicated to message-linked attachments.

Each attachment record MUST reference its owning chat session and message.

#### Scenario: Attachment record is created
- **WHEN** a chat turn with attachments is successfully accepted
- **THEN** SQLite stores one attachment record per uploaded file
- **AND** each record links to the owning `session_id` and `message_id`

## MODIFIED Requirements

### Requirement: File Storage Layout
The system MUST use data directory `~/.local/share/vibe-tree/`. The database file MUST be at `~/.local/share/vibe-tree/state.db`. Log files MUST be stored under `~/.local/share/vibe-tree/logs/`. Each execution MUST have its own log file: `{execution_id}.log`.

Chat attachments MUST be stored under a dedicated subdirectory of the data directory, organized by session and message so that persisted attachment files can be re-read for future chat reconstruction.

#### Scenario: Execution log file creation
- **WHEN** a new execution starts
- **THEN** a log file is created at `~/.local/share/vibe-tree/logs/{execution_id}.log`

#### Scenario: Chat attachment file creation
- **WHEN** a user sends a chat turn with attachments
- **THEN** each accepted attachment is stored under the chat attachment data directory
- **AND** the storage path is stable enough for later message history reconstruction
