# chat-cli-session-resume Specification

## Purpose

Define how CLI-backed chat sessions persist and reuse native CLI session or thread references across turns, including Codex app-server thread resume behavior.
## Requirements
### Requirement: Chat CLI turns MUST persist and reuse CLI session references
When a chat turn is executed through a primary CLI tool, the system MUST persist the CLI session/thread reference produced by that tool.

Subsequent turns in the same chat session MUST prefer the stored CLI session reference and invoke the tool's native resume mechanism instead of always reconstructing the full conversation in the prompt.

For Codex-backed chat turns, the native resume mechanism MUST prefer app-server `thread/resume` when the stored session reference is a valid Codex thread id.

If the native resume attempt fails, the system MUST retry once using locally reconstructed prompt input.

#### Scenario: Codex turn persists thread id
- **WHEN** a Codex CLI chat turn completes successfully
- **THEN** the system stores the returned `thread_id` as the session's CLI session reference

#### Scenario: Claude turn resumes by session id
- **WHEN** a chat session already has a stored Claude session id
- **THEN** the next turn invokes Claude Code with that session id and only sends the current turn input

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

### Requirement: Codex CLI session resume MUST restore reasoning effort defaults
When a chat session stores a Codex thread id and a last-used `reasoning_effort`, the system MUST include that effort as `config.model_reasoning_effort` during Codex thread start or resume.

#### Scenario: Codex resume restores reasoning effort default
- **WHEN** a chat session already stores a Codex thread id and `reasoning_effort`
- **THEN** the next Codex thread start or resume includes `config.model_reasoning_effort`
- **AND** only the current turn input is sent in `turn/start`

