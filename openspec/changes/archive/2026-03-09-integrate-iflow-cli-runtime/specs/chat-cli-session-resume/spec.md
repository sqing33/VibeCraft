## MODIFIED Requirements

### Requirement: Chat CLI turns MUST persist and reuse CLI session references
When a chat turn is executed through a primary CLI tool, the system MUST persist the CLI session/thread reference produced by that tool.

Subsequent turns in the same chat session MUST prefer the stored CLI session reference and invoke the tool's native resume mechanism instead of always reconstructing the full conversation in the prompt.

For Codex-backed chat turns, the native resume mechanism MUST prefer app-server `thread/resume` when the stored session reference is a valid Codex thread id.
For IFLOW-backed chat turns, the native resume mechanism MUST prefer `iflow --resume <session-id>` when a stored IFLOW session id exists.

If the native resume attempt fails, the system MUST retry once using locally reconstructed prompt input.

#### Scenario: Codex turn persists thread id
- **WHEN** a Codex CLI chat turn completes successfully
- **THEN** the system stores the returned `thread_id` as the session's CLI session reference

#### Scenario: Claude turn resumes by session id
- **WHEN** a chat session already has a stored Claude session id
- **THEN** the next turn invokes Claude Code with that session id and only sends the current turn input

#### Scenario: IFLOW turn persists session id
- **WHEN** an IFLOW CLI chat turn completes successfully
- **THEN** the system stores the returned IFLOW `session-id` as the session's CLI session reference

#### Scenario: IFLOW turn resumes by session id
- **WHEN** a chat session already has a stored IFLOW session id
- **THEN** the next turn invokes IFLOW with that session id and only sends the current turn input

#### Scenario: Resume failure falls back to local reconstruction
- **WHEN** a chat turn attempts CLI resume and the CLI reports resume failure
- **THEN** the system retries using locally reconstructed prompt input

#### Scenario: Codex app-server resumes by stored thread id
- **WHEN** a chat session already has a stored Codex thread id
- **THEN** the next Codex turn invokes app-server `thread/resume`
- **AND** only the current turn input is sent in `turn/start`

#### Scenario: Codex thread resume fails
- **WHEN** a Codex turn attempts app-server `thread/resume` and the server rejects the thread id
- **THEN** the system starts a new thread and retries using locally reconstructed prompt input
