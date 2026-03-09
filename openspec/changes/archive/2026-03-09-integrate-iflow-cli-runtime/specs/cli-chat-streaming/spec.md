## ADDED Requirements

### Requirement: Plain-text CLI wrappers MUST support line-based assistant streaming
When a CLI-backed chat tool does not expose structured JSON stream events, the wrapper MAY emit plain-text stdout lines.
In that case, the daemon MUST still be able to surface best-effort assistant deltas during execution and MUST treat the artifact directory as the final source of truth for completed turn content.

#### Scenario: IFLOW emits plain-text deltas
- **WHEN** an IFLOW-backed chat turn streams plain-text stdout during a non-interactive run
- **THEN** the daemon relays those lines as assistant delta updates before turn completion
- **AND** the final assistant message is still overridden by `final_message.md` when that artifact exists
