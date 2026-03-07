## MODIFIED Requirements

### Requirement: CLI chat SHOULD expose available thinking or progress events
When a CLI tool exposes reasoning, thinking, plan, or tool-progress events during a chat turn, the system MUST map them into user-visible intermediate chat events.

For Codex-backed chat turns, the system MUST prefer fine-grained reasoning and planning notifications over completed-item snapshots when they are available.

If stable reasoning text is unavailable for a Codex turn, the system MAY show plan or tool progress updates instead of suppressing all mid-turn feedback.

#### Scenario: Codex emits reasoning deltas
- **WHEN** Codex app-server emits `item/reasoning/summaryTextDelta` or `item/reasoning/textDelta`
- **THEN** the daemon emits `chat.turn.thinking.delta` incrementally during the turn

#### Scenario: Codex emits plan deltas without reasoning
- **WHEN** Codex app-server emits `item/plan/delta` but no stable reasoning text yet
- **THEN** the daemon emits user-visible intermediate updates instead of waiting for `item.completed`
