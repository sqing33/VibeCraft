## MODIFIED Requirements

### Requirement: iFlow runtime MUST use official authentication only
The `iflow` runtime MUST NOT depend on the generic OpenAI-compatible LLM source/model settings.

The runtime MUST support exactly these official iFlow auth inputs:
- browser-auth state persisted in the managed iFlow home
- explicit official iFlow API key from the CLI tool settings card

#### Scenario: iFlow API key auth injects official env
- **WHEN** a chat or repo analysis request selects `iflow` and the tool is configured for `api_key`
- **THEN** the runtime injects official iFlow auth env values
- **AND** it does not inject OpenAI-compatible auth env values

#### Scenario: iFlow browser auth reuses managed home
- **WHEN** a chat or repo analysis request selects `iflow` and the tool is configured for `browser`
- **THEN** the runtime uses the daemon-managed iFlow home for auth reuse
- **AND** it relies on the persisted official browser login state

### Requirement: iFlow runtime MUST support per-turn MCP and skill injection
The `iflow` runtime MUST inject the effective MCP and skill selection derived from vibecraft settings and the chat session.

#### Scenario: iFlow turn receives effective MCP allow-list
- **WHEN** an iFlow chat turn runs with selected/default MCP server ids
- **THEN** the runtime syncs those MCP definitions into project scope
- **AND** passes only the effective subset through `--allowed-mcp-server-names`

#### Scenario: iFlow turn receives effective skills
- **WHEN** an iFlow chat turn runs with enabled skill bindings
- **THEN** the runtime appends effective skill instructions to the system prompt before launching the CLI
