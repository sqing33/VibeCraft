## ADDED Requirements

### Requirement: CLI chat SHOULD expose available thinking or progress events
When a CLI tool exposes reasoning, thinking, plan, or tool-progress events during a chat turn, the system MUST map them into user-visible intermediate chat events.

For Codex-backed chat turns, the system MUST prefer fine-grained reasoning and planning notifications over completed-item snapshots when they are available.

If a tool does not expose stable reasoning text, the system MAY show plan/progress/tool events instead of true reasoning text.

#### Scenario: Claude emits thinking delta
- **WHEN** Claude Code emits a thinking/reasoning event during a chat turn
- **THEN** the daemon emits `chat.turn.thinking.delta`

#### Scenario: Codex emits progress without stable reasoning
- **WHEN** Codex emits plan/tool/progress events but no stable reasoning text
- **THEN** the daemon emits user-visible intermediate status updates rather than suppressing all mid-turn feedback

#### Scenario: Codex emits reasoning deltas
- **WHEN** Codex app-server emits `item/reasoning/summaryTextDelta` or `item/reasoning/textDelta`
- **THEN** the daemon emits `chat.turn.thinking.delta` incrementally during the turn

#### Scenario: Codex emits plan deltas without reasoning
- **WHEN** Codex app-server emits `item/plan/delta` but no stable reasoning text yet
- **THEN** the daemon emits user-visible intermediate updates instead of waiting for `item.completed`

### Requirement: CLI chat MUST distinguish thinking from tool, plan, and question activity
When Codex exposes reasoning, command execution, plan updates, user-input requests, or system progress during a chat turn, the daemon MUST map them into distinct structured runtime entries instead of collapsing them into a single thinking string.

Legacy `chat.turn.thinking.delta` events MAY continue for compatibility, but they MUST NOT be the only representation of tool or plan activity.

#### Scenario: Codex emits command execution events
- **WHEN** Codex app-server emits command execution start/output/end notifications
- **THEN** the daemon emits `chat.turn.event` entries with `kind=tool` and stable entry IDs
- **AND** command output updates the same tool entry in place

#### Scenario: Codex emits plan or question events
- **WHEN** Codex app-server emits plan deltas or user-input requests
- **THEN** the daemon emits `chat.turn.event` entries with `kind=plan` or `kind=question`
- **AND** the frontend can render them with dedicated styles

