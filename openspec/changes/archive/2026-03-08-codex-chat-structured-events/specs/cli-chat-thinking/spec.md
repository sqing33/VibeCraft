## MODIFIED Requirements

### Requirement: CLI chat SHOULD expose available thinking or progress events
When a CLI tool exposes reasoning, thinking, plan, tool-progress, question, or system progress events during a chat turn, the system MUST map them into user-visible intermediate chat events without collapsing everything into one thinking string.

For Codex-backed chat turns, the system MUST prefer fine-grained reasoning and planning notifications over completed-item snapshots when they are available, and MUST distinguish `thinking`, `tool`, `plan`, `question`, and `progress/system` event kinds in the structured stream.

Legacy `chat.turn.thinking.delta` events MAY continue for compatibility, but they MUST NOT be the only representation of tool or plan activity.

#### Scenario: Codex emits reasoning deltas
- **WHEN** Codex app-server emits reasoning content deltas
- **THEN** the daemon emits `chat.turn.event` with `kind=thinking` incrementally during the turn

#### Scenario: Codex emits command execution events
- **WHEN** Codex app-server emits command execution start/output/end notifications
- **THEN** the daemon emits `chat.turn.event` entries with `kind=tool` and stable entry IDs
- **AND** command output is updated in place instead of being appended to a plain thinking string

#### Scenario: Codex emits plan or user-input events
- **WHEN** Codex app-server emits plan updates or user-input requests
- **THEN** the daemon emits separate `kind=plan` or `kind=question` structured events
- **AND** the frontend can render them with dedicated styles
