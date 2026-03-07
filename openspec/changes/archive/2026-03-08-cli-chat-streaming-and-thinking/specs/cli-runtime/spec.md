## ADDED Requirements

### Requirement: Chat wrappers MUST emit normalized streaming events
CLI wrappers used for chat MUST emit a normalized event stream for the daemon to consume while the process is running.

The normalized stream MUST support at least:
- assistant text deltas
- session reference updates
- final completion signal

#### Scenario: Wrapper emits assistant delta events
- **WHEN** the underlying CLI emits incremental assistant text
- **THEN** the wrapper writes normalized assistant delta events to stdout for the daemon to relay
