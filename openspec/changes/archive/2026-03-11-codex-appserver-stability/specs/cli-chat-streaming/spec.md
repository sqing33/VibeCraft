## ADDED Requirements

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

