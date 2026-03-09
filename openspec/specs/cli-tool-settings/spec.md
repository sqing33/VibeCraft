# cli-tool-settings Specification

## Purpose

CLI tool settings define the primary execution tools for `vibe-tree`, binding protocol families to concrete CLI runtimes and tool-specific defaults.

## Requirements

### Requirement: System MUST expose CLI tool settings for primary execution tools
The system MUST expose configurable primary CLI tools for at least `codex`, `claude`, and `iflow`.

Each CLI tool MUST declare:
- `id`
- `label`
- `protocol_family`
- `cli_family`
- `enabled`
- optional executable path override

For `iflow`, the settings model MUST also include:
- official auth mode (`browser` or `api_key`)
- official base URL
- persisted masked API key support
- ordered iFlow model list
- default iFlow model
- browser-auth state derived from the managed iFlow home

#### Scenario: Read CLI tools with iFlow official settings
- **WHEN** client requests CLI tool settings
- **THEN** the daemon returns the configured `codex`, `claude`, and `iflow` tools
- **AND** `iflow` includes official auth, masked-key, browser-auth, and model-list metadata

#### Scenario: Update iFlow official settings
- **WHEN** client saves CLI tool settings with a new `iflow_auth_mode`, `iflow_models`, or `iflow_default_model`
- **THEN** the daemon persists the new iFlow settings
- **AND** later iFlow runtime resolution uses those values unless overridden per request

#### Scenario: Update non-iFlow tool defaults
- **WHEN** client saves CLI tool settings with a new default model for `codex` or `claude`
- **THEN** the daemon persists the new default model
- **AND** later CLI resolution for that tool uses the saved default unless overridden per request
