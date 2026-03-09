## MODIFIED Requirements

### Requirement: Chat CLI turns MUST persist and reuse CLI session references
When a chat turn is executed through a primary CLI tool, the system MUST persist the CLI session/thread reference produced by that tool.

Subsequent turns in the same chat session MUST prefer the stored CLI session reference and invoke the tool's native resume mechanism instead of always reconstructing the full conversation in the prompt.

For Codex-backed chat turns, the native resume mechanism MUST prefer app-server `thread/resume` when the stored session reference is a valid Codex thread id.
For IFLOW-backed chat turns, the native resume mechanism MUST prefer `iflow --resume <session-id>` when a stored IFLOW session id exists.
For OpenCode-backed chat turns, the native resume mechanism MUST prefer `opencode run --session <session-id>` when a stored OpenCode session id exists.

For Codex-backed chat turns, the system MUST additionally reuse a compatible in-memory app-server runtime for the same chat session when one is still alive in the current daemon process.

If the native resume attempt fails, the system MUST retry once using locally reconstructed prompt input.

#### Scenario: OpenCode turn persists session id
- **WHEN** an OpenCode CLI chat turn completes successfully
- **THEN** the system stores the returned OpenCode `session id` as the session's CLI session reference

#### Scenario: OpenCode turn resumes by session id
- **WHEN** a chat session already has a stored OpenCode session id
- **THEN** the next turn invokes OpenCode with that session id and only sends the current turn input
