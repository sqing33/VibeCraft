## ADDED Requirements

### Requirement: Codex chat runtime MUST inject only session-selected MCP servers
When a chat turn runs through the Codex app-server transport, the system MUST derive the effective MCP server set from the chat session and selected CLI tool, then pass only that set through the thread request `config` overrides.

#### Scenario: Thread start injects selected MCPs
- **WHEN** a new Codex-backed chat session has two selected MCP ids
- **THEN** `thread/start` includes only those two MCP servers in `config.mcp_servers`

#### Scenario: Thread resume preserves selected MCPs
- **WHEN** a Codex-backed chat session resumes an existing thread
- **THEN** `thread/resume` includes the same effective `config.mcp_servers` selection for that session

### Requirement: Codex chat runtime MUST inject effective skill guidance
When a chat turn runs through the Codex app-server transport, the system MUST append an effective skill allowlist to the thread base instructions.

The effective skill set MUST be the current CLI tool enabled skill set, intersected with the expert `enabled_skills` list when the expert declares one.
The injected guidance MUST include each skill id, a short description when available, its path, and instructions to read `SKILL.md` on demand instead of assuming its contents.

#### Scenario: Tool-enabled skill is injected
- **WHEN** a skill is enabled for the selected CLI tool and not excluded by the active expert
- **THEN** the thread base instructions include that skill in the injected allowlist block

#### Scenario: Expert restriction narrows effective skills
- **WHEN** the active expert declares `enabled_skills` containing only one of several tool-enabled skills
- **THEN** the injected skill allowlist contains only that intersected skill set
