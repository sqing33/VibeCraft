# cli-tool-settings Specification

## Purpose

CLI tool settings define the primary execution tools for `vibe-tree`, binding protocol families to concrete CLI runtimes and default models.

## Requirements

### Requirement: System MUST expose CLI tool settings for primary execution tools
The system MUST expose configurable primary CLI tools for at least `codex` and `claude`.

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
- **THEN** the daemon returns the configured `codex` and `claude` tools with their protocol binding and default model

#### Scenario: Update CLI tool defaults
- **WHEN** client saves CLI tool settings with a new default model for `codex`
- **THEN** the daemon persists the new default model
- **AND** later CLI resolution for `codex` uses that model unless overridden per request
