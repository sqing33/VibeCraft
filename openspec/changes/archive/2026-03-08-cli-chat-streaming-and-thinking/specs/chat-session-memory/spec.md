## ADDED Requirements

### Requirement: Chat turn completion MUST remain stable when stream events degrade
Even when CLI streaming/thinking event parsing is partial or degraded, the final assistant message and session persistence MUST still succeed through final artifact fallback.

#### Scenario: Stream parse partially fails but final response succeeds
- **WHEN** some CLI mid-turn events cannot be parsed
- **THEN** the system still stores the final assistant message from the completed turn
- **AND** the turn does not fail solely because intermediate event parsing was incomplete
