# cli-chat-streaming Specification

## Purpose

Define how CLI-backed chat turns stream assistant output and structured runtime activity, especially for Codex-backed sessions.
## Requirements
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

### Requirement: Structured Codex runtime feed MUST be recoverable from backend state
When a Codex-backed chat turn emits structured runtime activity, the daemon MUST persist each turn entry before or together with broadcasting incremental runtime events.

The persisted timeline MUST be sufficient to reconstruct the currently visible answer, tool, plan, question, system, and progress entries after a page refresh or WebSocket reconnect.

#### Scenario: Structured runtime event is persisted before refresh recovery
- **WHEN** a Codex-backed turn emits a `chat.turn.event` update for an entry
- **THEN** the daemon persists the updated turn entry in backend state
- **AND** a later page reload can rebuild that entry without relying on browser session storage

#### Scenario: Final answer matches persisted answer entry
- **WHEN** the final assistant message is stored for a Codex-backed turn
- **THEN** the persisted `kind=answer` timeline content matches the final assistant message content
- **AND** the completed turn can be restored from backend state alone

### Requirement: Codex app-server stdout parsing MUST tolerate non-JSON lines
The system MUST NOT fail a Codex-backed chat turn solely because the Codex app-server stdout stream contains non-JSON lines.

The daemon MUST ignore such lines for protocol handling and MUST capture them as diagnostics when an artifact directory is available.

#### Scenario: Non-JSON stdout line is ignored
- **WHEN** the Codex app-server stdout stream contains a line that is not valid JSON-RPC envelope JSON
- **THEN** the daemon continues parsing subsequent JSON-RPC envelopes
- **AND** the turn continues streaming without entering a failed terminal state because of that line

#### Scenario: Non-JSON stdout line is captured for diagnostics
- **WHEN** the daemon encounters a non-JSON stdout line during a Codex chat turn
- **AND** a turn artifact directory is available
- **THEN** the daemon appends that line to a turn-scoped diagnostics artifact
- **AND** the user-facing timeline is not polluted with raw log lines

### Requirement: Codex app-server client MUST support large JSON-RPC envelopes
The daemon MUST be able to parse individual Codex app-server JSON-RPC envelopes that exceed 4MB without aborting the connection.

If the daemon imposes a hard maximum envelope size, it MUST fail gracefully (diagnostics + terminal turn state) rather than crashing or deadlocking.

#### Scenario: Large envelope does not kill the stream
- **WHEN** the Codex app-server emits a single JSON-RPC envelope larger than 4MB
- **THEN** the daemon can still parse and process that envelope
- **AND** subsequent envelopes continue to be processed

### Requirement: Codex app-server calls MUST retry on transient overload
When a Codex app-server JSON-RPC call fails with an overload indication (for example error code `-32001`), the daemon MUST retry that call using exponential backoff with jitter before failing the turn.

Retries MUST be bounded (maximum attempts and maximum delay) to avoid infinite waiting.

#### Scenario: Overloaded thread start is retried
- **WHEN** `thread/start` fails with a transient overload error
- **THEN** the daemon retries `thread/start` with exponential backoff and jitter
- **AND** the turn proceeds without requiring the user to resend the message if a later retry succeeds

### Requirement: Codex turns MUST converge to a terminal state on transport failure
If the Codex app-server transport closes before `turn/completed` is received, the daemon MUST ensure the chat turn reaches a terminal state and the UI can stop showing it as running.

If any assistant text has already been streamed for the turn, the daemon MUST store a best-effort final assistant message from the accumulated text and MUST emit `chat.turn.completed`.

If no assistant text has been streamed and the failure happens before a usable thread/turn is established, the daemon MUST fall back once to the legacy CLI wrapper stream.

#### Scenario: Stream disconnect after partial assistant output still completes the turn
- **WHEN** the app-server stream disconnects mid-turn after the daemon has already streamed assistant deltas
- **THEN** the daemon completes the turn using the accumulated assistant text as the final assistant message
- **AND** the daemon emits `chat.turn.completed`

#### Scenario: Early transport failure falls back to legacy wrapper
- **WHEN** the Codex app-server transport fails before streaming any assistant output for a turn
- **THEN** the daemon retries the turn once using the legacy CLI wrapper transport

