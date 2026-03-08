## MODIFIED Requirements

### Requirement: System MUST expose MCP settings as a JSON-native registry with per-tool defaults
The system MUST provide settings APIs and UI for a global MCP registry.

Each MCP entry MUST include:
- a stable `id` derived from the JSON server key
- the parsed server `config` needed to construct a Codex `mcp_servers` entry
- the original `raw_json` used for editing
- `default_enabled_cli_tool_ids`

The system MUST accept both `{"mcpServers": {...}}` and flat `{...}` JSON shapes when saving MCP entries.
The settings UI MUST present MCP management in a dedicated `MCP` tab.
The MCP settings UI MUST NOT require the user to separately enter display name, server id, or a second tool-level enable binding.

#### Scenario: Read MCP settings
- **WHEN** client requests MCP settings
- **THEN** the daemon returns all configured MCP entries and available CLI tools
- **AND** each MCP entry includes parsed config, raw JSON, and default-enabled tool bindings

#### Scenario: Save MCP settings from wrapped JSON
- **WHEN** user saves an MCP entry using `{"mcpServers": {"mcp-router": {...}}}`
- **THEN** the daemon persists a server whose `id` is `mcp-router`
- **AND** future reads return both its parsed config and the original JSON text

#### Scenario: Save MCP settings from flat JSON
- **WHEN** user saves an MCP entry using `{"mcp-router": {...}}`
- **THEN** the daemon persists the same `mcp-router` entry
- **AND** the MCP remains eligible for session selection and runtime injection

### Requirement: System MUST expose Skill settings as discovered catalog state
The system MUST provide a dedicated `Skill` settings tab that reflects the skills currently discovered from project-scoped and user-scoped skill directories.

Each skill entry MUST include:
- stable `id`
- description when available
- resolved `path`
- source/discovery metadata

Discovered skills MUST be considered available to CLI runtimes by default.
The Skill settings UI MUST NOT present per-skill enable/disable switches or per-tool binding switches.

#### Scenario: Read Skill settings
- **WHEN** client requests Skill settings
- **THEN** the daemon returns the currently discovered skill catalog
- **AND** each skill entry includes id, description, path, and source metadata

#### Scenario: Discover skills from project and user roots
- **WHEN** matching `SKILL.md` files exist under project or user skill directories
- **THEN** the daemon includes them in the Skill settings response
- **AND** duplicate ids are de-duplicated into a single visible skill entry

### Requirement: Chat sessions MUST persist current MCP selection independently of defaults
The system MUST store the selected MCP ids for each chat session.
New chat sessions MUST initialize that selection from the default-enabled MCP set for the chosen CLI tool.
Users MAY change the MCP selection for the current session without modifying global defaults.
Any persisted MCP entry MUST remain selectable for a session even when it is not default-enabled for that CLI tool.

#### Scenario: New session inherits MCP defaults
- **WHEN** user creates a new `codex` chat session
- **THEN** the created session stores the MCP ids that are default-enabled for `codex`

#### Scenario: Session can select non-default MCP
- **WHEN** a saved MCP entry is not default-enabled for `codex`
- **AND** the user manually selects it for the current chat session
- **THEN** the session stores that MCP id
- **AND** the global default-enabled bindings remain unchanged
