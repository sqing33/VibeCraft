## MODIFIED Requirements

### Requirement: System MUST expose CLI tool settings for primary execution tools
The system MUST expose configurable primary CLI tools for at least `codex`, `claude`, `iflow`, and `opencode`.

Each CLI tool MUST declare:
- `id`
- `label`
- `protocol_family` (for backward-compatible single-family clients)
- optional `protocol_families` (the complete compatible protocol list)
- `cli_family`
- `enabled`
- optional executable path override

CLI tool settings MUST remain tool-level only. The CLI tool payload MUST NOT be the primary source of truth for shared model pools or per-runtime model bindings.

For `iflow`, the settings model MUST also include:
- browser-auth status derived from the managed iFlow home
- a browser-login action entrypoint and related status metadata

When `protocol_families` is omitted, the system MUST treat `protocol_family` as the tool's only compatible protocol.
When `protocol_families` is present, `protocol_family` MUST remain a stable primary/default family for backward compatibility.

#### Scenario: Read CLI tools including iFlow and OpenCode
- **WHEN** client requests CLI tool settings
- **THEN** the daemon returns the configured `codex`, `claude`, `iflow`, and `opencode` tools
- **AND** each tool includes a backward-compatible `protocol_family`
- **AND** multi-protocol tools also include `protocol_families`
- **AND** `iflow` includes browser-auth and login-status metadata

#### Scenario: Update CLI command path override
- **WHEN** client saves CLI tool settings with a new executable path override for `codex`, `claude`, `iflow`, or `opencode`
- **THEN** the daemon persists the new command path
- **AND** later runtime launches for that tool use the saved executable path unless overridden per request

#### Scenario: CLI tool settings do not own runtime model pools
- **WHEN** client saves or reads CLI tool settings
- **THEN** default model lists and per-model source bindings are managed through runtime model settings instead of the CLI tool payload
