## MODIFIED Requirements

### Requirement: System MUST expose CLI tool settings for primary execution tools
The system MUST expose configurable primary CLI tools for at least `codex`, `claude`, and `iflow`.

Each CLI tool MUST declare:
- `id`
- `label`
- `protocol_family`
- `cli_family`
- `enabled`
- `default_model_id`
- optional executable path override

#### Scenario: Read CLI tools
- **WHEN** client requests CLI tool settings
- **THEN** the daemon returns the configured `codex`, `claude`, and `iflow` tools with their protocol binding and default model

#### Scenario: Update CLI tool defaults
- **WHEN** client saves CLI tool settings with a new default model for `iflow`
- **THEN** the daemon persists the new default model
- **AND** later CLI resolution for `iflow` uses that model unless overridden per request
