## MODIFIED Requirements

### Requirement: CLI chat MUST stream assistant output incrementally
When a chat turn is executed through a CLI tool, the system MUST emit assistant output incrementally instead of waiting for the entire CLI process to finish before sending the first delta.

For Codex-backed chat turns, the system MUST prefer a fine-grained transport that exposes message delta notifications when available.
For OpenCode-backed chat turns, the system MUST parse best-effort JSON line events from `opencode run --format json` when available.

If the fine-grained Codex transport cannot be started or initialized, the system MUST fall back to the legacy parseable wrapper stream instead of failing the turn before any model output is produced.

#### Scenario: OpenCode emits assistant output through JSON events
- **WHEN** an OpenCode-backed chat turn is started through the wrapper with `--format json`
- **THEN** the daemon emits one or more `chat.turn.delta` events when assistant text is present in the JSON event stream
- **AND** the final assistant message still matches the completed turn result

#### Scenario: OpenCode runtime falls back to artifact truth source
- **WHEN** OpenCode JSON events are incomplete or omit the final assistant body
- **THEN** the completed turn still reads `final_message.md` and `session.json` from the wrapper artifact directory
- **AND** the user still receives a valid final assistant message
