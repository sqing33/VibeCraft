## MODIFIED Requirements

### Requirement: CLI chat MUST stream assistant output incrementally
When a chat turn is executed through a CLI tool, the system MUST emit assistant output incrementally instead of waiting for the entire CLI process to finish before sending the first delta.

For Codex-backed chat turns, the system MUST also expose a structured turn event stream that keeps assistant answer output separate from other runtime activity.

The daemon MUST preserve backward compatibility by continuing to emit legacy `chat.turn.delta` events while the structured event stream is available.

#### Scenario: Codex emits structured answer events
- **WHEN** a Codex-backed chat turn streams assistant text
- **THEN** the daemon emits `chat.turn.event` entries with `kind=answer` and incremental append operations
- **AND** the daemon also emits legacy `chat.turn.delta` compatibility events

#### Scenario: Assistant answer remains attached to final message
- **WHEN** the turn completes
- **THEN** the final assistant message content matches the accumulated `kind=answer` stream
- **AND** the answer can still be rendered independently from thinking, tool, and progress entries
