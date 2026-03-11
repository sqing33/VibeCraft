## ADDED Requirements

### Requirement: Codex error notifications MUST respect will_retry semantics
When Codex app-server emits an `error` notification that includes `will_retry=true`, the daemon MUST treat it as a non-fatal progress signal.

When `will_retry=false`, the daemon MUST surface a visible error entry in the structured runtime feed and MUST still converge the turn to a terminal state.

#### Scenario: will_retry=true error is rendered as progress
- **WHEN** Codex emits an `error` notification with `will_retry=true`
- **THEN** the daemon emits a `chat.turn.event` entry with `kind=system` or `kind=progress`
- **AND** the entry status indicates the turn is still running (not failed)

#### Scenario: will_retry=false error is rendered as error but does not hang the turn
- **WHEN** Codex emits an `error` notification with `will_retry=false`
- **THEN** the daemon emits a `chat.turn.event` entry with `kind=error`
- **AND** the turn still reaches a terminal state (completed or failed) instead of remaining stuck in running

### Requirement: Tool execution feed MUST cap streamed stdout/stderr size
When a Codex chat turn streams tool execution output, the daemon MUST cap the amount of stdout/stderr included in the persisted `chat.turn.event` entry metadata.

The daemon MUST preserve the full raw stdout/stderr output in turn artifacts when an artifact directory is available.

#### Scenario: Large tool output is truncated in the structured feed
- **WHEN** a tool execution produces stdout or stderr larger than the configured feed limit
- **THEN** the `chat.turn.event` tool entry includes only the tail of stdout/stderr in its metadata
- **AND** the metadata indicates the output was truncated

#### Scenario: Full tool output is preserved in artifacts
- **WHEN** a tool execution produces stdout or stderr output during a Codex turn
- **AND** a turn artifact directory is available
- **THEN** the daemon writes the full output to artifacts
- **AND** the structured feed remains bounded in size

