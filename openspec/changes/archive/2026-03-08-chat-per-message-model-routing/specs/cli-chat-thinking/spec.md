## ADDED Requirements

### Requirement: Codex CLI chat MUST support selectable reasoning effort
When the active chat runtime is Codex CLI, the system MUST allow the user to select a reasoning effort level for the next turn.

The selected effort MUST be persisted as the session's default effort after a successful turn.

#### Scenario: Send Codex turn with selected effort
- **WHEN** the user selects `xhigh` for a Codex CLI turn
- **AND** sends a message
- **THEN** the daemon sends `effort=xhigh` to Codex app-server `turn/start`
- **AND** the session stores `reasoning_effort=xhigh` for later turns
