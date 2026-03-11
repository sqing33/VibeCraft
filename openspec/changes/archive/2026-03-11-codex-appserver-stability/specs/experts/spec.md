## ADDED Requirements

### Requirement: Chat turns MUST enforce expert timeout_ms
When a chat turn is executed using an expert that resolves to a non-zero timeout, the daemon MUST enforce that timeout for the turn execution.

The daemon MUST ignore HTTP request cancellation for long-running turns, but it MUST still stop the turn when the expert timeout elapses.

#### Scenario: Chat turn stops after expert timeout
- **WHEN** the active chat expert resolves with `timeout_ms > 0`
- **AND** the turn execution exceeds that timeout
- **THEN** the daemon stops the turn execution
- **AND** the persisted turn timeline reaches a terminal state (not running)

