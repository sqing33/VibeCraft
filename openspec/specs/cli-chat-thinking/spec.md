# cli-chat-thinking Specification

## Purpose

Define how CLI-backed chat turns expose reasoning, planning, tool progress, and other non-final process signals to the user.
## Requirements
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

### Requirement: Persisted Codex reasoning MUST avoid duplicate summary and raw text
For one Codex reasoning item, the system MUST prefer readable `summaryTextDelta` content for the visible thinking timeline whenever it is available.

If readable summary content has already been emitted for that reasoning item, later raw `textDelta` content for the same item MUST NOT be appended to the visible persisted timeline entry.

If no readable summary content exists for that reasoning item, the system MAY use raw reasoning text as the visible fallback.

#### Scenario: Summary reasoning suppresses raw reasoning duplication
- **WHEN** one Codex reasoning item emits both `item/reasoning/summaryTextDelta` and `item/reasoning/textDelta`
- **THEN** the visible persisted thinking timeline keeps the readable summary content
- **AND** the raw reasoning text does not create duplicate visible characters in the same turn timeline

#### Scenario: Raw reasoning is used when no summary exists
- **WHEN** a Codex reasoning item emits raw `item/reasoning/textDelta` content but no readable summary deltas
- **THEN** the system persists that raw reasoning as the visible thinking content for the timeline

### Requirement: Codex CLI chat MUST support selectable reasoning effort
When the active chat runtime is Codex CLI, the system MUST allow the user to select a reasoning effort level for the next turn.

The selected effort MUST be persisted as the session's default effort after a successful turn.

#### Scenario: Send Codex turn with selected effort
- **WHEN** the user selects `xhigh` for a Codex CLI turn
- **AND** sends a message
- **THEN** the daemon sends `effort=xhigh` to Codex app-server `turn/start`
- **AND** the session stores `reasoning_effort=xhigh` for later turns

