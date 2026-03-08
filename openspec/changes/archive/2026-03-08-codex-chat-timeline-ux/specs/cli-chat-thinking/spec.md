## MODIFIED Requirements

### Requirement: CLI chat MUST distinguish thinking from tool, plan, and question activity
When Codex exposes reasoning, command execution, plan updates, user-input requests, or system progress during a chat turn, the daemon MUST map them into distinct structured runtime entries instead of collapsing them into a single thinking string.

Legacy `chat.turn.thinking.delta` events MAY continue for compatibility, but they MUST NOT be the only representation of tool or plan activity.

For Codex-backed chat turns, consecutive reasoning deltas MAY append to the current `kind=thinking` entry only while no non-thinking runtime activity has interleaved. Any interleaving tool, plan, question, system, progress, or answer activity MUST close the current thinking entry, and later reasoning MUST create a new thinking entry.

#### Scenario: Codex emits command execution events
- **WHEN** Codex app-server emits command execution start/output/end notifications
- **THEN** the daemon emits `chat.turn.event` entries with `kind=tool` and stable entry IDs
- **AND** command output updates the same tool entry in place

#### Scenario: Interleaved tool call splits thinking timeline
- **WHEN** a Codex turn produces `thinking → tool → thinking`
- **THEN** the daemon emits at least two separate `kind=thinking` entries in chronological order
- **AND** the tool entry appears between them in the structured runtime feed

#### Scenario: Codex emits plan or question events
- **WHEN** Codex app-server emits plan deltas or user-input requests
- **THEN** the daemon emits `chat.turn.event` entries with `kind=plan` or `kind=question`
- **AND** the frontend can render them with dedicated styles
