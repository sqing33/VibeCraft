## MODIFIED Requirements

### Requirement: System MUST expose MCP settings as a JSON-native registry with per-tool defaults
The system MUST provide settings APIs and UI for a global MCP registry.

Each MCP entry MUST include:
- a stable `id` derived from the JSON server key
- the parsed server `config` needed to construct a managed gateway downstream entry
- the original `raw_json` used for editing
- `default_enabled_cli_tool_ids`

The MCP settings payload MUST additionally include gateway configuration state containing at least:
- `enabled`
- `idle_ttl_seconds`

The system MUST accept both `{"mcpServers": {...}}` and flat `{...}` JSON shapes when saving MCP entries.
The settings UI MUST present MCP management in a dedicated `MCP` tab.
The MCP settings UI MUST NOT require the user to separately enter display name, server id, or a second tool-level enable binding.
The MCP settings UI MUST use a fixed top toolbar and a separately scrollable card list area.
The MCP card list SHOULD render as a two-column grid when width permits.
The CLI default toggle region inside each MCP card MUST use a compact multi-column layout instead of one full-width row per CLI tool.
New MCP cards MUST start with empty editable JSON and MAY provide a non-persisted placeholder example.
The MCP settings UI MUST also expose gateway enablement, idle TTL, and a runtime status entry point without requiring users to inspect raw config files.

#### Scenario: Read MCP settings
- **WHEN** client requests MCP settings
- **THEN** the daemon returns all configured MCP entries and available CLI tools
- **AND** each MCP entry includes parsed config, raw JSON, and default-enabled tool bindings
- **AND** the response includes persisted gateway configuration

#### Scenario: Save MCP settings from wrapped JSON
- **WHEN** user saves an MCP entry using `{"mcpServers": {"mcp-router": {...}}}`
- **THEN** the daemon persists a server whose `id` is `mcp-router`
- **AND** future reads return both its parsed config and the original JSON text

#### Scenario: Save MCP settings from flat JSON
- **WHEN** user saves an MCP entry using `{"mcp-router": {...}}`
- **THEN** the daemon persists the same `mcp-router` entry
- **AND** the MCP remains eligible for session selection and gateway-managed runtime use

#### Scenario: MCP settings page keeps toolbar visible
- **WHEN** the MCP settings tab contains enough cards to overflow vertically
- **THEN** the top area with count, add button, and save button remains visible
- **AND** only the MCP card list area scrolls

#### Scenario: New MCP uses placeholder instead of prefilled content
- **WHEN** user adds a new MCP card
- **THEN** the editable JSON value starts empty
- **AND** the placeholder example is shown until the user enters content

#### Scenario: User enables gateway and sets idle TTL
- **WHEN** user saves MCP settings with gateway enabled and a new idle TTL value
- **THEN** the daemon persists the gateway configuration
- **AND** future MCP settings reads return the same gateway enablement and TTL

### Requirement: Chat sessions MUST persist current MCP selection independently of defaults
The system MUST store the selected MCP ids for each chat session.
New chat sessions MUST initialize that selection from the default-enabled MCP set for the chosen CLI tool.
Users MAY change the MCP selection for the current session without modifying global defaults.
Any persisted MCP entry MUST remain selectable for a session even when it is not default-enabled for that CLI tool.
When a session's selected MCP ids change, the daemon MUST use that updated selection as the source of truth for gateway access on the next chat turn.

#### Scenario: New session inherits MCP defaults
- **WHEN** user creates a new `codex` chat session
- **THEN** the created session stores the MCP ids that are default-enabled for `codex`

#### Scenario: Session can select non-default MCP
- **WHEN** a saved MCP entry is not default-enabled for `codex`
- **AND** the user manually selects it for the current chat session
- **THEN** the session stores that MCP id
- **AND** the global default-enabled bindings remain unchanged

#### Scenario: Updated session selection becomes next-turn gateway policy
- **WHEN** user changes the current session's selected MCP ids during an active conversation
- **THEN** the daemon persists the updated selection before starting the next turn
- **AND** the next turn's gateway access policy uses the updated selection
