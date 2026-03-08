## ADDED Requirements

### Requirement: CLI chat MUST stream assistant output incrementally
When a chat turn is executed through a CLI tool, the system MUST emit assistant output incrementally instead of waiting for the entire CLI process to finish before sending the first delta.

For Codex-backed chat turns, the system MUST prefer a fine-grained transport that exposes message delta notifications when available.

If the fine-grained Codex transport cannot be started or initialized, the system MUST fall back to the legacy parseable wrapper stream instead of failing the turn before any model output is produced.

#### Scenario: CLI assistant output streams during execution
- **WHEN** a CLI-backed chat turn starts producing assistant text
- **THEN** the daemon emits one or more `chat.turn.delta` events before the turn completes
- **AND** the final assistant message still matches the completed turn result

#### Scenario: Codex emits message delta through app-server
- **WHEN** a Codex-backed chat turn is started successfully through app-server
- **THEN** the daemon emits one or more `chat.turn.delta` events from `item/agentMessage/delta` before turn completion
- **AND** the final assistant message still matches the completed turn result

#### Scenario: Codex app-server startup fails early
- **WHEN** the Codex fine-grained transport fails before turn execution begins
- **THEN** the system retries the turn through the legacy CLI wrapper path
- **AND** the user still receives a valid assistant result when the wrapper path succeeds

### Requirement: CLI chat MUST expose a structured runtime feed for Codex turns
When a Codex-backed chat turn streams runtime activity, the daemon MUST emit `chat.turn.event` entries that keep answer text separate from other runtime activity.

Each structured runtime entry MUST carry a stable `entry_id` and a per-turn chronological `seq`. Updates to the same logical entry MUST reuse the same `entry_id` and `seq` so the frontend can update that entry in place without losing timeline order.

The daemon MUST preserve backward compatibility by continuing to emit legacy `chat.turn.delta` events while the structured feed is available.

#### Scenario: Codex emits structured answer events
- **WHEN** a Codex-backed chat turn streams assistant text
- **THEN** the daemon emits `chat.turn.event` entries with `kind=answer` and append/upsert operations
- **AND** the daemon also emits legacy `chat.turn.delta` compatibility events

#### Scenario: Structured runtime feed preserves timeline order
- **WHEN** Codex emits interleaved thinking, tool, and answer activity during one turn
- **THEN** each `chat.turn.event` payload includes stable `entry_id` and `seq`
- **AND** later updates to the same tool or answer entry reuse the original `seq`

#### Scenario: Final assistant message matches structured answer feed
- **WHEN** the turn completes
- **THEN** the final assistant message matches the accumulated `kind=answer` content
- **AND** the frontend can render answer independently from thinking, tool, plan, and progress entries
