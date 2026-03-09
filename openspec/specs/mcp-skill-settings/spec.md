# mcp-skill-settings Specification

## Purpose

MCP / Skill settings define how `vibe-tree` persists Codex-facing MCP registries, exposes them in the settings UI, discovers skills from project/user directories, allows local skill installation, and applies both to chat sessions without mutating the user's global Codex configuration.

## Requirements

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
The MCP settings UI MUST use a fixed top toolbar and a separately scrollable card list area.
The MCP card list SHOULD render as a two-column grid when width permits.
The CLI default toggle region inside each MCP card MUST use a compact multi-column layout instead of one full-width row per CLI tool.
New MCP cards MUST start with empty editable JSON and MAY provide a non-persisted placeholder example.

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

#### Scenario: MCP settings page keeps toolbar visible
- **WHEN** the MCP settings tab contains enough cards to overflow vertically
- **THEN** the top area with count, add button, and save button remains visible
- **AND** only the MCP card list area scrolls

#### Scenario: New MCP uses placeholder instead of prefilled content
- **WHEN** user adds a new MCP card
- **THEN** the editable JSON value starts empty
- **AND** the placeholder example is shown until the user enters content

### Requirement: System MUST expose Skill settings as discovered catalog state with a global enabled switch
The system MUST provide a dedicated `Skill` settings tab that reflects the skills currently discovered from project-scoped and user-scoped skill directories.

Each skill entry MUST include:
- stable `id`
- description when available
- resolved `path`
- source/discovery metadata
- persisted `enabled` state used by vibe-tree runtime filtering

The Skill settings UI MUST present a single enable/disable switch per skill.
The Skill settings UI MUST NOT present per-tool binding switches.
The Skill settings UI MUST use a fixed top toolbar and a separately scrollable content area.
Skills without an explicit persisted binding MUST default to enabled.

#### Scenario: Read Skill settings
- **WHEN** client requests Skill settings
- **THEN** the daemon returns the currently discovered skill catalog
- **AND** each skill entry includes id, description, path, source metadata, and enabled state

#### Scenario: Discover skills from project and user roots
- **WHEN** matching `SKILL.md` files exist under project or user skill directories
- **THEN** the daemon includes them in the Skill settings response
- **AND** duplicate ids are de-duplicated into a single visible skill entry

#### Scenario: Disable a discovered skill
- **WHEN** user turns off a discovered skill in settings
- **THEN** the daemon persists that skill as disabled in `skill_bindings`
- **AND** subsequent Skill settings reads return that skill with `enabled=false`

### Requirement: System MUST support installing local skills for later Codex use
The system MUST allow users to add local Skill packages from the Skill settings page.

The system MUST accept either:
- a zip archive containing exactly one skill root with `SKILL.md`
- a directory upload whose reconstructed contents contain exactly one skill root with `SKILL.md`

Installed skills MUST be copied into the user-level Codex skills directory at `~/.codex/skills/<skill-id>/`.
After successful installation, the new skill MUST be discoverable from the Skill settings API.

#### Scenario: Install skill from zip
- **WHEN** user uploads a zip archive that contains one skill with `SKILL.md`
- **THEN** the daemon installs it to `~/.codex/skills/<skill-id>/`
- **AND** the Skill settings response includes that skill on the next read

#### Scenario: Install skill from directory upload
- **WHEN** user uploads a directory containing one skill with `SKILL.md`
- **THEN** the daemon reconstructs the directory tree, installs the skill to `~/.codex/skills/<skill-id>/`
- **AND** the installed skill becomes available to the runtime after discovery refresh

### Requirement: Codex runtime MUST inject only enabled discovered skills
The system MUST continue discovering skills from project and user roots.
The vibe-tree runtime MUST only inject skills that are both discovered and enabled.
If an expert specifies `enabled_skills`, the effective injected skill set MUST be the intersection of discovered skills, enabled skills, and `expert.enabled_skills`.

#### Scenario: Runtime excludes disabled skill
- **WHEN** a discovered skill has been disabled in Skill settings
- **THEN** it is absent from the effective Codex skill injection set

#### Scenario: Expert narrows enabled skills further
- **WHEN** two skills are enabled globally but expert `enabled_skills` contains only one of them
- **THEN** only that one skill is injected for that expert's Codex runtime

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
