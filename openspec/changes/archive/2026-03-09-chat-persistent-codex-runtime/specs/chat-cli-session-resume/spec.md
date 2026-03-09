## MODIFIED Requirements

### Requirement: Chat CLI turns MUST persist and reuse CLI session references
When a chat turn is executed through a primary CLI tool, the system MUST persist the CLI session/thread reference produced by that tool.

Subsequent turns in the same chat session MUST prefer the stored CLI session reference and invoke the tool's native resume mechanism instead of always reconstructing the full conversation in the prompt.

For Codex-backed chat turns, the system MUST additionally reuse a compatible in-memory app-server runtime for the same chat session when one is still alive in the current daemon process.

If the native resume attempt fails, the system MUST retry once using locally reconstructed prompt input.

#### Scenario: Warm Codex runtime handles the next turn without reinitializing app-server
- **WHEN** a Codex-backed chat session already has an alive compatible app-server runtime in the daemon
- **AND** the user sends the next turn in the same chat session
- **THEN** the system reuses that app-server runtime
- **AND** does not spawn a second `codex app-server` process for that turn
- **AND** only the current turn input is sent in `turn/start`

#### Scenario: Cold daemon resumes Codex thread from persisted session reference
- **WHEN** the daemon no longer has a warm runtime for a chat session
- **AND** the session still has a persisted Codex `thread_id`
- **THEN** the system starts a new app-server runtime
- **AND** calls `thread/resume` before `turn/start`

#### Scenario: Incompatible runtime configuration forces a rebuild
- **WHEN** a chat session changes model, workspace, base instructions, MCP configuration, or reasoning-effort-derived runtime config
- **THEN** the system does not reuse the old warm runtime
- **AND** rebuilds a new app-server runtime for the next turn
