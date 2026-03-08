# mcp-skill-settings Specification

## Purpose

MCP / Skill settings define how `vibe-tree` persists Codex-facing MCP registries and skill bindings, exposes them in the settings UI, and applies them to chat sessions without mutating the user's global Codex configuration.

## Requirements

### Requirement: System MUST expose MCP settings with per-tool defaults
The system MUST provide settings APIs and UI for a global MCP registry.

Each MCP entry MUST include:
- stable `id`
- display `label`
- connection/runtime fields needed to construct a Codex `mcp_servers` entry
- `enabled_cli_tool_ids`
- `default_enabled_cli_tool_ids`
- `enabled`

The settings UI MUST present MCP management in a dedicated `MCP` tab.

#### Scenario: Read MCP settings
- **WHEN** client requests MCP settings
- **THEN** the daemon returns all configured MCP entries and available CLI tools
- **AND** each MCP entry includes both enabled tool bindings and default-enabled tool bindings

#### Scenario: Save MCP defaults
- **WHEN** user updates MCP settings and marks an MCP as default-enabled for `codex`
- **THEN** the daemon persists that default binding
- **AND** future new chat sessions using `codex` initialize with that MCP selected

### Requirement: System MUST expose Skill settings with per-tool bindings
The system MUST provide settings APIs and UI for discovered skills and their tool bindings.

Each skill entry MUST include:
- stable `id`
- description and path when available
- source/discovery metadata
- `enabled_cli_tool_ids`
- `enabled`

The settings UI MUST present skill management in a dedicated `Skill` tab.
New skill bindings MUST default to enabled for all available CLI tools.

#### Scenario: Read Skill settings
- **WHEN** client requests Skill settings
- **THEN** the daemon returns discovered skills and available CLI tools
- **AND** each skill entry includes its current enabled tool bindings

#### Scenario: Newly discovered skill defaults to enabled for all tools
- **WHEN** daemon discovers a skill that does not yet have a saved binding
- **THEN** it returns that skill as enabled for all current CLI tools by default

### Requirement: Chat sessions MUST persist current MCP selection independently of defaults
The system MUST store the selected MCP ids for each chat session.
New chat sessions MUST initialize that selection from the default-enabled MCP set for the chosen CLI tool.
Users MAY change the MCP selection for the current session without modifying global defaults.

#### Scenario: New session inherits MCP defaults
- **WHEN** user creates a new `codex` chat session
- **THEN** the created session stores the MCP ids that are default-enabled for `codex`

#### Scenario: Session MCP changes do not mutate defaults
- **WHEN** user changes the MCP selection for an existing chat session
- **THEN** the session stores the new MCP ids
- **AND** the saved MCP default-enabled tool bindings remain unchanged
