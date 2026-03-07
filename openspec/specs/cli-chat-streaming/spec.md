## ADDED Requirements

### Requirement: CLI chat MUST stream assistant output incrementally
When a chat turn is executed through a CLI tool, the system MUST emit assistant output incrementally instead of waiting for the entire CLI process to finish before sending the first delta.

#### Scenario: CLI assistant output streams during execution
- **WHEN** a CLI-backed chat turn starts producing assistant text
- **THEN** the daemon emits one or more `chat.turn.delta` events before the turn completes
- **AND** the final assistant message still matches the completed turn result
