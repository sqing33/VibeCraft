## ADDED Requirements

### Requirement: CLI chat SHOULD expose available thinking or progress events
When a CLI tool exposes reasoning, thinking, plan, or tool-progress events during a chat turn, the system MUST map them into user-visible intermediate chat events.

If a tool does not expose stable reasoning text, the system MAY show plan/progress/tool events instead of true reasoning text.

#### Scenario: Claude emits thinking delta
- **WHEN** Claude Code emits a thinking/reasoning event during a chat turn
- **THEN** the daemon emits `chat.turn.thinking.delta`

#### Scenario: Codex emits progress without stable reasoning
- **WHEN** Codex emits plan/tool/progress events but no stable reasoning text
- **THEN** the daemon emits user-visible intermediate status updates rather than suppressing all mid-turn feedback
