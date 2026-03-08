## MODIFIED Requirements

### Requirement: Thinking translation MUST stay scoped to thinking content
When thinking translation is enabled for a chat turn, the system MUST apply translated output only to thinking content derived from the model reasoning stream.

The system MUST NOT overwrite answer, tool, plan, or progress entries with translated thinking text.

#### Scenario: Thinking translation updates structured feed
- **WHEN** translated thinking deltas are produced during a Codex turn
- **THEN** the daemon updates the active `kind=thinking` structured entry with translated content metadata
- **AND** the frontend can display translated thinking while preserving the original answer and tool entries
