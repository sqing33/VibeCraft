## MODIFIED Requirements

### Requirement: CLI chat MUST stream assistant output incrementally
When a chat turn is executed through a CLI tool, the system MUST emit assistant output incrementally instead of waiting for the entire CLI process to finish before sending the first delta.

For Codex-backed chat turns, the system MUST prefer a fine-grained transport that exposes message delta notifications when available.

If the fine-grained Codex transport cannot be started or initialized, the system MUST fall back to the legacy parseable wrapper stream instead of failing the turn before any model output is produced.

#### Scenario: Codex emits message delta through app-server
- **WHEN** a Codex-backed chat turn is started successfully through app-server
- **THEN** the daemon emits one or more `chat.turn.delta` events from `item/agentMessage/delta` before turn completion
- **AND** the final assistant message still matches the completed turn result

#### Scenario: Codex app-server startup fails early
- **WHEN** the Codex fine-grained transport fails before turn execution begins
- **THEN** the system retries the turn through the legacy CLI wrapper path
- **AND** the user still receives a valid assistant result when the wrapper path succeeds
