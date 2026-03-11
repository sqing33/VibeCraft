## ADDED Requirements

### Requirement: Daemon MUST enumerate importable Codex CLI threads from `~/.codex`

The system MUST read Codex CLI history only from the current user's `~/.codex` directory.

The backend MUST inspect the latest readable `state_*.sqlite`, enumerate stored threads, and return a list suitable for frontend selection.

Each returned item MUST include:
- `thread_id`
- a readable `display_title`
- thread timestamps
- whether the thread has already been imported into local chat sessions

#### Scenario: List importable threads

- **WHEN** the frontend requests the Codex history list
- **THEN** the backend returns threads from `~/.codex/state_*.sqlite`
- **AND** each thread includes a readable `display_title`
- **AND** each thread indicates whether it is already imported

### Requirement: Imported Codex threads MUST be idempotently mapped into local chat sessions

When importing a Codex thread, the system MUST create at most one local chat session for the pair `cli_tool_id=codex` and `cli_session_id=<thread_id>`.

Imported sessions MUST persist the original Codex thread id so later chat turns can use existing CLI resume behavior.

#### Scenario: Import a new Codex thread

- **WHEN** a selected Codex thread has not been imported before
- **THEN** the backend creates a local chat session
- **AND** the session stores `cli_tool_id=codex`
- **AND** the session stores `cli_session_id=<thread_id>`

#### Scenario: Import the same thread again

- **WHEN** the user imports a Codex thread that already exists locally
- **THEN** the backend does not create a duplicate chat session
- **AND** the import result marks that thread as skipped or already imported

### Requirement: Imported Codex history MUST preserve restorable process details

The system MUST map rollout JSONL events into existing chat transcript and turn timeline tables.

At minimum, imported history MUST preserve:
- user messages
- final assistant messages
- tool calls and tool outputs
- available reasoning summaries
- agent progress messages when present

Imported timeline data MUST be readable through the existing chat turn timeline API without additional frontend-only reconstruction.

#### Scenario: Import thread with tool execution history

- **WHEN** a Codex rollout contains tool calls and outputs
- **THEN** the imported session contains `chat_turn_items` that restore those tool steps
- **AND** the existing timeline UI can render the command and output details
