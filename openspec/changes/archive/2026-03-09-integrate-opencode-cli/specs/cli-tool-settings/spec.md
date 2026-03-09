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
- `default_model_id`
- optional executable path override

When `protocol_families` is omitted, the system MUST treat `protocol_family` as the tool's only compatible protocol.
When `protocol_families` is present, `protocol_family` MUST remain a stable primary/default family for backward compatibility.

#### Scenario: Read CLI tools including OpenCode
- **WHEN** client requests CLI tool settings
- **THEN** the daemon returns the configured `codex`, `claude`, `iflow`, and `opencode` tools
- **AND** each tool includes a backward-compatible `protocol_family`
- **AND** multi-protocol tools also include `protocol_families`

#### Scenario: Update OpenCode default model
- **WHEN** client saves CLI tool settings with a new default model for `opencode`
- **THEN** the daemon persists the new default model
- **AND** later CLI resolution for `opencode` uses that model unless overridden per request
