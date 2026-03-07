## MODIFIED Requirements

### Requirement: Chat CLI turns MUST persist and reuse CLI session references
When a chat turn is executed through a primary CLI tool, the system MUST persist the CLI session/thread reference produced by that tool.

Subsequent turns in the same chat session MUST prefer the stored CLI session reference and invoke the tool's native resume mechanism instead of always reconstructing the full conversation in the prompt.

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
