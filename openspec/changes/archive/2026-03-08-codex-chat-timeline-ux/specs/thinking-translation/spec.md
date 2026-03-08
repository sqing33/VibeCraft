## MODIFIED Requirements

### Requirement: Structured runtime feed MUST keep translation scoped to thinking
When thinking translation is enabled during a Codex turn, translated reasoning MUST remain scoped to the active thinking entry in the runtime feed.

For structured Codex turns, `chat.turn.thinking.translation.delta` SHOULD include the target thinking `entry_id`. When a thinking segment is closed by interleaving runtime activity, the system MUST flush buffered translation before later reasoning starts a new thinking entry.

The system MUST NOT overwrite answer, tool, plan, or progress entries with translated thinking text.

#### Scenario: Thinking translation updates active thinking entry
- **WHEN** translated thinking deltas are emitted during a Codex turn
- **THEN** the frontend updates the matching `kind=thinking` entry with translated content metadata
- **AND** the original answer and tool entries remain unchanged

#### Scenario: Thinking boundary flushes translation before next segment
- **WHEN** a tool or plan event interrupts an in-progress thinking segment
- **THEN** any buffered translation for the current thinking segment is emitted before the next thinking segment starts
- **AND** later translation deltas target the new thinking entry instead of the old one
